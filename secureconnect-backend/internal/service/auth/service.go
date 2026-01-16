package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/internal/repository/redis"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/email"
	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/logger"
)

// UserRepository interface
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	UpdateStatus(ctx context.Context, userID uuid.UUID, status string) error
	EmailExists(ctx context.Context, email string) (bool, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
}

// DirectoryRepository interface
type DirectoryRepository interface {
	EmailExists(ctx context.Context, email string) (bool, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
	SetEmailToUserID(ctx context.Context, email string, userID uuid.UUID) error
	SetUsernameToUserID(ctx context.Context, username string, userID uuid.UUID) error
}

// SessionRepository interface
type SessionRepository interface {
	CreateSession(ctx context.Context, session *redis.Session, ttl time.Duration) error
	GetSession(ctx context.Context, sessionID string) (*redis.Session, error)
	DeleteSession(ctx context.Context, sessionID string, userID uuid.UUID) error
	BlacklistToken(ctx context.Context, jti string, expiresAt time.Duration) error
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
	GetAccountLock(ctx context.Context, key string) (*redis.AccountLock, error)
	LockAccount(ctx context.Context, key string, lockedUntil time.Time) error
	GetFailedLoginAttempts(ctx context.Context, key string) (int, error)
	SetFailedLoginAttempts(ctx context.Context, key string, attempts int) error
	GetFailedLoginAttempt(ctx context.Context, key string) (*redis.FailedLoginAttempt, error)
	SetFailedLoginAttempt(ctx context.Context, key string, attempt *redis.FailedLoginAttempt) error
	DeleteFailedLoginAttempts(ctx context.Context, key string) error
}

// PresenceRepository interface
type PresenceRepository interface {
	SetUserOnline(ctx context.Context, userID uuid.UUID) error
	SetUserOffline(ctx context.Context, userID uuid.UUID) error
}

// EmailVerificationRepository interface for password reset tokens
type EmailVerificationRepository interface {
	CreateToken(ctx context.Context, userID uuid.UUID, newEmail, token string, expiresAt time.Time) error
	GetToken(ctx context.Context, token string) (*cockroach.EmailVerificationToken, error)
	MarkTokenUsed(ctx context.Context, token string) error
}

// EmailService interface for sending emails
type EmailService interface {
	SendPasswordResetEmail(ctx context.Context, to string, data *email.PasswordResetEmailData) error
}

// Service handles authentication business logic
type Service struct {
	userRepo              UserRepository
	directoryRepo         DirectoryRepository
	sessionRepo           SessionRepository
	presenceRepo          PresenceRepository
	emailVerificationRepo EmailVerificationRepository
	emailService          EmailService
	jwtManager            *jwt.JWTManager
}

// NewService creates a new auth service
func NewService(
	userRepo UserRepository,
	directoryRepo DirectoryRepository,
	sessionRepo SessionRepository,
	presenceRepo PresenceRepository,
	emailVerificationRepo EmailVerificationRepository,
	emailService EmailService,
	jwtManager *jwt.JWTManager,
) *Service {
	return &Service{
		userRepo:              userRepo,
		directoryRepo:         directoryRepo,
		sessionRepo:           sessionRepo,
		presenceRepo:          presenceRepo,
		emailVerificationRepo: emailVerificationRepo,
		emailService:          emailService,
		jwtManager:            jwtManager,
	}
}

// RegisterInput contains user registration data
type RegisterInput struct {
	Email       string
	Username    string
	Password    string
	DisplayName string
}

// RegisterOutput contains registration result
type RegisterOutput struct {
	User         *domain.UserResponse
	AccessToken  string
	RefreshToken string
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, input *RegisterInput) (*RegisterOutput, error) {
	// 1. Validate input
	if err := s.validateRegisterInput(input); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 2. Check if email already exists in database (source of truth)
	emailExists, err := s.userRepo.EmailExists(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if emailExists {
		return nil, fmt.Errorf("email already registered")
	}

	// 3. Check if username already exists in database
	usernameExists, err := s.userRepo.UsernameExists(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if usernameExists {
		return nil, fmt.Errorf("username already taken")
	}

	// 4. Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 5. Create user entity
	user := &domain.User{
		UserID:       uuid.New(),
		Email:        input.Email,
		Username:     input.Username,
		PasswordHash: string(passwordHash),
		DisplayName:  input.DisplayName,
		Status:       "offline",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 6. Save to CockroachDB
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 7. Update Redis directory for fast lookups
	if err := s.directoryRepo.SetEmailToUserID(ctx, user.Email, user.UserID); err != nil {
		return nil, fmt.Errorf("failed to update email directory: %w", err)
	}

	if err := s.directoryRepo.SetUsernameToUserID(ctx, user.Username, user.UserID); err != nil {
		return nil, fmt.Errorf("failed to update username directory: %w", err)
	}

	// 8. Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.UserID, user.Email, user.Username, "user")
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 9. Store session in Redis
	session := &redis.Session{
		SessionID:    uuid.New().String(),
		UserID:       user.UserID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(constants.SessionExpiry),
	}

	if err := s.sessionRepo.CreateSession(ctx, session, constants.SessionExpiry); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &RegisterOutput{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// LoginInput contains login credentials
type LoginInput struct {
	Email    string
	Password string
}

// LoginOutput contains login result
type LoginOutput struct {
	User         *domain.UserResponse
	AccessToken  string
	RefreshToken string
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
	// 1. Get user by email
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// 2. Compare password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// 3. Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.UserID, user.Email, user.Username, "user")
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 4. Store session
	session := &redis.Session{
		SessionID:    uuid.New().String(),
		UserID:       user.UserID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(constants.SessionExpiry),
	}

	if err := s.sessionRepo.CreateSession(ctx, session, constants.SessionExpiry); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// 5. Update user status to online
	if err := s.userRepo.UpdateStatus(ctx, user.UserID, "online"); err != nil {
		// Non-critical, log but don't fail
		return &LoginOutput{
			User:         user.ToResponse(),
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}, nil
	}

	return &LoginOutput{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RefreshTokenInput contains refresh token
type RefreshTokenInput struct {
	RefreshToken string
}

// RefreshTokenOutput contains new tokens
type RefreshTokenOutput struct {
	AccessToken  string
	RefreshToken string
}

// RefreshToken generates new access token from refresh token
func (s *Service) RefreshToken(ctx context.Context, input *RefreshTokenInput) (*RefreshTokenOutput, error) {
	// 1. Validate refresh token
	claims, err := s.jwtManager.ValidateToken(input.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// 2. Get user to ensure they still exist
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// 3. Generate new tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.UserID, user.Email, user.Username, "user")
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &RefreshTokenOutput{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// Logout invalidates user session and blacklists token
func (s *Service) Logout(ctx context.Context, sessionID string, userID uuid.UUID, tokenString string) error {
	// 1. Validate session belongs to user
	session, err := s.sessionRepo.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	if session.UserID != userID {
		return fmt.Errorf("unauthorized: session does not belong to user")
	}

	// 2. Delete session
	if err := s.sessionRepo.DeleteSession(ctx, sessionID, userID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// 3. Update user status to offline in CockroachDB
	if err := s.userRepo.UpdateStatus(ctx, userID, "offline"); err != nil {
		// Log but don't fail - session is already deleted
		logger.Warn("Failed to update user status during logout",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}

	// 4. Remove from presence in Redis
	if err := s.presenceRepo.SetUserOffline(ctx, userID); err != nil {
		// Log but don't fail - session is already deleted
		logger.Warn("Failed to update user presence during logout",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}

	// 5. Extract JTI and blacklist token
	// We parse unverified because we trust the source (AuthMiddleware already validated signature)
	// or even if we don't, we just want to block THIS string.
	// However, extracting claims is safer.
	claims, err := s.jwtManager.ValidateToken(tokenString)
	if err == nil && claims.ID != "" {
		// Calculate remaining time
		expiresIn := time.Until(claims.ExpiresAt.Time)
		if expiresIn > 0 {
			if err := s.sessionRepo.BlacklistToken(ctx, claims.ID, expiresIn); err != nil {
				// Log but don't fail, session is already deleted
				logger.Warn("Failed to blacklist token during logout",
					zap.String("user_id", userID.String()),
					zap.String("jti", claims.ID),
					zap.Error(err))
			}
		}
	}

	return nil
}

// IsTokenRevoked checks if a token has been blacklisted
func (s *Service) IsTokenRevoked(ctx context.Context, tokenString string) (bool, error) {
	// Extract JTI
	// For performance, we might want a lightweight extraction without full validation if middleware did it?
	// But duplicate validation is safer.
	claims, err := s.jwtManager.ValidateToken(tokenString)
	if err != nil {
		// If invalid, it's effectively revoked/useless
		return true, nil
	}

	if claims.ID == "" {
		// Tokens without ID cannot be blacklisted (old tokens?)
		return false, nil
	}

	return s.sessionRepo.IsTokenBlacklisted(ctx, claims.ID)
}

// validateRegisterInput validates registration input
func (s *Service) validateRegisterInput(input *RegisterInput) error {
	if input.Email == "" {
		return fmt.Errorf("email is required")
	}
	if input.Username == "" || len(input.Username) < constants.MinUsernameLength {
		return fmt.Errorf("username must be at least %d characters", constants.MinUsernameLength)
	}
	if input.Password == "" || len(input.Password) < constants.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", constants.MinPasswordLength)
	}
	if input.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}
	return nil
}

// FailedLoginAttempt represents a failed login attempt
type FailedLoginAttempt struct {
	UserID      uuid.UUID
	Email       string
	IP          string
	Attempts    int
	LockedUntil *time.Time
}

// checkAccountLocked checks if an account is locked
func (s *Service) checkAccountLocked(ctx context.Context, email string) (bool, error) {
	key := fmt.Sprintf("failed_login:%s", email)

	// Check if account is locked
	locked, err := s.sessionRepo.GetAccountLock(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check account lock: %w", err)
	}

	if locked != nil && time.Now().Before(locked.LockedUntil) {
		return true, nil
	}

	return false, nil
}

// recordFailedLogin records a failed login attempt
func (s *Service) recordFailedLogin(ctx context.Context, email, ip string, userID uuid.UUID) error {
	key := fmt.Sprintf("failed_login:%s", email)

	// Get current attempts
	attempts, err := s.sessionRepo.GetFailedLoginAttempts(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get login attempts: %w", err)
	}

	attempts++

	// Check if should lock account
	if attempts >= constants.MaxFailedLoginAttempts {
		lockedUntil := time.Now().Add(constants.AccountLockDuration)
		// Store full failed login attempt information including IP
		attempt := &redis.FailedLoginAttempt{
			UserID:      userID,
			Email:       email,
			IP:          ip,
			Attempts:    attempts,
			LockedUntil: &lockedUntil,
		}
		if err := s.sessionRepo.SetFailedLoginAttempt(ctx, key, attempt); err != nil {
			return fmt.Errorf("failed to set failed login attempt: %w", err)
		}
		if err := s.sessionRepo.LockAccount(ctx, key, lockedUntil); err != nil {
			return fmt.Errorf("failed to lock account: %w", err)
		}
	} else {
		// Update attempts with IP information
		attempt := &redis.FailedLoginAttempt{
			UserID:   userID,
			Email:    email,
			IP:       ip,
			Attempts: attempts,
		}
		if err := s.sessionRepo.SetFailedLoginAttempt(ctx, key, attempt); err != nil {
			return fmt.Errorf("failed to set failed login attempt: %w", err)
		}
	}

	return nil
}

// RequestPasswordResetInput contains data for password reset request
type RequestPasswordResetInput struct {
	Email string
}

// RequestPasswordReset initiates password reset flow
func (s *Service) RequestPasswordReset(ctx context.Context, input *RequestPasswordResetInput) error {
	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		// Don't reveal if user exists or not - return generic message
		logger.Info("Password reset requested for non-existent email",
			zap.String("email", input.Email))
		return nil
	}

	// Generate reset token
	token, err := generateToken()
	if err != nil {
		logger.Error("Failed to generate password reset token",
			zap.String("user_id", user.UserID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to generate reset token")
	}

	// Create token with 1 hour expiration
	expiresAt := time.Now().Add(1 * time.Hour)
	err = s.emailVerificationRepo.CreateToken(ctx, user.UserID, "", token, expiresAt)
	if err != nil {
		logger.Error("Failed to create password reset token",
			zap.String("user_id", user.UserID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to create reset token")
	}

	// Send password reset email
	err = s.emailService.SendPasswordResetEmail(ctx, user.Email, &email.PasswordResetEmailData{
		Username: user.Username,
		Token:    token,
		AppURL:   env.GetString("APP_URL", "http://localhost:9090"),
	})
	if err != nil {
		logger.Error("Failed to send password reset email",
			zap.String("user_id", user.UserID.String()),
			zap.String("email", user.Email),
			zap.Error(err))
		// Don't fail - token is created, user can request again
		return nil
	}

	logger.Info("Password reset email sent",
		zap.String("user_id", user.UserID.String()),
		zap.String("email", user.Email))

	return nil
}

// ResetPasswordInput contains data for password reset
type ResetPasswordInput struct {
	Token       string
	NewPassword string
}

// ResetPassword completes password reset flow
func (s *Service) ResetPassword(ctx context.Context, input *ResetPasswordInput) error {
	// Get token
	evt, err := s.emailVerificationRepo.GetToken(ctx, input.Token)
	if err != nil {
		logger.Info("Invalid password reset token used",
			zap.String("token_prefix", maskToken(input.Token)))
		return fmt.Errorf("invalid or expired token")
	}

	// Check if token is expired
	if time.Now().After(evt.ExpiresAt) {
		logger.Info("Expired password reset token used",
			zap.String("token_prefix", maskToken(input.Token)),
			zap.String("user_id", evt.UserID.String()))
		return fmt.Errorf("token has expired")
	}

	// Check if token is already used
	if evt.UsedAt != nil {
		logger.Info("Already used password reset token attempted",
			zap.String("token_prefix", maskToken(input.Token)),
			zap.String("user_id", evt.UserID.String()))
		return fmt.Errorf("token already used")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, evt.UserID)
	if err != nil {
		logger.Error("Failed to get user for password reset",
			zap.String("user_id", evt.UserID.String()),
			zap.Error(err))
		return fmt.Errorf("user not found")
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Failed to hash new password",
			zap.String("user_id", user.UserID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to process password")
	}

	// Update user password
	user.PasswordHash = string(passwordHash)
	user.UpdatedAt = time.Now()
	err = s.userRepo.Update(ctx, user)
	if err != nil {
		logger.Error("Failed to update user password",
			zap.String("user_id", user.UserID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to update password")
	}

	// Mark token as used
	err = s.emailVerificationRepo.MarkTokenUsed(ctx, input.Token)
	if err != nil {
		logger.Warn("Failed to mark password reset token as used",
			zap.String("token_prefix", maskToken(input.Token)),
			zap.String("user_id", evt.UserID.String()),
			zap.Error(err))
		// Don't fail - password is already updated
	}

	logger.Info("Password reset completed",
		zap.String("user_id", user.UserID.String()))

	return nil
}

// generateToken generates a random token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// maskToken returns a safe masked version of a token for logging
// Shows only first 4 and last 4 characters, with middle masked
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// clearFailedLoginAttempts clears failed login attempts on successful login
func (s *Service) clearFailedLoginAttempts(ctx context.Context, email string) error {
	key := fmt.Sprintf("failed_login:%s", email)

	if err := s.sessionRepo.DeleteFailedLoginAttempts(ctx, key); err != nil {
		// Log but don't fail
		logger.Warn("Failed to clear failed login attempts",
			zap.String("email", email),
			zap.Error(err))
	}

	return nil
}

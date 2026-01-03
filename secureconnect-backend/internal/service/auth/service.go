package auth

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	
	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/internal/repository/redis"
	"secureconnect-backend/pkg/jwt"
)

// Service handles authentication business logic
type Service struct {
	userRepo      *cockroach.UserRepository
	directoryRepo *redis.DirectoryRepository
	sessionRepo   *redis.SessionRepository
	jwtManager    *jwt.JWTManager
}

// NewService creates a new auth service
func NewService(
	userRepo *cockroach.UserRepository,
	directoryRepo *redis.DirectoryRepository,
	sessionRepo *redis.SessionRepository,
	jwtManager *jwt.JWTManager,
) *Service {
	return &Service{
		userRepo:      userRepo,
		directoryRepo: directoryRepo,
		sessionRepo:   sessionRepo,
		jwtManager:    jwtManager,
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
	
	// 2. Check if email already exists (fast check via Redis directory)
	emailExists, err := s.directoryRepo.EmailExists(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if emailExists {
		return nil, fmt.Errorf("email already registered")
	}
	
	// 3. Check if username already exists
	usernameExists, err := s.directoryRepo.UsernameExists(ctx, input.Username)
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
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour), // 30 days
	}
	
	if err := s.sessionRepo.CreateSession(ctx, session, 30*24*time.Hour); err != nil {
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
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
	}
	
	if err := s.sessionRepo.CreateSession(ctx, session, 30*24*time.Hour); err != nil {
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

// Logout invalidates user session
func (s *Service) Logout(ctx context.Context, sessionID string, userID uuid.UUID) error {
	return s.sessionRepo.DeleteSession(ctx, sessionID, userID)
}

// validateRegisterInput validates registration input
func (s *Service) validateRegisterInput(input *RegisterInput) error {
	if input.Email == "" {
		return fmt.Errorf("email is required")
	}
	if input.Username == "" || len(input.Username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if input.Password == "" || len(input.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if input.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}
	return nil
}

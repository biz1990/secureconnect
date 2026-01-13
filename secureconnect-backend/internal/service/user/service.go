package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/email"
)

// Service handles user business logic
type Service struct {
	userRepo              *cockroach.UserRepository
	blockedUserRepo       *cockroach.BlockedUserRepository
	emailVerificationRepo *cockroach.EmailVerificationRepository
	emailService          *email.Service
}

// NewService creates a new user service
func NewService(
	userRepo *cockroach.UserRepository,
	blockedUserRepo *cockroach.BlockedUserRepository,
	emailVerificationRepo *cockroach.EmailVerificationRepository,
	emailService *email.Service,
) *Service {
	return &Service{
		userRepo:              userRepo,
		blockedUserRepo:       blockedUserRepo,
		emailVerificationRepo: emailVerificationRepo,
		emailService:          emailService,
	}
}

// GetProfile retrieves user profile by ID
func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateProfile updates user profile information
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName *string, avatarURL *string) error {
	// Get current user to validate
	currentUser, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Handle optional displayName - use new value if provided, otherwise keep existing
	displayNameValue := currentUser.DisplayName
	if displayName != nil {
		displayNameValue = *displayName
	}

	// Handle optional avatarURL - use new value if provided, otherwise keep existing
	avatarURLValue := currentUser.AvatarURL
	if avatarURL != nil {
		avatarURLValue = avatarURL
	}

	// Build update struct
	update := &domain.User{
		UserID:       currentUser.UserID,
		Email:        currentUser.Email,
		Username:     currentUser.Username,
		PasswordHash: currentUser.PasswordHash,
		DisplayName:  displayNameValue,
		AvatarURL:    avatarURLValue,
		Status:       currentUser.Status,
	}

	return s.userRepo.Update(ctx, update)
}

// ChangePassword changes user password
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	// Get current user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("invalid old password")
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	update := &domain.User{
		UserID:       user.UserID,
		Email:        user.Email,
		Username:     user.Username,
		PasswordHash: string(passwordHash),
		DisplayName:  user.DisplayName,
		AvatarURL:    user.AvatarURL,
		Status:       user.Status,
		UpdatedAt:    time.Now(),
	}

	return s.userRepo.Update(ctx, update)
}

// InitiateEmailChange initiates email change process
func (s *Service) InitiateEmailChange(ctx context.Context, userID uuid.UUID, newEmail, password string) error {
	// Verify user password first
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return fmt.Errorf("invalid password")
	}

	// Validate new email is not already taken
	exists, err := s.userRepo.EmailExists(ctx, newEmail)
	if err != nil {
		return fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return fmt.Errorf("email already in use")
	}

	// Generate verification token
	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Create token with 24 hour expiration
	expiresAt := time.Now().Add(constants.EmailVerificationExpiry)
	err = s.emailVerificationRepo.CreateToken(ctx, userID, newEmail, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}

	// Send verification email
	// Get user's display name for personalization
	userInfo, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Send email with verification token
	err = s.emailService.SendVerificationEmail(ctx, newEmail, &email.VerificationEmailData{
		Username: userInfo.Username,
		Token:    token,
		NewEmail: newEmail,
		AppURL:   "", // TODO: Get from config
	})
	if err != nil {
		// Log error but don't fail - token is still created
		// User can request another verification email
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}

// VerifyEmailChange verifies and completes email change
func (s *Service) VerifyEmailChange(ctx context.Context, userID uuid.UUID, token string) error {
	// Get token
	evt, err := s.emailVerificationRepo.GetToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	// Verify token belongs to user
	if evt.UserID != userID {
		return fmt.Errorf("token does not belong to user")
	}

	// Check if token is expired
	if time.Now().After(evt.ExpiresAt) {
		return fmt.Errorf("token has expired")
	}

	// Check if token is already used
	if evt.UsedAt != nil {
		return fmt.Errorf("token already used")
	}

	// Mark token as used
	if err := s.emailVerificationRepo.MarkTokenUsed(ctx, token); err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	// Update user email
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	user.Email = evt.NewEmail
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

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

// DeleteAccount deletes user account (soft delete recommended)
func (s *Service) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	// Soft delete by setting status to deleted
	err := s.userRepo.UpdateStatus(ctx, userID, "deleted")
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}
	return nil
}

// GetBlockedUsers retrieves list of blocked users
func (s *Service) GetBlockedUsers(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	return s.blockedUserRepo.GetBlockedUsers(ctx, userID, limit, offset)
}

// BlockUser blocks another user
func (s *Service) BlockUser(ctx context.Context, requestingUserID, targetUserID uuid.UUID, reason string) error {
	// Cannot block yourself
	if requestingUserID == targetUserID {
		return fmt.Errorf("cannot block yourself")
	}

	// Check if target user exists
	_, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("target user not found: %w", err)
	}

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	return s.blockedUserRepo.BlockUser(ctx, requestingUserID, targetUserID, reasonPtr)
}

// UnblockUser unblocks a user
func (s *Service) UnblockUser(ctx context.Context, requestingUserID, targetUserID uuid.UUID) error {
	return s.blockedUserRepo.UnblockUser(ctx, requestingUserID, targetUserID)
}

// GetFriends retrieves list of friends
func (s *Service) GetFriends(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	return s.userRepo.GetFriends(ctx, userID, limit, offset)
}

// GetFriendRequests retrieves incoming friend requests
func (s *Service) GetFriendRequests(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	return s.userRepo.GetFriendRequests(ctx, userID, limit, offset)
}

// SendFriendRequest sends a friend request
func (s *Service) SendFriendRequest(ctx context.Context, requestingUserID, targetUserID uuid.UUID) error {
	// Cannot send friend request to yourself
	if requestingUserID == targetUserID {
		return fmt.Errorf("cannot send friend request to yourself")
	}

	// Check if target user exists
	_, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("target user not found: %w", err)
	}

	// Check if already friends or pending
	status, err := s.userRepo.GetFriendship(ctx, requestingUserID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check friendship status: %w", err)
	}

	if status != "" {
		if status == "accepted" {
			return fmt.Errorf("already friends")
		}
		if status == "pending" {
			return fmt.Errorf("friend request already pending")
		}
		if status == "blocked" {
			return fmt.Errorf("cannot send friend request to blocked user")
		}
	}

	return s.userRepo.CreateFriendRequest(ctx, requestingUserID, targetUserID)
}

// AcceptFriendRequest accepts a friend request
func (s *Service) AcceptFriendRequest(ctx context.Context, userID uuid.UUID, friendID uuid.UUID) error {
	// Check if friendship exists and is pending
	status, err := s.userRepo.GetFriendship(ctx, userID, friendID)
	if err != nil {
		return fmt.Errorf("failed to check friendship status: %w", err)
	}

	if status != "pending" {
		return fmt.Errorf("no pending friend request found")
	}

	return s.userRepo.UpdateFriendshipStatus(ctx, userID, friendID, "accepted")
}

// RejectFriendRequest rejects a friend request
func (s *Service) RejectFriendRequest(ctx context.Context, userID uuid.UUID, friendID uuid.UUID) error {
	// Check if friendship exists and is pending
	status, err := s.userRepo.GetFriendship(ctx, userID, friendID)
	if err != nil {
		return fmt.Errorf("failed to check friendship status: %w", err)
	}

	if status != "pending" {
		return fmt.Errorf("no pending friend request found")
	}

	return s.userRepo.UpdateFriendshipStatus(ctx, userID, friendID, "rejected")
}

// Unfriend removes a friend relationship
func (s *Service) Unfriend(ctx context.Context, userID uuid.UUID, friendID uuid.UUID) error {
	// Check if friendship exists and is accepted
	status, err := s.userRepo.GetFriendship(ctx, userID, friendID)
	if err != nil {
		return fmt.Errorf("failed to check friendship status: %w", err)
	}

	if status != "accepted" {
		return fmt.Errorf("not friends with this user")
	}

	return s.userRepo.DeleteFriendship(ctx, userID, friendID)
}

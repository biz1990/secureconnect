package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/redis"
	"secureconnect-backend/pkg/jwt"
)

// Mocks
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) UpdateStatus(ctx context.Context, userID uuid.UUID, status string) error {
	args := m.Called(ctx, userID, status)
	return args.Error(0)
}

func (m *MockUserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

type MockDirectoryRepository struct {
	mock.Mock
}

func (m *MockDirectoryRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockDirectoryRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *MockDirectoryRepository) SetEmailToUserID(ctx context.Context, email string, userID uuid.UUID) error {
	args := m.Called(ctx, email, userID)
	return args.Error(0)
}

func (m *MockDirectoryRepository) SetUsernameToUserID(ctx context.Context, username string, userID uuid.UUID) error {
	args := m.Called(ctx, username, userID)
	return args.Error(0)
}

type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) CreateSession(ctx context.Context, session *redis.Session, ttl time.Duration) error {
	args := m.Called(ctx, session, ttl)
	return args.Error(0)
}

func (m *MockSessionRepository) GetSession(ctx context.Context, sessionID string) (*redis.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*redis.Session), args.Error(1)
}

func (m *MockSessionRepository) DeleteSession(ctx context.Context, sessionID string, userID uuid.UUID) error {
	args := m.Called(ctx, sessionID, userID)
	return args.Error(0)
}

func (m *MockSessionRepository) BlacklistToken(ctx context.Context, jti string, expiresAt time.Duration) error {
	args := m.Called(ctx, jti, expiresAt)
	return args.Error(0)
}

func (m *MockSessionRepository) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	args := m.Called(ctx, jti)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionRepository) GetAccountLock(ctx context.Context, key string) (*redis.AccountLock, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*redis.AccountLock), args.Error(1)
}

func (m *MockSessionRepository) LockAccount(ctx context.Context, key string, lockedUntil time.Time) error {
	args := m.Called(ctx, key, lockedUntil)
	return args.Error(0)
}

func (m *MockSessionRepository) GetFailedLoginAttempts(ctx context.Context, key string) (int, error) {
	args := m.Called(ctx, key)
	return args.Int(0), args.Error(1)
}

func (m *MockSessionRepository) SetFailedLoginAttempts(ctx context.Context, key string, attempts int) error {
	args := m.Called(ctx, key, attempts)
	return args.Error(0)
}

func (m *MockSessionRepository) DeleteFailedLoginAttempts(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockSessionRepository) GetFailedLoginAttempt(ctx context.Context, key string) (*redis.FailedLoginAttempt, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*redis.FailedLoginAttempt), args.Error(1)
}

func (m *MockSessionRepository) SetFailedLoginAttempt(ctx context.Context, key string, attempt *redis.FailedLoginAttempt) error {
	args := m.Called(ctx, key, attempt)
	return args.Error(0)
}

type MockPresenceRepository struct {
	mock.Mock
}

func (m *MockPresenceRepository) SetUserOnline(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockPresenceRepository) SetUserOffline(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func TestRegister(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockDirRepo := new(MockDirectoryRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockPresenceRepo := new(MockPresenceRepository)
	jwtManager := jwt.NewJWTManager("secret", 15*time.Minute, 24*time.Hour)

	service := NewService(mockUserRepo, mockDirRepo, mockSessionRepo, mockPresenceRepo, jwtManager)

	input := &RegisterInput{
		Email:       "test@example.com",
		Username:    "testuser",
		Password:    "password123",
		DisplayName: "Test User",
	}

	ctx := context.Background()

	// Expectations
	mockDirRepo.On("EmailExists", ctx, input.Email).Return(false, nil)
	mockDirRepo.On("UsernameExists", ctx, input.Username).Return(false, nil)
	mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
	mockDirRepo.On("SetEmailToUserID", ctx, input.Email, mock.AnythingOfType("uuid.UUID")).Return(nil)
	mockDirRepo.On("SetUsernameToUserID", ctx, input.Username, mock.AnythingOfType("uuid.UUID")).Return(nil)
	mockSessionRepo.On("CreateSession", ctx, mock.AnythingOfType("*redis.Session"), mock.Anything).Return(nil)

	// Execute
	output, err := service.Register(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, input.Email, output.User.Email)
	assert.NotEmpty(t, output.AccessToken)

	mockUserRepo.AssertExpectations(t)
	mockDirRepo.AssertExpectations(t)
	mockSessionRepo.AssertExpectations(t)
}

func TestRegister_EmailExists(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockDirRepo := new(MockDirectoryRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockPresenceRepo := new(MockPresenceRepository)
	jwtManager := jwt.NewJWTManager("secret", 15*time.Minute, 24*time.Hour)

	service := NewService(mockUserRepo, mockDirRepo, mockSessionRepo, mockPresenceRepo, jwtManager)

	input := &RegisterInput{
		Email:       "existing@example.com",
		Username:    "newuser",
		Password:    "password123",
		DisplayName: "Test User",
	}

	ctx := context.Background()

	// Expectations
	mockDirRepo.On("EmailExists", ctx, input.Email).Return(true, nil)

	// Execute
	output, err := service.Register(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "email already registered")

	mockDirRepo.AssertExpectations(t)
}

package jwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewJWTManager(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes"
	accessExpiry := 15 * time.Minute
	refreshExpiry := 24 * time.Hour

	manager := NewJWTManager(secret, accessExpiry, refreshExpiry)

	assert.NotNil(t, manager)
	assert.Equal(t, secret, manager.secretKey)
	assert.Equal(t, accessExpiry, manager.accessTokenDuration)
	assert.Equal(t, refreshExpiry, manager.refreshTokenDuration)
}

func TestGenerateAccessToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "test@example.com", "testuser", "user")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenerateRefreshToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestValidateToken_ValidToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	// Generate token
	token, err := manager.GenerateAccessToken(userID, "test@example.com", "testuser", "user")
	assert.NoError(t, err)

	// Validate token
	claims, err := manager.ValidateToken(token)

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "user", claims.Role)
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	// Create manager with very short expiry
	manager := NewJWTManager("test-secret", 1*time.Nanosecond, 24*time.Hour)
	userID := uuid.New()

	// Generate token
	token, err := manager.GenerateAccessToken(userID, "test@example.com", "testuser", "user")
	assert.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Validate expired token
	claims, err := manager.ValidateToken(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateToken_InvalidToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)

	invalidToken := "invalid.token.here"

	claims, err := manager.ValidateToken(invalidToken)

	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	// Generate with one secret
	manager1 := NewJWTManager("secret-1", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()
	token, err := manager1.GenerateAccessToken(userID, "test@example.com", "testuser", "user")
	assert.NoError(t, err)

	// Validate with different secret
	manager2 := NewJWTManager("secret-2", 15*time.Minute, 24*time.Hour)
	claims, err := manager2.ValidateToken(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestExtractUserID(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	// Generate token
	token, err := manager.GenerateAccessToken(userID, "test@example.com", "testuser", "user")
	assert.NoError(t, err)

	// Extract user ID
	extractedID, err := manager.ExtractUserID(token)
	assert.NoError(t, err)
	assert.Equal(t, userID, extractedID)
}

func TestTokenClaims(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "test@example.com", "testuser", "admin")
	assert.NoError(t, err)

	claims, err := manager.ValidateToken(token)
	assert.NoError(t, err)

	// Verify claims structure
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "admin", claims.Role)
	assert.NotZero(t, claims.IssuedAt)
	assert.NotZero(t, claims.ExpiresAt)
	assert.True(t, claims.ExpiresAt.After(claims.IssuedAt.Time))
	assert.Equal(t, "secureconnect-auth", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestIsTokenExpired(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Nanosecond, 24*time.Hour)
	userID := uuid.New()

	// Generate token with very short expiry
	token, err := manager.GenerateAccessToken(userID, "test@example.com", "testuser", "user")
	assert.NoError(t, err)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Check if expired
	expired := IsTokenExpired(token)
	assert.True(t, expired)
}

func TestRefreshTokenClaims(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)
	assert.NoError(t, err)

	claims, err := manager.ValidateToken(token)
	assert.NoError(t, err)

	// Verify refresh token has minimal claims
	assert.Equal(t, userID, claims.UserID)
	assert.Empty(t, claims.Email)    // Refresh tokens don't need email
	assert.Empty(t, claims.Username) // Or username
	assert.NotZero(t, claims.ExpiresAt)
}

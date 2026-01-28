package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"secureconnect-backend/internal/database"
	"secureconnect-backend/pkg/constants"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SessionRepository handles user session management in Redis
type SessionRepository struct {
	client        *database.RedisClient
	inMemoryCache *InMemorySessionCache
}

// InMemorySessionCache provides fallback storage when Redis is degraded
type InMemorySessionCache struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	userIndex map[uuid.UUID][]string // user_id -> session_ids
}

// NewInMemorySessionCache creates a new in-memory session cache
func NewInMemorySessionCache() *InMemorySessionCache {
	return &InMemorySessionCache{
		sessions:  make(map[string]*Session),
		userIndex: make(map[uuid.UUID][]string),
	}
}

// Set stores a session in memory
func (c *InMemorySessionCache) Set(sessionID string, session *Session, userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[sessionID] = session
	c.userIndex[userID] = append(c.userIndex[userID], sessionID)
}

// Get retrieves a session from memory
func (c *InMemorySessionCache) Get(sessionID string) (*Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	session, exists := c.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

// Delete removes a session from memory
func (c *InMemorySessionCache) Delete(sessionID string, userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, sessionID)
	if sessions, exists := c.userIndex[userID]; exists {
		for i, sid := range sessions {
			if sid == sessionID {
				c.userIndex[userID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
	}
}

// NewSessionRepository creates a new SessionRepository with in-memory fallback
func NewSessionRepository(client *database.RedisClient) *SessionRepository {
	return &SessionRepository{
		client:        client,
		inMemoryCache: NewInMemorySessionCache(),
	}
}

// Session represents a user session
type Session struct {
	SessionID    string    `json:"session_id"`
	UserID       uuid.UUID `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// CreateSession stores a new session
func (r *SessionRepository) CreateSession(ctx context.Context, session *Session, ttl time.Duration) error {
	// If Redis is degraded, use in-memory fallback
	if r.client.IsDegraded() {
		r.inMemoryCache.Set(session.SessionID, session, session.UserID)
		return nil
	}

	key := fmt.Sprintf("session:%s", session.SessionID)

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	err = r.client.SafeSet(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Also store user_id -> session_id mapping for quick lookup
	userSessionKey := fmt.Sprintf("user:sessions:%s", session.UserID)
	err = r.client.SafeSAdd(ctx, userSessionKey, session.SessionID).Err()
	if err != nil {
		return fmt.Errorf("failed to add session to user index: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID
func (r *SessionRepository) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	// If Redis is degraded, use in-memory fallback
	if r.client.IsDegraded() {
		return r.inMemoryCache.Get(sessionID)
	}

	key := fmt.Sprintf("session:%s", sessionID)

	data, err := r.client.SafeGet(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	err = json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session
func (r *SessionRepository) DeleteSession(ctx context.Context, sessionID string, userID uuid.UUID) error {
	// If Redis is degraded, use in-memory fallback
	if r.client.IsDegraded() {
		r.inMemoryCache.Delete(sessionID, userID)
		return nil
	}

	key := fmt.Sprintf("session:%s", sessionID)

	err := r.client.SafeDel(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove from user index
	userSessionKey := fmt.Sprintf("user:sessions:%s", userID)
	r.client.SafeSRem(ctx, userSessionKey, sessionID)

	return nil
}

// DeleteAllUserSessions removes all sessions for a user
func (r *SessionRepository) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	userSessionKey := fmt.Sprintf("user:sessions:%s", userID)

	sessionIDs, err := r.client.SafeSMembers(ctx, userSessionKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Delete each session
	for _, sessionID := range sessionIDs {
		key := fmt.Sprintf("session:%s", sessionID)
		r.client.SafeDel(ctx, key)
	}

	// Delete user index
	r.client.SafeDel(ctx, userSessionKey)

	return nil
}

// RefreshSessionTTL extends session expiration
func (r *SessionRepository) RefreshSessionTTL(ctx context.Context, sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return r.client.SafeExpire(ctx, key, ttl).Err()
}

// BlacklistToken adds a token JTI to the blacklist with expiration
func (r *SessionRepository) BlacklistToken(ctx context.Context, jti string, expiresAt time.Duration) error {
	key := fmt.Sprintf("blacklist:%s", jti)
	return r.client.SafeSet(ctx, key, "revoked", expiresAt).Err()
}

// IsTokenBlacklisted checks if a token JTI is in the blacklist
func (r *SessionRepository) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", jti)
	exists, err := r.client.SafeExists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return exists > 0, nil
}

// AccountLock represents a locked account
type AccountLock struct {
	LockedUntil time.Time `json:"locked_until"`
}

// FailedLoginAttempt represents a failed login attempt
type FailedLoginAttempt struct {
	UserID      uuid.UUID  `json:"user_id"`
	Email       string     `json:"email"`
	IP          string     `json:"ip"`
	Attempts    int        `json:"attempts"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// GetAccountLock retrieves account lock status
func (r *SessionRepository) GetAccountLock(ctx context.Context, key string) (*AccountLock, error) {
	data, err := r.client.SafeGet(ctx, key).Result()
	if err != nil {
		// Handle Redis key not found gracefully - no lock exists
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account lock: %w", err)
	}

	if data == "" {
		return nil, nil
	}

	// Try JSON unmarshal first (new format)
	var accountLock AccountLock
	err = json.Unmarshal([]byte(data), &accountLock)
	if err == nil {
		return &accountLock, nil
	}

	// Fallback: Try parsing as Unix timestamp (backward compatibility with stale data)
	var unixTimestamp int64
	_, err = fmt.Sscanf(data, "%d", &unixTimestamp)
	if err == nil {
		// Successfully parsed as Unix timestamp - convert to time.Time
		lockedUntil := time.Unix(unixTimestamp, 0)
		return &AccountLock{LockedUntil: lockedUntil}, nil
	}

	// Both formats failed - data is corrupted
	return nil, fmt.Errorf("failed to parse account lock: invalid format (neither JSON nor Unix timestamp)")
}

// LockAccount locks an account
func (r *SessionRepository) LockAccount(ctx context.Context, key string, lockedUntil time.Time) error {
	// Use JSON format for consistency and proper serialization
	accountLock := &AccountLock{LockedUntil: lockedUntil}
	data, err := json.Marshal(accountLock)
	if err != nil {
		return fmt.Errorf("failed to marshal account lock: %w", err)
	}
	err = r.client.SafeSet(ctx, key, data, constants.AccountLockDuration).Err()
	if err != nil {
		return fmt.Errorf("failed to lock account: %w", err)
	}
	return nil
}

// GetFailedLoginAttempts retrieves failed login attempts
func (r *SessionRepository) GetFailedLoginAttempts(ctx context.Context, key string) (int, error) {
	data, err := r.client.SafeGet(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get failed login attempts: %w", err)
	}

	var attempts int
	if data != "" {
		_, err := fmt.Sscanf(data, "%d", &attempts)
		if err != nil {
			return 0, err
		}
	}

	return attempts, nil
}

// GetFailedLoginAttempt retrieves full failed login attempt information
func (r *SessionRepository) GetFailedLoginAttempt(ctx context.Context, key string) (*FailedLoginAttempt, error) {
	data, err := r.client.SafeGet(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get failed login attempt: %w", err)
	}

	if data == "" {
		return nil, nil
	}

	var attempt FailedLoginAttempt
	err = json.Unmarshal([]byte(data), &attempt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse failed login attempt: %w", err)
	}

	return &attempt, nil
}

// SetFailedLoginAttempts sets failed login attempts
func (r *SessionRepository) SetFailedLoginAttempts(ctx context.Context, key string, attempts int) error {
	data := fmt.Sprintf("%d", attempts)
	err := r.client.SafeSet(ctx, key, data, constants.FailedLoginWindow).Err()
	if err != nil {
		return fmt.Errorf("failed to set failed login attempts: %w", err)
	}
	return nil
}

// SetFailedLoginAttempt stores full failed login attempt information
func (r *SessionRepository) SetFailedLoginAttempt(ctx context.Context, key string, attempt *FailedLoginAttempt) error {
	data, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("failed to marshal failed login attempt: %w", err)
	}

	err = r.client.SafeSet(ctx, key, data, constants.FailedLoginWindow).Err()
	if err != nil {
		return fmt.Errorf("failed to set failed login attempt: %w", err)
	}
	return nil
}

// DeleteFailedLoginAttempts deletes failed login attempts
func (r *SessionRepository) DeleteFailedLoginAttempts(ctx context.Context, key string) error {
	err := r.client.SafeDel(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete failed login attempts: %w", err)
	}
	return nil
}

// IsDegraded returns true if Redis is in degraded mode
func (r *SessionRepository) IsDegraded() bool {
	return r.client.IsDegraded()
}

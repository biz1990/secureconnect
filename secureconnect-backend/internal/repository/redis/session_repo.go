package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SessionRepository handles user session management in Redis
type SessionRepository struct {
	client *redis.Client
}

// NewSessionRepository creates a new SessionRepository
func NewSessionRepository(client *redis.Client) *SessionRepository {
	return &SessionRepository{client: client}
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
	key := fmt.Sprintf("session:%s", session.SessionID)
	
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	
	err = r.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	// Also store user_id -> session_id mapping for quick lookup
	userSessionKey := fmt.Sprintf("user:sessions:%s", session.UserID)
	err = r.client.SAdd(ctx, userSessionKey, session.SessionID).Err()
	if err != nil {
		return fmt.Errorf("failed to add session to user index: %w", err)
	}
	
	return nil
}

// GetSession retrieves a session by ID
func (r *SessionRepository) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	
	data, err := r.client.Get(ctx, key).Result()
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
	key := fmt.Sprintf("session:%s", sessionID)
	
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	// Remove from user index
	userSessionKey := fmt.Sprintf("user:sessions:%s", userID)
	r.client.SRem(ctx, userSessionKey, sessionID)
	
	return nil
}

// DeleteAllUserSessions removes all sessions for a user
func (r *SessionRepository) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	userSessionKey := fmt.Sprintf("user:sessions:%s", userID)
	
	sessionIDs, err := r.client.SMembers(ctx, userSessionKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}
	
	// Delete each session
	for _, sessionID := range sessionIDs {
		key := fmt.Sprintf("session:%s", sessionID)
		r.client.Del(ctx, key)
	}
	
	// Delete user index
	r.client.Del(ctx, userSessionKey)
	
	return nil
}

// RefreshSessionTTL extends session expiration
func (r *SessionRepository) RefreshSessionTTL(ctx context.Context, sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return r.client.Expire(ctx, key, ttl).Err()
}

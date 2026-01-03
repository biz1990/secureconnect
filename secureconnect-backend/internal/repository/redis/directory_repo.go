package redis

import (
	"context"
	"fmt"
	
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DirectoryRepository handles user directory (email->UserID mapping) in Redis
// This is the Global Directory for fast user lookups across sharded CockroachDB
// Per spec: docs/04-database-sharding-strategy.md
type DirectoryRepository struct {
	client *redis.Client
}

// NewDirectoryRepository creates a new DirectoryRepository
func NewDirectoryRepository(client *redis.Client) *DirectoryRepository {
	return &DirectoryRepository{client: client}
}

// SetEmailToUserID maps email to user_id for fast lookup
func (r *DirectoryRepository) SetEmailToUserID(ctx context.Context, email string, userID uuid.UUID) error {
	key := fmt.Sprintf("directory:email:%s", email)
	err := r.client.Set(ctx, key, userID.String(), 0).Err() // No expiration
	if err != nil {
		return fmt.Errorf("failed to set email mapping: %w", err)
	}
	return nil
}

// GetUserIDByEmail retrieves user_id from email
func (r *DirectoryRepository) GetUserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	key := fmt.Sprintf("directory:email:%s", email)
	
	userIDStr, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return uuid.Nil, fmt.Errorf("email not found in directory")
		}
		return uuid.Nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID format: %w", err)
	}
	
	return userID, nil
}

// SetUsernameToUserID maps username to user_id
func (r *DirectoryRepository) SetUsernameToUserID(ctx context.Context, username string, userID uuid.UUID) error {
	key := fmt.Sprintf("directory:username:%s", username)
	err := r.client.Set(ctx, key, userID.String(), 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set username mapping: %w", err)
	}
	return nil
}

// GetUserIDByUsername retrieves user_id from username
func (r *DirectoryRepository) GetUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	key := fmt.Sprintf("directory:username:%s", username)
	
	userIDStr, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return uuid.Nil, fmt.Errorf("username not found in directory")
		}
		return uuid.Nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID format: %w", err)
	}
	
	return userID, nil
}

// DeleteEmailMapping removes email->UserID mapping
func (r *DirectoryRepository) DeleteEmailMapping(ctx context.Context, email string) error {
	key := fmt.Sprintf("directory:email:%s", email)
	return r.client.Del(ctx, key).Err()
}

// DeleteUsernameMapping removes username->UserID mapping
func (r *DirectoryRepository) DeleteUsernameMapping(ctx context.Context, username string) error {
	key := fmt.Sprintf("directory:username:%s", username)
	return r.client.Del(ctx, key).Err()
}

// EmailExists checks if email exists in directory
func (r *DirectoryRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	key := fmt.Sprintf("directory:email:%s", email)
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return count > 0, nil
}

// UsernameExists checks if username exists in directory
func (r *DirectoryRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	key := fmt.Sprintf("directory:username:%s", username)
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check username existence: %w", err)
	}
	return count > 0, nil
}

package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// PresenceRepository handles user online/offline status in Redis
type PresenceRepository struct {
	client *redis.Client
}

// NewPresenceRepository creates a new PresenceRepository
func NewPresenceRepository(client *redis.Client) *PresenceRepository {
	return &PresenceRepository{client: client}
}

// SetUserOnline marks user as online
func (r *PresenceRepository) SetUserOnline(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("presence:%s", userID)

	// Set status with TTL (auto-expire after 5 minutes if not refreshed)
	err := r.client.Set(ctx, key, "online", 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to set user online: %w", err)
	}

	// Add to online users set for quick listing
	err = r.client.SAdd(ctx, "presence:online", userID.String()).Err()
	if err != nil {
		return fmt.Errorf("failed to add to online set: %w", err)
	}

	return nil
}

// SetUserOffline marks user as offline
func (r *PresenceRepository) SetUserOffline(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("presence:%s", userID)

	// Delete presence key
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete presence: %w", err)
	}

	// Remove from online set
	err = r.client.SRem(ctx, "presence:online", userID.String()).Err()
	if err != nil {
		return fmt.Errorf("failed to remove from online set: %w", err)
	}

	return nil
}

// IsUserOnline checks if user is currently online
func (r *PresenceRepository) IsUserOnline(ctx context.Context, userID uuid.UUID) (bool, error) {
	key := fmt.Sprintf("presence:%s", userID)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check presence: %w", err)
	}

	return exists > 0, nil
}

// RefreshPresence keeps user online (heartbeat)
func (r *PresenceRepository) RefreshPresence(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("presence:%s", userID)

	// Refresh TTL
	err := r.client.Expire(ctx, key, 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to refresh presence: %w", err)
	}

	return nil
}

// GetOnlineUsers retrieves list of online user IDs
func (r *PresenceRepository) GetOnlineUsers(ctx context.Context) ([]uuid.UUID, error) {
	userIDStrs, err := r.client.SMembers(ctx, "presence:online").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get online users: %w", err)
	}

	userIDs := make([]uuid.UUID, 0, len(userIDStrs))
	for _, idStr := range userIDStrs {
		userID, err := uuid.Parse(idStr)
		if err != nil {
			continue // Skip invalid UUIDs
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

// GetOnlineCount returns number of online users
func (r *PresenceRepository) GetOnlineCount(ctx context.Context) (int64, error) {
	count, err := r.client.SCard(ctx, "presence:online").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count online users: %w", err)
	}
	return count, nil
}

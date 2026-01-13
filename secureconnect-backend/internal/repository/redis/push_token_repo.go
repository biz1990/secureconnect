package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/push"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PushTokenRepository handles push notification token storage in Redis
type PushTokenRepository struct {
	client *redis.Client
}

// NewPushTokenRepository creates a new push token repository
func NewPushTokenRepository(client *redis.Client) *PushTokenRepository {
	return &PushTokenRepository{
		client: client,
	}
}

// Store stores a push notification token
func (r *PushTokenRepository) Store(ctx context.Context, token *push.Token) error {
	// Generate ID if not provided
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now().Unix()
	if token.CreatedAt == 0 {
		token.CreatedAt = now
	}
	token.UpdatedAt = now

	// Serialize token
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Store token in Redis
	// Key format: push:token:{token}
	tokenKey := fmt.Sprintf("push:token:%s", token.Token)
	if err := r.client.Set(ctx, tokenKey, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	// Add to user's token set
	// Key format: push:user:{userID}:tokens
	userTokensKey := fmt.Sprintf("push:user:%s:tokens", token.UserID)
	if err := r.client.SAdd(ctx, userTokensKey, token.Token).Err(); err != nil {
		return fmt.Errorf("failed to add token to user set: %w", err)
	}

	// Set expiration on user tokens set (30 days)
	if err := r.client.Expire(ctx, userTokensKey, constants.PushTokenExpiry).Err(); err != nil {
		logger.Warn("Failed to set expiration on user tokens set",
			zap.String("user_id", token.UserID.String()),
			zap.Error(err))
	}

	logger.Debug("Push token stored",
		zap.String("token_id", token.ID.String()),
		zap.String("user_id", token.UserID.String()),
		zap.String("token_type", string(token.Type)))

	return nil
}

// GetByToken retrieves a token by its value
func (r *PushTokenRepository) GetByToken(ctx context.Context, tokenStr string) (*push.Token, error) {
	tokenKey := fmt.Sprintf("push:token:%s", tokenStr)
	data, err := r.client.Get(ctx, tokenKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Token not found
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	var token push.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// GetByUserID retrieves all tokens for a user
func (r *PushTokenRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*push.Token, error) {
	userTokensKey := fmt.Sprintf("push:user:%s:tokens", userID)
	tokens, err := r.client.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user tokens: %w", err)
	}

	var result []*push.Token
	for _, tokenStr := range tokens {
		token, err := r.GetByToken(ctx, tokenStr)
		if err != nil {
			logger.Warn("Failed to get token",
				zap.String("user_id", userID.String()),
				zap.String("token", tokenStr),
				zap.Error(err))
			continue
		}
		if token != nil {
			result = append(result, token)
		}
	}

	return result, nil
}

// Update updates an existing token
func (r *PushTokenRepository) Update(ctx context.Context, token *push.Token) error {
	token.UpdatedAt = time.Now().Unix()

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	tokenKey := fmt.Sprintf("push:token:%s", token.Token)
	if err := r.client.Set(ctx, tokenKey, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}

	logger.Debug("Push token updated",
		zap.String("token_id", token.ID.String()),
		zap.String("user_id", token.UserID.String()))

	return nil
}

// Delete removes a token
func (r *PushTokenRepository) Delete(ctx context.Context, tokenID uuid.UUID) error {
	// First, get the token to find its value and user ID
	// We need to scan for the token since we only have the ID
	// In production, you might want to store token ID -> token mapping
	// For now, we'll use a different approach

	// Get all token keys and find the one with matching ID
	iter := r.client.Scan(ctx, 0, "push:token:*", 0).Iterator()
	for iter.Next(ctx) {
		tokenKey := iter.Val()
		data, err := r.client.Get(ctx, tokenKey).Bytes()
		if err != nil {
			continue
		}

		var token push.Token
		if err := json.Unmarshal(data, &token); err != nil {
			continue
		}

		if token.ID == tokenID {
			// Remove from user's token set
			userTokensKey := fmt.Sprintf("push:user:%s:tokens", token.UserID)
			r.client.SRem(ctx, userTokensKey, token.Token)

			// Delete token
			if err := r.client.Del(ctx, tokenKey).Err(); err != nil {
				return fmt.Errorf("failed to delete token: %w", err)
			}

			logger.Debug("Push token deleted",
				zap.String("token_id", tokenID.String()),
				zap.String("user_id", token.UserID.String()))
			return nil
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan tokens: %w", err)
	}

	return nil // Token not found
}

// DeleteByUserID removes all tokens for a user
func (r *PushTokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	userTokensKey := fmt.Sprintf("push:user:%s:tokens", userID)
	tokens, err := r.client.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	// Delete all tokens
	for _, tokenStr := range tokens {
		tokenKey := fmt.Sprintf("push:token:%s", tokenStr)
		if err := r.client.Del(ctx, tokenKey).Err(); err != nil {
			logger.Warn("Failed to delete token",
				zap.String("user_id", userID.String()),
				zap.String("token", tokenStr),
				zap.Error(err))
		}
	}

	// Delete user tokens set
	if err := r.client.Del(ctx, userTokensKey).Err(); err != nil {
		return fmt.Errorf("failed to delete user tokens set: %w", err)
	}

	logger.Debug("All push tokens deleted for user",
		zap.String("user_id", userID.String()),
		zap.Int("count", len(tokens)))

	return nil
}

// MarkInactive marks a token as inactive
func (r *PushTokenRepository) MarkInactive(ctx context.Context, tokenID uuid.UUID) error {
	// Find the token
	iter := r.client.Scan(ctx, 0, "push:token:*", 0).Iterator()
	for iter.Next(ctx) {
		tokenKey := iter.Val()
		data, err := r.client.Get(ctx, tokenKey).Bytes()
		if err != nil {
			continue
		}

		var token push.Token
		if err := json.Unmarshal(data, &token); err != nil {
			continue
		}

		if token.ID == tokenID {
			token.Active = false
			token.UpdatedAt = time.Now().Unix()

			data, err := json.Marshal(token)
			if err != nil {
				return fmt.Errorf("failed to marshal token: %w", err)
			}

			if err := r.client.Set(ctx, tokenKey, data, 0).Err(); err != nil {
				return fmt.Errorf("failed to update token: %w", err)
			}

			logger.Debug("Push token marked as inactive",
				zap.String("token_id", tokenID.String()),
				zap.String("user_id", token.UserID.String()))
			return nil
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan tokens: %w", err)
	}

	return nil // Token not found
}

// CleanupInactiveTokens removes tokens that have been inactive for more than the specified duration
func (r *PushTokenRepository) CleanupInactiveTokens(ctx context.Context, inactiveDuration time.Duration) error {
	cutoff := time.Now().Add(-inactiveDuration).Unix()
	count := 0

	iter := r.client.Scan(ctx, 0, "push:token:*", 0).Iterator()
	for iter.Next(ctx) {
		tokenKey := iter.Val()
		data, err := r.client.Get(ctx, tokenKey).Bytes()
		if err != nil {
			continue
		}

		var token push.Token
		if err := json.Unmarshal(data, &token); err != nil {
			continue
		}

		// Delete inactive tokens older than cutoff
		if !token.Active && token.UpdatedAt < cutoff {
			// Remove from user's token set
			userTokensKey := fmt.Sprintf("push:user:%s:tokens", token.UserID)
			r.client.SRem(ctx, userTokensKey, token.Token)

			// Delete token
			if err := r.client.Del(ctx, tokenKey).Err(); err != nil {
				logger.Warn("Failed to delete inactive token",
					zap.String("token_id", token.ID.String()),
					zap.Error(err))
				continue
			}
			count++
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan tokens: %w", err)
	}

	logger.Info("Cleanup inactive push tokens completed",
		zap.Int("count", count),
		zap.Duration("inactive_duration", inactiveDuration))

	return nil
}

// GetActiveTokensCount returns the count of active tokens for a user
func (r *PushTokenRepository) GetActiveTokensCount(ctx context.Context, userID uuid.UUID) (int, error) {
	tokens, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, token := range tokens {
		if token.Active {
			count++
		}
	}

	return count, nil
}

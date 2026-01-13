package middleware

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	appJWT "secureconnect-backend/pkg/jwt"
)

// RedisRevocationChecker implements RevocationChecker using Redis
type RedisRevocationChecker struct {
	client *redis.Client
}

// NewRedisRevocationChecker creates a new RedisRevocationChecker
func NewRedisRevocationChecker(client *redis.Client) *RedisRevocationChecker {
	return &RedisRevocationChecker{client: client}
}

// IsTokenRevoked checks if a token is in the Redis blacklist
func (c *RedisRevocationChecker) IsTokenRevoked(ctx context.Context, tokenString string) (bool, error) {
	// Parse token without verification (signature validated by middleware already)
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &appJWT.Claims{})
	if err != nil {
		return false, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*appJWT.Claims)
	if !ok {
		return false, fmt.Errorf("invalid claims")
	}

	if claims.ID == "" {
		return false, nil
	}

	key := fmt.Sprintf("blacklist:%s", claims.ID)
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist in redis: %w", err)
	}

	return exists > 0, nil
}

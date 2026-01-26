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
		// Fail-open: If we can't parse the token, assume it's not revoked
		// This allows the request to proceed based on JWT validation alone
		return false, nil
	}

	claims, ok := token.Claims.(*appJWT.Claims)
	if !ok {
		// Fail-open: Invalid claims format, assume not revoked
		return false, nil
	}

	if claims.ID == "" {
		return false, nil
	}

	key := fmt.Sprintf("blacklist:%s", claims.ID)
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		// Fail-open: If Redis is unavailable, assume token is not revoked
		// This prevents service disruption during Redis outages
		return false, nil
	}

	return exists > 0, nil
}

package middleware

import (
	"context"
	"fmt"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	
	"secureconnect-backend/pkg/response"
)

// RateLimiter implements token bucket rate limiting using Redis
type RateLimiter struct {
	client             *redis.Client
	requestsPerMinute  int
	windowSize         time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *redis.Client, requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		client:            client,
		requestsPerMinute: requestsPerMinute,
		windowSize:        time.Minute,
	}
}

// Middleware returns Gin middleware for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client identifier (IP or user_id if authenticated)
		identifier := rl.getIdentifier(c)
		
		// Check rate limit
		allowed, err := rl.allow(c.Request.Context(), identifier)
		if err != nil {
			// On error, allow request but log
			c.Next()
			return
		}
		
		if !allowed {
			response.Error(c, 429, "RATE_LIMIT_EXCEEDED", "Too many requests. Please try again later.")
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// allow checks if request is allowed under rate limit
func (rl *RateLimiter) allow(ctx context.Context, identifier string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", identifier)
	
	// Use Redis INCR with expiration
	pipe := rl.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rl.windowSize)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	
	count, err := incr.Result()
	if err != nil {
		return false, err
	}
	
	return count <= int64(rl.requestsPerMinute), nil
}

// getIdentifier returns client identifier for rate limiting
func (rl *RateLimiter) getIdentifier(c *gin.Context) string {
	// Try to get user_id from context (if authenticated)
	if userID, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("user:%v", userID)
	}
	
	// Fall back to IP address
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// ResetLimit resets rate limit for an identifier (admin function)
func (rl *RateLimiter) ResetLimit(ctx context.Context, identifier string) error {
	key := fmt.Sprintf("ratelimit:%s", identifier)
	return rl.client.Del(ctx, key).Err()
}

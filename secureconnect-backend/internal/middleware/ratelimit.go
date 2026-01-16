package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements Redis-based rate limiting
type RateLimiter struct {
	redisClient *redis.Client
	requests    int
	window      time.Duration
}

// NewRateLimiter creates a new rate limiter
// requests: maximum number of requests allowed
// window: time window for the rate limit (e.g., 1 minute)
func NewRateLimiter(redisClient *redis.Client, requests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		requests:    requests,
		window:      window,
	}
}

// Middleware returns a Gin middleware for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()
		if clientIP == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to determine client IP"})
			c.Abort()
			return
		}

		// Get user ID if authenticated (for per-user rate limiting)
		userID, exists := c.Get("user_id")
		var identifier string
		if exists {
			identifier = fmt.Sprintf("user:%v", userID)
		} else {
			identifier = fmt.Sprintf("ip:%s", clientIP)
		}

		// Check rate limit
		allowed, remaining, resetTime, err := rl.checkRateLimit(c.Request.Context(), identifier)
		if err != nil {
			// Fail-open: Allow request if Redis is unavailable to prevent service disruption
			// Log the error but continue processing
			c.Next()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "Rate limit exceeded",
				"limit":     rl.requests,
				"remaining": remaining,
				"reset_at":  resetTime,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkRateLimit checks if the request is within rate limits
func (rl *RateLimiter) checkRateLimit(ctx context.Context, identifier string) (bool, int, int64, error) {
	// Redis key for rate limiting
	key := fmt.Sprintf("ratelimit:%s", identifier)

	// Use Redis INCR to count requests
	now := time.Now().Unix()
	windowStart := now - int64(rl.window.Seconds())

	// Get current count
	countCmd := rl.redisClient.Get(ctx, key)
	count, err := countCmd.Int()
	if err != nil && err != redis.Nil {
		return false, 0, 0, fmt.Errorf("failed to get rate limit count: %w", err)
	}

	// If no count exists, start fresh
	if err == redis.Nil {
		count = 0
	}

	// Check if we're in the same window
	lastResetCmd := rl.redisClient.Get(ctx, key+":reset")
	lastReset, err := lastResetCmd.Int64()
	if err != nil && err != redis.Nil {
		return false, 0, 0, fmt.Errorf("failed to get last reset time: %w", err)
	}

	if err == redis.Nil || lastReset < windowStart {
		// New window, reset count
		pipe := rl.redisClient.Pipeline()
		pipe.Set(ctx, key, 1, rl.window)
		pipe.Set(ctx, key+":reset", now, rl.window)
		_, err := pipe.Exec(ctx)
		if err != nil {
			return false, 0, 0, fmt.Errorf("failed to reset rate limit: %w", err)
		}
		count = 1
		lastReset = now
	} else {
		// Increment count
		pipe := rl.redisClient.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, rl.window)
		_, err := pipe.Exec(ctx)
		if err != nil {
			return false, 0, 0, fmt.Errorf("failed to increment rate limit: %w", err)
		}
		count++
	}

	remaining := rl.requests - count
	if remaining < 0 {
		remaining = 0
	}

	allowed := count <= rl.requests
	resetTime := lastReset + int64(rl.window.Seconds())

	return allowed, remaining, resetTime, nil
}

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"secureconnect-backend/internal/database"
	"secureconnect-backend/pkg/logger"
)

// RateLimiterConfig holds configuration for rate limiting with degraded mode support
type RateLimiterConfig struct {
	RedisClient            interface{} // Allow both *redis.Client and *database.RedisClient
	RequestsPerMin         int
	Window                 time.Duration
	EnableInMemoryFallback bool
}

// InMemoryRateLimiter provides in-memory rate limiting as fallback when Redis is degraded
type InMemoryRateLimiter struct {
	mu     sync.RWMutex
	limits map[string]*userRateLimit
}

type userRateLimit struct {
	count       int
	windowStart int64
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		limits: make(map[string]*userRateLimit),
	}
}

// Check checks if a request is within rate limits using in-memory tracking
func (im *InMemoryRateLimiter) Check(identifier string, requests int, window time.Duration) (bool, int, int64, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	limiter, exists := im.limits[identifier]
	if !exists {
		// First request for this identifier
		im.limits[identifier] = &userRateLimit{
			count:       1,
			windowStart: windowStart,
		}
		return true, requests - 1, 0, nil
	}

	// Check if window has expired
	if limiter.windowStart < windowStart {
		// Reset for new window
		limiter.count = 1
		limiter.windowStart = windowStart
	} else {
		// Increment count within window
		limiter.count++
	}

	remaining := requests - limiter.count
	if remaining < 0 {
		remaining = 0
	}

	allowed := limiter.count <= requests
	return allowed, remaining, limiter.windowStart + int64(window.Seconds()), nil
}

// RateLimiterWithFallback wraps the original Redis-based rate limiter with in-memory fallback
type RateLimiterWithFallback struct {
	redisLimiter    *RateLimiter
	inMemoryLimiter *InMemoryRateLimiter
	config          RateLimiterConfig
}

// NewRateLimiterWithFallback creates a new rate limiter with degraded mode support
func NewRateLimiterWithFallback(config RateLimiterConfig) *RateLimiterWithFallback {
	var redisClient *redis.Client
	if rc, ok := config.RedisClient.(*database.RedisClient); ok {
		redisClient = rc.Client
	} else if rc, ok := config.RedisClient.(*redis.Client); ok {
		redisClient = rc
	}

	return &RateLimiterWithFallback{
		redisLimiter:    NewRateLimiter(redisClient, config.RequestsPerMin, config.Window),
		inMemoryLimiter: NewInMemoryRateLimiter(),
		config:          config,
	}
}

// Middleware returns a Gin middleware for rate limiting with degraded mode support
func (rl *RateLimiterWithFallback) Middleware() gin.HandlerFunc {
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

		// Check if Redis is in degraded mode
		isRedisDegraded := false
		if redisClient, ok := rl.config.RedisClient.(*database.RedisClient); ok {
			// Redis degraded mode check
			isRedisDegraded = redisClient.IsDegraded()
		}

		var allowed bool
		var remaining int
		var resetTime int64
		var err error

		if isRedisDegraded && rl.config.EnableInMemoryFallback {
			// DEGRADED MODE: Use in-memory rate limiting
			logger.Warn("Using in-memory rate limiting (Redis degraded)",
				zap.String("service", "api-gateway"),
				zap.String("identifier", identifier))

			allowed, remaining, resetTime, err = rl.inMemoryLimiter.Check(
				identifier,
				rl.config.RequestsPerMin,
				rl.config.Window,
			)

			if err != nil {
				logger.Error("In-memory rate limiting check failed",
					zap.Error(err),
					zap.String("identifier", identifier))
			}
		} else {
			// NORMAL MODE: Use Redis-based rate limiting
			allowed, remaining, resetTime, err = rl.redisLimiter.checkRateLimit(
				c.Request.Context(),
				identifier,
			)

			if err != nil {
				// Log error but don't fail-open if Redis is degraded
				if isRedisDegraded {
					logger.Warn("Redis rate limit check failed (Redis degraded), allowing request",
						zap.Error(err),
						zap.String("identifier", identifier))
					// Fail-open: Allow request to prevent service disruption
					allowed = true
					remaining = rl.config.RequestsPerMin
					resetTime = time.Now().Unix() + int64(rl.config.Window.Seconds())
					err = nil
				} else {
					// Redis is healthy but operation failed - this is a real error
					logger.Error("Redis rate limit check failed",
						zap.Error(err),
						zap.String("identifier", identifier))
				}
			}
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerMin))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "Rate limit exceeded",
				"limit":     rl.config.RequestsPerMin,
				"remaining": remaining,
				"reset_at":  resetTime,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkRateLimit checks if a request is within rate limits using Redis
// This is a wrapper around the original RateLimiter's checkRateLimit method
func (rl *RateLimiterWithFallback) checkRateLimit(ctx context.Context, identifier string) (bool, int, int64, error) {
	if redisClient, ok := rl.config.RedisClient.(*database.RedisClient); ok {
		// Use the new SafeGet method from RedisClient
		countCmd := redisClient.SafeGet(ctx, fmt.Sprintf("ratelimit:%s", identifier))
		count, err := countCmd.Int()
		if err != nil {
			return false, 0, 0, fmt.Errorf("failed to get rate limit count: %w", err)
		}

		now := time.Now().Unix()
		windowStart := now - int64(rl.config.Window.Seconds())

		// Get last reset time
		lastResetCmd := redisClient.SafeGet(ctx, fmt.Sprintf("ratelimit:%s:reset", identifier))
		lastReset, err := lastResetCmd.Int64()
		if err != nil && err != redis.Nil {
			return false, 0, 0, fmt.Errorf("failed to get last reset time: %w", err)
		}

		// If no last reset or window has expired, reset count
		if err == redis.Nil || lastReset < windowStart {
			// New window, reset count
			pipe := redisClient.Client.Pipeline()
			pipe.Set(ctx, fmt.Sprintf("ratelimit:%s", identifier), 1, rl.config.Window)
			pipe.Set(ctx, fmt.Sprintf("ratelimit:%s:reset", identifier), now, rl.config.Window)
			_, err := pipe.Exec(ctx)
			if err != nil {
				return false, 0, 0, fmt.Errorf("failed to reset rate limit: %w", err)
			}
			count = 1
			lastReset = now
		} else {
			// Increment count within window
			pipe := redisClient.Client.Pipeline()
			pipe.Incr(ctx, fmt.Sprintf("ratelimit:%s", identifier))
			pipe.Expire(ctx, fmt.Sprintf("ratelimit:%s", identifier), rl.config.Window)
			_, err := pipe.Exec(ctx)
			if err != nil {
				return false, 0, 0, fmt.Errorf("failed to increment rate limit: %w", err)
			}
			count++
		}

		remaining := rl.config.RequestsPerMin - count
		if remaining < 0 {
			remaining = 0
		}

		allowed := count <= rl.config.RequestsPerMin
		return allowed, remaining, lastReset + int64(rl.config.Window.Seconds()), nil
	}
	return false, 0, 0, fmt.Errorf("redis client type assertion failed")
}

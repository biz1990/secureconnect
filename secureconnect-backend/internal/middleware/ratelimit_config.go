package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"secureconnect-backend/pkg/env"
)

// RateLimitConfig holds rate limit configuration for different endpoints
type RateLimitConfig struct {
	Endpoint string
	Requests int
	Window   time.Duration
}

// RateLimitConfigManager manages rate limit configurations
type RateLimitConfigManager struct {
	configs map[string]RateLimitConfig
}

// NewRateLimitConfigManager creates a new rate limit configuration manager
// Rate limits can be overridden via environment variables:
// - RATELIMIT_AUTH_REGISTER: Requests per minute for /v1/auth/register (default: 5)
// - RATELIMIT_AUTH_LOGIN: Requests per minute for /v1/auth/login (default: 10)
// - RATELIMIT_AUTH_REFRESH: Requests per minute for /v1/auth/refresh (default: 10)
// - RATELIMIT_AUTH_PASSWORD_RESET_REQUEST: Requests per minute for /v1/auth/password-reset/request (default: 3)
// - RATELIMIT_AUTH_PASSWORD_RESET_CONFIRM: Requests per minute for /v1/auth/password-reset/confirm (default: 5)
func NewRateLimitConfigManager() *RateLimitConfigManager {
	return &RateLimitConfigManager{
		configs: map[string]RateLimitConfig{
			// Authentication endpoints - stricter limits
			"/v1/auth/register": {
				Requests: env.GetInt("RATELIMIT_AUTH_REGISTER", 5),
				Window:   time.Minute,
			},
			"/v1/auth/login": {
				Requests: env.GetInt("RATELIMIT_AUTH_LOGIN", 10),
				Window:   time.Minute,
			},
			"/v1/auth/refresh": {
				Requests: env.GetInt("RATELIMIT_AUTH_REFRESH", 10),
				Window:   time.Minute,
			},
			"/v1/auth/password-reset/request": {
				Requests: env.GetInt("RATELIMIT_AUTH_PASSWORD_RESET_REQUEST", 3), // Prevent email spam
				Window:   time.Minute,
			},
			"/v1/auth/password-reset/confirm": {
				Requests: env.GetInt("RATELIMIT_AUTH_PASSWORD_RESET_CONFIRM", 5), // Prevent token abuse
				Window:   time.Minute,
			},

			// User management endpoints
			"/v1/users/me": {
				Requests: env.GetInt("RATELIMIT_USERS_ME", 50),
				Window:   time.Minute,
			},
			"/v1/users/me/password": {
				Requests: env.GetInt("RATELIMIT_USERS_ME_PASSWORD", 5),
				Window:   time.Minute,
			},
			"/v1/users/me/email": {
				Requests: env.GetInt("RATELIMIT_USERS_ME_EMAIL", 5),
				Window:   time.Minute,
			},
			"/v1/users/me/friends": {
				Requests: env.GetInt("RATELIMIT_USERS_ME_FRIENDS", 30),
				Window:   time.Minute,
			},
			"/v1/users/:id/block": {
				Requests: env.GetInt("RATELIMIT_USERS_ID_BLOCK", 20),
				Window:   time.Minute,
			},

			// Key management endpoints
			"/v1/keys/upload": {
				Requests: env.GetInt("RATELIMIT_KEYS_UPLOAD", 20),
				Window:   time.Minute,
			},
			"/v1/keys/rotate": {
				Requests: env.GetInt("RATELIMIT_KEYS_ROTATE", 10),
				Window:   time.Minute,
			},

			// Message endpoints
			"/v1/messages": {
				Requests: env.GetInt("RATELIMIT_MESSAGES", 100),
				Window:   time.Minute,
			},
			"/v1/messages/search": {
				Requests: env.GetInt("RATELIMIT_MESSAGES_SEARCH", 50),
				Window:   time.Minute,
			},

			// Conversation endpoints
			"/v1/conversations": {
				Requests: env.GetInt("RATELIMIT_CONVERSATIONS", 50),
				Window:   time.Minute,
			},
			"/v1/conversations/:id": {
				Requests: env.GetInt("RATELIMIT_CONVERSATIONS_ID", 100),
				Window:   time.Minute,
			},
			"/v1/conversations/:id/participants": {
				Requests: env.GetInt("RATELIMIT_CONVERSATIONS_ID_PARTICIPANTS", 30),
				Window:   time.Minute,
			},

			// Call endpoints
			"/v1/calls/initiate": {
				Requests: env.GetInt("RATELIMIT_CALLS_INITIATE", 10),
				Window:   time.Minute,
			},
			"/v1/calls/:id": {
				Requests: env.GetInt("RATELIMIT_CALLS_ID", 30),
				Window:   time.Minute,
			},
			"/v1/calls/:id/join": {
				Requests: env.GetInt("RATELIMIT_CALLS_ID_JOIN", 10),
				Window:   time.Minute,
			},

			// Storage endpoints
			"/v1/storage/upload-url": {
				Requests: env.GetInt("RATELIMIT_STORAGE_UPLOAD_URL", 20),
				Window:   time.Minute,
			},
			"/v1/storage/download-url": {
				Requests: env.GetInt("RATELIMIT_STORAGE_DOWNLOAD_URL", 30),
				Window:   time.Minute,
			},
			"/v1/storage/files": {
				Requests: env.GetInt("RATELIMIT_STORAGE_FILES", 20),
				Window:   time.Minute,
			},

			// Notification endpoints
			"/v1/notifications": {
				Requests: env.GetInt("RATELIMIT_NOTIFICATIONS", 50),
				Window:   time.Minute,
			},
			"/v1/notifications/read-all": {
				Requests: env.GetInt("RATELIMIT_NOTIFICATIONS_READ_ALL", 20),
				Window:   time.Minute,
			},

			// Admin endpoints - very strict limits
			"/v1/admin/stats": {
				Requests: env.GetInt("RATELIMIT_ADMIN_STATS", 30),
				Window:   time.Minute,
			},
			"/v1/admin/users": {
				Requests: env.GetInt("RATELIMIT_ADMIN_USERS", 20),
				Window:   time.Minute,
			},
			"/v1/admin/audit-logs": {
				Requests: env.GetInt("RATELIMIT_ADMIN_AUDIT_LOGS", 50),
				Window:   time.Minute,
			},
		},
	}
}

// GetConfig returns rate limit configuration for a specific endpoint
func (m *RateLimitConfigManager) GetConfig(endpoint string) RateLimitConfig {
	if config, exists := m.configs[endpoint]; exists {
		return config
	}
	// Default rate limit
	return RateLimitConfig{
		Requests: 100,
		Window:   time.Minute,
	}
}

// GetConfigForPath returns rate limit configuration based on path pattern matching
func (m *RateLimitConfigManager) GetConfigForPath(path string) RateLimitConfig {
	// Try exact match first
	if config, exists := m.configs[path]; exists {
		return config
	}

	// Try prefix match for parameterized paths
	for pattern, config := range m.configs {
		if isPathMatch(path, pattern) {
			return config
		}
	}

	// Default rate limit (configurable via RATELIMIT_DEFAULT)
	return RateLimitConfig{
		Requests: env.GetInt("RATELIMIT_DEFAULT", 100),
		Window:   time.Minute,
	}
}

// isPathMatch checks if a path matches a pattern (e.g., /v1/users/:id matches /v1/users/123)
func isPathMatch(path, pattern string) bool {
	// Simple pattern matching - in production, you might want to use a more sophisticated approach
	// For now, just check if path starts with pattern's base path
	pathParts := splitPath(path)
	patternParts := splitPath(pattern)

	if len(patternParts) == 0 {
		return false
	}

	// Check if all non-parameter parts of pattern match
	for i, part := range patternParts {
		if len(part) > 0 && part[0] != ':' {
			if i >= len(pathParts) || pathParts[i] != part {
				return false
			}
		}
	}

	return true
}

// splitPath splits a path into parts
func splitPath(path string) []string {
	parts := []string{}
	current := ""
	for _, ch := range path {
		if ch == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// AdvancedRateLimiter is an enhanced rate limiter with per-endpoint configuration
type AdvancedRateLimiter struct {
	redisClient *redis.Client
	configMgr   *RateLimitConfigManager
}

// NewAdvancedRateLimiter creates a new advanced rate limiter
func NewAdvancedRateLimiter(redisClient *redis.Client) *AdvancedRateLimiter {
	return &AdvancedRateLimiter{
		redisClient: redisClient,
		configMgr:   NewRateLimitConfigManager(),
	}
}

// Middleware returns a Gin middleware for advanced rate limiting
func (rl *AdvancedRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()
		if clientIP == "" {
			c.JSON(500, gin.H{"error": "Unable to determine client IP"})
			c.Abort()
			return
		}

		// Get user ID if authenticated (for per-user rate limiting)
		userID, exists := c.Get("user_id")
		var identifier string
		if exists {
			identifier = "user:" + userID.(string)
		} else {
			identifier = "ip:" + clientIP
		}

		// Get rate limit config for this endpoint
		path := c.Request.URL.Path
		config := rl.configMgr.GetConfigForPath(path)

		// Check rate limit
		allowed, remaining, resetTime, err := rl.checkRateLimit(c, identifier, config.Requests, config.Window)
		if err != nil {
			c.JSON(500, gin.H{"error": "Rate limit check failed"})
			c.Abort()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
		c.Header("X-RateLimit-Window", config.Window.String())

		if !allowed {
			c.JSON(429, gin.H{
				"error":       "Rate limit exceeded",
				"limit":       config.Requests,
				"remaining":   remaining,
				"reset_at":    resetTime,
				"retry_after": config.Window.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkRateLimit checks if request is within rate limits using Redis sliding window
func (rl *AdvancedRateLimiter) checkRateLimit(c *gin.Context, identifier string, requests int, window time.Duration) (bool, int, int64, error) {
	ctx := c.Request.Context()
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	// Redis key for rate limiting
	key := fmt.Sprintf("ratelimit:%s", identifier)
	windowKey := fmt.Sprintf("ratelimit:%s:window", identifier)

	// Use Redis pipeline for atomic operations
	pipe := rl.redisClient.Pipeline()

	// Get current window start
	pipe.Get(ctx, windowKey)

	// Increment request count
	pipe.Incr(ctx, key)

	// Set expiration on key
	pipe.Expire(ctx, key, window)

	// Execute pipeline
	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to execute Redis pipeline: %w", err)
	}

	// Parse results
	lastWindowStartBytes := results[0].(*redis.StringCmd).Val()
	count, err := results[1].(*redis.IntCmd).Result()
	if err != nil && err != redis.Nil {
		return false, 0, 0, fmt.Errorf("failed to get request count: %w", err)
	}

	// Check if we need to reset window
	lastWindowStart, _ := strconv.ParseInt(lastWindowStartBytes, 10, 64)
	if lastWindowStart < windowStart || err != nil {
		// New window, reset count
		if err := rl.redisClient.Set(ctx, windowKey, windowStart, window).Err(); err != nil {
			return false, 0, 0, fmt.Errorf("failed to set window start: %w", err)
		}
		if err := rl.redisClient.Set(ctx, key, 1, window).Err(); err != nil {
			return false, 0, 0, fmt.Errorf("failed to reset request count: %w", err)
		}
		count = int64(1)
	}

	remaining := requests - int(count)
	if remaining < 0 {
		remaining = 0
	}

	allowed := int(count) <= requests
	resetTime := lastWindowStart + int64(window.Seconds())

	return allowed, remaining, resetTime, nil
}

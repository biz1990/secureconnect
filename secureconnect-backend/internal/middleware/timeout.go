package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	DefaultTimeout time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		DefaultTimeout: 30 * time.Second,
	}
}

// TimeoutMiddleware implements global request timeout protection
type TimeoutMiddleware struct {
	config *TimeoutConfig
}

// NewTimeoutMiddleware creates a new timeout middleware
func NewTimeoutMiddleware(config *TimeoutConfig) *TimeoutMiddleware {
	if config == nil {
		config = DefaultTimeoutConfig()
	}
	return &TimeoutMiddleware{config: config}
}

// SetConfig updates timeout configuration
func (tm *TimeoutMiddleware) SetConfig(config *TimeoutConfig) {
	tm.config = config
}

// Middleware returns a Gin middleware for timeout protection
func (tm *TimeoutMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for per-route timeout override
		timeout := tm.config.DefaultTimeout
		if timeoutOverride, exists := c.Get("timeout_override"); exists {
			if duration, ok := timeoutOverride.(time.Duration); ok {
				timeout = duration
				logger.Debug("Using per-route timeout override",
					zap.Duration("timeout", timeout),
					zap.String("path", c.Request.URL.Path))
			}
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)

		// Store cancel function in context for manual cancellation
		c.Set("cancel_func", cancel)

		// Store timeout for metrics
		c.Set("request_timeout", timeout)

		// Replace request context with timeout context
		c.Request = c.Request.WithContext(ctx)

		// Track request start
		startTime := time.Now()

		// Process request
		c.Next()

		// Check if context was cancelled (timeout)
		select {
		case <-ctx.Done():
			// Request timed out
			duration := time.Since(startTime)
			metrics.RecordRequestTimeout(timeout, duration, c.Request.Method, c.Request.URL.Path)

			logger.Warn("Request timed out",
				zap.Duration("timeout", timeout),
				zap.Duration("duration", duration),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)

			// Return 504 Gateway Timeout
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"error": "Request timeout",
				"code":  "REQUEST_TIMEOUT",
			})
			c.Abort()
			return
		default:
			// Request completed successfully
			duration := time.Since(startTime)
			metrics.RecordRequestDuration(duration, c.Request.Method, c.Request.URL.Path, strconv.Itoa(c.Writer.Status()))
		}
	}
}

// WithTimeout creates a context with timeout for use in handlers
// This allows handlers to create their own timeout contexts
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// CancelRequest cancels the current request
func CancelRequest(c *gin.Context) {
	if cancelFunc, exists := c.Get("cancel_func"); exists {
		if fn, ok := cancelFunc.(context.CancelFunc); ok {
			fn()
			logger.Debug("Request cancelled manually",
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method))
		}
	}
}

// SetTimeoutOverride sets a per-route timeout override
// This allows specific routes to have different timeouts
func SetTimeoutOverride(c *gin.Context, timeout time.Duration) {
	c.Set("timeout_override", timeout)
}

// GetTimeout returns the timeout for the current request
func GetTimeout(c *gin.Context) time.Duration {
	if timeoutOverride, exists := c.Get("timeout_override"); exists {
		if duration, ok := timeoutOverride.(time.Duration); ok {
			return duration
		}
	}
	if timeout, exists := c.Get("request_timeout"); exists {
		if duration, ok := timeout.(time.Duration); ok {
			return duration
		}
	}
	// Return default (this shouldn't happen if middleware is used)
	return 30 * time.Second
}

// GetTimeoutRemaining returns the remaining time before timeout
func GetTimeoutRemaining(c *gin.Context) time.Duration {
	timeout := GetTimeout(c)
	deadline, ok := c.Request.Context().Deadline()
	if !ok {
		return timeout
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsTimedOut checks if the request is close to timing out
func IsTimedOut(c *gin.Context) bool {
	remaining := GetTimeoutRemaining(c)
	return remaining < 100*time.Millisecond // Less than 100ms remaining
}

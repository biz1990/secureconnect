package middleware

import (
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestLogger logs HTTP requests with request ID
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID
		requestID := uuid.New().String()
		c.Set("request_id", requestID)
		
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		
		// Process request
		c.Next()
		
		// Log request details
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		
		// Build log message
		if query != "" {
			path = path + "?" + query
		}
		
		// Simple log output (in production, use structured logging)
		if statusCode >= 500 {
			// Error
			c.Writer.Header().Set("X-Request-ID", requestID)
			// log.Printf("[ERROR] %s | %3d | %13v | %15s | %-7s %s",
			// 	requestID, statusCode, latency, clientIP, method, path)
		} else if statusCode >= 400 {
			// Client error
			c.Writer.Header().Set("X-Request-ID", requestID)
			// log.Printf("[WARN] %s | %3d | %13v | %15s | %-7s %s",
			// 	requestID, statusCode, latency, clientIP, method, path)
		} else {
			// Success
			c.Writer.Header().Set("X-Request-ID", requestID)
			// log.Printf("[INFO] %s | %3d | %13v | %15s | %-7s %s",
			// 	requestID, statusCode, latency, clientIP, method, path)
		}
	}
}

package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security headers to all responses
// Implements recommendations from security audit
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// XSS Protection
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// HSTS (HTTP Strict Transport Security)
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Referrer Policy
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")

		// Permissions Policy
		c.Writer.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

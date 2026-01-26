package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	// Get allowed origins from environment or use defaults
	allowedOrigins := map[string]bool{
		"http://localhost:3000": true,
		"http://localhost:8080": true,
		"http://127.0.0.1:3000": true,
		"http://127.0.0.1:8080": true,
	}

	// Add production origins from environment if set
	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		// Parse comma-separated origins
		for _, origin := range strings.Split(origins, ",") {
			allowedOrigins[strings.TrimSpace(origin)] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Only set CORS headers for allowed origins
		if allowedOrigins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin != "" {
			// Reject requests from disallowed origins
			c.AbortWithStatus(403)
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

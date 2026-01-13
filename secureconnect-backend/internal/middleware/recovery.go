package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"secureconnect-backend/pkg/response"
)

// Recovery recovers from panics and returns 500 error
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log panic (in production, use proper logging)
				// log.Printf("[PANIC] %v", err)

				// Return 500 error
				response.InternalError(c, "Internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// HealthCheck middleware ensures service health
func HealthCheck(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"service": serviceName,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

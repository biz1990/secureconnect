package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"secureconnect-backend/pkg/jwt"
)

// RevocationChecker defines interface for checking if a token is revoked (blacklisted)
type RevocationChecker interface {
	// IsTokenRevoked checks if a JWT token has been revoked/blacklisted
	IsTokenRevoked(ctx context.Context, tokenString string) (bool, error)
}

// AuthMiddleware creates a Gin middleware that validates JWT tokens
// It checks for the Authorization header, validates the token, and checks revocation status
// If valid, it sets user_id, username, and role in the Gin context
// Parameters:
//   - jwtManager: JWT manager for token validation
//   - revocationChecker: Optional checker for token revocation (can be nil)
func AuthMiddleware(jwtManager *jwt.JWTManager, revocationChecker RevocationChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Check revocation
		if revocationChecker != nil {
			revoked, err := revocationChecker.IsTokenRevoked(c.Request.Context(), tokenString)
			if err != nil {
				// Fail open or closed? Closed (secure) implies error => unauthorized
				// But Redis failure shouldn't necessarily block all traffic?
				// For high security: block.
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify token status"})
				c.Abort()
				return
			}
			if revoked {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Token revoked"})
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

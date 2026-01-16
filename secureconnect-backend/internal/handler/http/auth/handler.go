package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/service/auth"
	"secureconnect-backend/pkg/response"
)

// Handler handles HTTP requests for authentication
type Handler struct {
	authService *auth.Service
}

// NewHandler creates a new auth handler
func NewHandler(authService *auth.Service) *Handler {
	return &Handler{
		authService: authService,
	}
}

// RegisterRequest represents registration request body
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Username    string `json:"username" binding:"required,min=3,max=30"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
}

// LoginRequest represents login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenRequest represents refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RequestPasswordResetRequest represents password reset request
type RequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents password reset confirmation
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// Register handles user registration
// POST /v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Call service
	output, err := h.authService.Register(c.Request.Context(), &auth.RegisterInput{
		Email:       req.Email,
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})

	if err != nil {
		// Check for specific errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "email already registered") || strings.Contains(errMsg, "username already taken") {
			response.Conflict(c, errMsg)
			return
		}
		if strings.Contains(errMsg, "validation failed") {
			response.ValidationError(c, errMsg)
			return
		}
		// Log the actual error for debugging
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to register user",
				"details": errMsg,
			},
			"meta": gin.H{
				"timestamp": time.Now().UTC(),
			},
		})
		return
	}

	// Return response
	response.Success(c, http.StatusCreated, gin.H{
		"user":          output.User,
		"access_token":  output.AccessToken,
		"refresh_token": output.RefreshToken,
	})
}

// Login handles user login
// POST /v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Call service
	output, err := h.authService.Login(c.Request.Context(), &auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		if err.Error() == "invalid credentials" {
			response.Unauthorized(c, "Invalid email or password")
			return
		}
		response.InternalError(c, "Failed to login")
		return
	}

	// Return response
	response.Success(c, http.StatusOK, gin.H{
		"user":          output.User,
		"access_token":  output.AccessToken,
		"refresh_token": output.RefreshToken,
	})
}

// RefreshToken handles token refresh
// POST /v1/auth/refresh
func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Call service
	output, err := h.authService.RefreshToken(c.Request.Context(), &auth.RefreshTokenInput{
		RefreshToken: req.RefreshToken,
	})

	if err != nil {
		response.Unauthorized(c, "Invalid or expired refresh token")
		return
	}

	// Return response
	response.Success(c, http.StatusOK, gin.H{
		"access_token":  output.AccessToken,
		"refresh_token": output.RefreshToken,
	})
}

// Logout handles user logout
// POST /v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	// Get user_id from context (set by auth middleware)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Get session ID from request (could be from header or body)
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		// Optionally get from body
		var req struct {
			SessionID string `json:"session_id"`
		}
		c.ShouldBindJSON(&req)
		sessionID = req.SessionID
	}

	// Extract token string for blacklisting
	authHeader := c.GetHeader("Authorization")
	parts := strings.Split(authHeader, " ")
	tokenString := ""
	if len(parts) == 2 && parts[0] == "Bearer" {
		tokenString = parts[1]
	}

	// Call service
	if err := h.authService.Logout(c.Request.Context(), sessionID, userID, tokenString); err != nil {
		response.InternalError(c, "Failed to logout")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// GetProfile returns current user profile
// GET /v1/auth/profile
func (h *Handler) GetProfile(c *gin.Context) {
	// Get user_id from context (set by auth middleware)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	// Return user info from token claims
	response.Success(c, http.StatusOK, gin.H{
		"user_id":  userIDVal,
		"email":    c.GetString("email"),
		"username": c.GetString("username"),
		"role":     c.GetString("role"),
	})
}

// RequestPasswordReset handles password reset request
// POST /v1/auth/password-reset/request
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req RequestPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Call service
	err := h.authService.RequestPasswordReset(c.Request.Context(), &auth.RequestPasswordResetInput{
		Email: req.Email,
	})

	if err != nil {
		response.InternalError(c, "Failed to process password reset request")
		return
	}

	// Always return success to prevent email enumeration
	response.Success(c, http.StatusOK, gin.H{
		"message": "If an account exists with this email, a password reset link has been sent",
	})
}

// ResetPassword handles password reset confirmation
// POST /v1/auth/password-reset/confirm
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Call service
	err := h.authService.ResetPassword(c.Request.Context(), &auth.ResetPasswordInput{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	})

	if err != nil {
		response.Unauthorized(c, "Invalid or expired reset token")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Password has been reset successfully",
	})
}

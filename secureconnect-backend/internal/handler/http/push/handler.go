package push

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/push"
)

// Handler handles push notification HTTP requests
type Handler struct {
	pushService *push.Service
}

// NewHandler creates a new push notification handler
func NewHandler(pushService *push.Service) *Handler {
	return &Handler{
		pushService: pushService,
	}
}

// RegisterTokenRequest represents request to register a push token
type RegisterTokenRequest struct {
	Token    string         `json:"token" binding:"required"`
	Type     push.TokenType `json:"type" binding:"required,oneof=fcm apns web"`
	DeviceID string         `json:"device_id"`
	Platform string         `json:"platform"` // ios, android, web
}

// RegisterToken registers a new push notification token for the authenticated user
// @Summary Register push notification token
// @Description Register a new push notification token for the authenticated user
// @Tags Push
// @Accept json
// @Produce json
// @Param request body RegisterTokenRequest true "Token registration data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /push/tokens [post]
func (h *Handler) RegisterToken(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse request
	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate platform
	if req.Platform != "" && req.Platform != "ios" && req.Platform != "android" && req.Platform != "web" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid platform. Must be 'ios', 'android', or 'web'"})
		return
	}

	// Create token
	token := &push.Token{
		UserID:    userID,
		Token:     req.Token,
		Type:      req.Type,
		DeviceID:  req.DeviceID,
		Platform:  req.Platform,
		Active:    true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Register token
	if err := h.pushService.RegisterToken(c.Request.Context(), token); err != nil {
		logger.Error("Failed to register push token",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register token"})
		return
	}

	logger.Info("Push token registered",
		zap.String("user_id", userID.String()),
		zap.String("token_type", string(req.Type)),
		zap.String("platform", req.Platform))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Token registered successfully",
		"token_id": token.ID,
	})
}

// UnregisterTokenRequest represents request to unregister a push token
type UnregisterTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// UnregisterToken removes a push notification token
// @Summary Unregister push notification token
// @Description Remove a push notification token for the authenticated user
// @Tags Push
// @Accept json
// @Produce json
// @Param request body UnregisterTokenRequest true "Token unregistration data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /push/tokens [delete]
func (h *Handler) UnregisterToken(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse request
	var req UnregisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get token by value
	token, err := h.pushService.GetTokenByValue(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get token"})
		return
	}

	// Verify token belongs to user
	if token == nil || token.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	// Unregister token
	if err := h.pushService.UnregisterToken(c.Request.Context(), token.ID); err != nil {
		logger.Error("Failed to unregister push token",
			zap.String("user_id", userID.String()),
			zap.String("token_id", token.ID.String()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister token"})
		return
	}

	logger.Info("Push token unregistered",
		zap.String("user_id", userID.String()),
		zap.String("token_id", token.ID.String()))

	c.JSON(http.StatusOK, gin.H{
		"message": "Token unregistered successfully",
	})
}

// UnregisterAllTokens removes all push notification tokens for the authenticated user
// @Summary Unregister all push notification tokens
// @Description Remove all push notification tokens for the authenticated user
// @Tags Push
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /push/tokens/all [delete]
func (h *Handler) UnregisterAllTokens(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Unregister all tokens
	if err := h.pushService.UnregisterAllTokens(c.Request.Context(), userID); err != nil {
		logger.Error("Failed to unregister all push tokens",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister tokens"})
		return
	}

	logger.Info("All push tokens unregistered",
		zap.String("user_id", userID.String()))

	c.JSON(http.StatusOK, gin.H{
		"message": "All tokens unregistered successfully",
	})
}

// GetTokens returns all push notification tokens for the authenticated user
// @Summary Get push notification tokens
// @Description Get all push notification tokens for the authenticated user
// @Tags Push
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /push/tokens [get]
func (h *Handler) GetTokens(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get tokens
	tokens, err := h.pushService.GetTokensByUserID(c.Request.Context(), userID)
	if err != nil {
		logger.Error("Failed to get push tokens",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tokens": tokens,
		"count":  len(tokens),
	})
}

// TestNotificationRequest represents request to send a test notification
type TestNotificationRequest struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"`
}

// TestNotification sends a test push notification to the authenticated user
// @Summary Send test push notification
// @Description Send a test push notification to the authenticated user's registered devices
// @Tags Push
// @Accept json
// @Produce json
// @Param request body TestNotificationRequest true "Test notification data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /push/test [post]
func (h *Handler) TestNotification(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse request
	var req TestNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create test notification
	notification := &push.Notification{
		Title:    req.Title,
		Body:     req.Body,
		Priority: "normal",
		Sound:    "default",
		Data: map[string]string{
			"type": "test",
		},
	}

	// Send notification
	if err := h.pushService.SendCustomNotification(c.Request.Context(), notification, []uuid.UUID{userID}); err != nil {
		logger.Error("Failed to send test notification",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send test notification"})
		return
	}

	logger.Info("Test notification sent",
		zap.String("user_id", userID.String()),
		zap.String("title", req.Title))

	c.JSON(http.StatusOK, gin.H{
		"message": "Test notification sent successfully",
	})
}

// GetTokenCount returns the count of active push notification tokens for the authenticated user
// @Summary Get active push notification token count
// @Description Get the count of active push notification tokens for the authenticated user
// @Tags Push
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /push/tokens/count [get]
func (h *Handler) GetTokenCount(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get token count
	count, err := h.pushService.GetActiveTokensCount(c.Request.Context(), userID)
	if err != nil {
		logger.Error("Failed to get push token count",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get token count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active_tokens_count": count,
	})
}

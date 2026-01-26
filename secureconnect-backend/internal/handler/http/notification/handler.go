package notification

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/service/notification"
	"secureconnect-backend/pkg/response"
)

// Handler handles notification HTTP requests
type Handler struct {
	notificationService *notification.Service
}

// NewHandler creates a new notification handler
func NewHandler(notificationService *notification.Service) *Handler {
	return &Handler{
		notificationService: notificationService,
	}
}

// GetNotifications retrieves user's notifications
// GET /v1/notifications
func (h *Handler) GetNotifications(c *gin.Context) {
	// Get user ID from context
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

	// Parse query parameters
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get notifications
	result, err := h.notificationService.GetNotifications(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.InternalError(c, "Failed to get notifications")
		return
	}

	response.Success(c, http.StatusOK, result)
}

// GetNotificationCount retrieves unread notification count
// GET /v1/notifications/count
func (h *Handler) GetNotificationCount(c *gin.Context) {
	// Get user ID from context
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

	// Get unread count
	count, err := h.notificationService.GetUnreadCount(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to get notification count")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"unread_count": count,
	})
}

// MarkAsRead marks a notification as read
// POST /v1/notifications/:id/read
func (h *Handler) MarkAsRead(c *gin.Context) {
	notificationIDStr := c.Param("id")

	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid notification ID")
		return
	}

	// Get user ID from context
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

	// Mark as read
	err = h.notificationService.MarkAsRead(c.Request.Context(), notificationID, userID)
	if err != nil {
		response.NotFound(c, "Notification not found")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Notification marked as read",
	})
}

// MarkAllAsRead marks all notifications as read
// POST /v1/notifications/read-all
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	// Get user ID from context
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

	// Mark all as read
	err := h.notificationService.MarkAllAsRead(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to mark all notifications as read")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "All notifications marked as read",
	})
}

// DeleteNotification deletes a notification
// DELETE /v1/notifications/:id
func (h *Handler) DeleteNotification(c *gin.Context) {
	notificationIDStr := c.Param("id")

	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid notification ID")
		return
	}

	// Get user ID from context
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

	// Delete notification
	err = h.notificationService.DeleteNotification(c.Request.Context(), notificationID, userID)
	if err != nil {
		response.NotFound(c, "Notification not found")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Notification deleted",
	})
}

// GetPreferences retrieves notification preferences
// GET /v1/notifications/preferences
func (h *Handler) GetPreferences(c *gin.Context) {
	// Get user ID from context
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

	// Get preferences
	pref, err := h.notificationService.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to get notification preferences")
		return
	}

	response.Success(c, http.StatusOK, pref)
}

// UpdatePreferences updates notification preferences
// PATCH /v1/notifications/preferences
func (h *Handler) UpdatePreferences(c *gin.Context) {
	var req domain.NotificationPreferenceUpdate

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get user ID from context
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

	// Update preferences
	pref, err := h.notificationService.UpdatePreferences(c.Request.Context(), userID, &req)
	if err != nil {
		response.InternalError(c, "Failed to update notification preferences")
		return
	}

	response.Success(c, http.StatusOK, pref)
}

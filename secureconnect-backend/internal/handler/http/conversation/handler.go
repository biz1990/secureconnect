package conversation

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	
	"secureconnect-backend/internal/service/conversation"
	"secureconnect-backend/pkg/response"
)

// Handler handles conversation HTTP requests
type Handler struct {
	conversationService *conversation.Service
}

// NewHandler creates a new conversation handler
func NewHandler(conversationService *conversation.Service) *Handler {
	return &Handler{
		conversationService: conversationService,
	}
}

// CreateConversationRequest represents create conversation request
type CreateConversationRequest struct {
	Title          string   `json:"title" binding:"required"`
	Type           string   `json:"type" binding:"required,oneof=direct group"`
	ParticipantIDs []string `json:"participant_ids" binding:"required,min=2"`
	IsE2EEEnabled  *bool    `json:"is_e2ee_enabled"` // Optional, defaults to true
}

// CreateConversation creates a new conversation
// POST /v1/conversations
func (h *Handler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}
	
	// Get creator from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}
	
	creatorID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}
	
	// Parse participant IDs
	participantUUIDs := make([]uuid.UUID, len(req.ParticipantIDs))
	for i, idStr := range req.ParticipantIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.ValidationError(c, "Invalid participant ID: "+idStr)
			return
		}
		participantUUIDs[i] = id
	}
	
	// Default E2EE enabled to true
	isE2EEEnabled := true
	if req.IsE2EEEnabled != nil {
		isE2EEEnabled = *req.IsE2EEEnabled
	}
	
	// Create conversation
	conv, err := h.conversationService.CreateConversation(c.Request.Context(), &conversation.CreateConversationInput{
		Title:         req.Title,
		Type:          req.Type,
		CreatedBy:     creatorID,
		Participants:  participantUUIDs,
		IsE2EEEnabled: isE2EEEnabled,
	})
	
	if err != nil {
		response.InternalError(c, "Failed to create conversation: "+err.Error())
		return
	}
	
	response.Success(c, http.StatusCreated, conv)
}

// GetConversations retrieves user's conversations
// GET /v1/conversations?limit=20&offset=0
func (h *Handler) GetConversations(c *gin.Context) {
	// Get user ID
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
	
	// Parse query params
	limit := 20
	offset := 0
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := uuid.Parse(limitStr); err == nil {
			_ = l // Just to avoid unused var
		}
	}
	
	// Get conversations
	conversations, err := h.conversationService.GetUserConversations(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.InternalError(c, "Failed to get conversations")
		return
	}
	
	response.Success(c, http.StatusOK, gin.H{
		"conversations": conversations,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetConversation retrieves a specific conversation
// GET /v1/conversations/:id
func (h *Handler) GetConversation(c *gin.Context) {
	conversationIDStr := c.Param("id")
	
	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}
	
	conversation, err := h.conversationService.GetConversation(c.Request.Context(), conversationID)
	if err != nil {
		response.NotFound(c, "Conversation not found")
		return
	}
	
	response.Success(c, http.StatusOK, conversation)
}

// UpdateSettings updates conversation settings
// PUT /v1/conversations/:id/settings
func (h *Handler) UpdateSettings(c *gin.Context) {
	conversationIDStr := c.Param("id")
	
	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}
	
	var req struct {
		IsE2EEEnabled bool `json:"is_e2ee_enabled"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}
	
	if err := h.conversationService.UpdateE2EESettings(c.Request.Context(), conversationID, req.IsE2EEEnabled); err != nil {
		response.InternalError(c, "Failed to update settings")
		return
	}
	
	response.Success(c, http.StatusOK, gin.H{
		"message": "Settings updated successfully",
	})
}

// AddParticipants adds users to a conversation
// POST /v1/conversations/:id/participants
func (h *Handler) AddParticipants(c *gin.Context) {
	conversationIDStr := c.Param("id")
	
	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}
	
	var req struct {
		UserIDs []string `json:"user_ids" binding:"required,min=1"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}
	
	// Parse UUIDs
	userUUIDs := make([]uuid.UUID, len(req.UserIDs))
	for i, idStr := range req.UserIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.ValidationError(c, "Invalid user ID: "+idStr)
			return
		}
		userUUIDs[i] = id
	}
	
	if err := h.conversationService.AddParticipants(c.Request.Context(), conversationID, userUUIDs); err != nil {
		response.InternalError(c, "Failed to add participants")
		return
	}
	
	response.Success(c, http.StatusOK, gin.H{
		"message": "Participants added successfully",
	})
}

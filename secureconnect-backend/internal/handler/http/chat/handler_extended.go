package chat

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/service/chat"
	"secureconnect-backend/pkg/response"
)

// ExtendedHandler extends the existing chat handler with additional endpoints
type ExtendedHandler struct {
	chatService *chat.ExtendedService
}

// NewExtendedHandler creates a new extended chat handler
func NewExtendedHandler(chatService *chat.ExtendedService) *ExtendedHandler {
	return &ExtendedHandler{
		chatService: chatService,
	}
}

// DeleteMessageRequest represents delete message request
type DeleteMessageRequest struct {
	ConversationID string `json:"conversation_id" binding:"required,uuid"`
}

// DeleteMessage deletes a message
// DELETE /v1/messages/:id
func (h *ExtendedHandler) DeleteMessage(c *gin.Context) {
	messageIDStr := c.Param("id")

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid message ID")
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

	// Delete message (only if user is the sender)
	err = h.chatService.DeleteMessage(c.Request.Context(), messageID, userID)
	if err != nil {
		response.InternalError(c, "Failed to delete message: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Message deleted successfully",
	})
}

// MarkMessagesAsReadRequest represents mark as read request
type MarkMessagesAsReadRequest struct {
	ConversationID string `json:"conversation_id" binding:"required,uuid"`
	LastMessageID  string `json:"last_message_id" binding:"required,uuid"`
}

// MarkMessagesAsRead marks messages as read up to a specific message
// POST /v1/messages/read
func (h *ExtendedHandler) MarkMessagesAsRead(c *gin.Context) {
	var req MarkMessagesAsReadRequest
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

	// Parse conversation ID and last message ID
	conversationID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}

	lastMessageID, err := uuid.Parse(req.LastMessageID)
	if err != nil {
		response.ValidationError(c, "Invalid last message ID")
		return
	}

	// Mark messages as read
	err = h.chatService.MarkMessagesAsRead(c.Request.Context(), userID, conversationID, lastMessageID)
	if err != nil {
		response.InternalError(c, "Failed to mark messages as read: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Messages marked as read",
	})
}

// SearchMessagesQuery represents search query parameters
type SearchMessagesQuery struct {
	ConversationID string `form:"conversation_id" binding:"required,uuid"`
	Query          string `form:"query" binding:"required,min=1"`
	Limit          int    `form:"limit"`
	PageState      string `form:"page_state"`
}

// SearchMessages searches for messages in a conversation
// GET /v1/messages/search
func (h *ExtendedHandler) SearchMessages(c *gin.Context) {
	var query SearchMessagesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
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

	// Parse conversation ID
	conversationID, err := uuid.Parse(query.ConversationID)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}

	// Set default limit
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	// Search messages
	output, err := h.chatService.SearchMessages(c.Request.Context(), &chat.SearchMessagesInput{
		ConversationID: conversationID,
		UserID:         userID,
		Query:          query.Query,
		Limit:          query.Limit,
		PageState:      []byte(query.PageState),
	})

	if err != nil {
		response.InternalError(c, "Failed to search messages")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"messages":        output.Messages,
		"next_page_state": output.NextPageState,
		"has_more":        output.HasMore,
	})
}

// ForwardMessageRequest represents forward message request
type ForwardMessageRequest struct {
	MessageID      string `json:"message_id" binding:"required,uuid"`
	ConversationID string `json:"conversation_id" binding:"required,uuid"`
}

// ForwardMessage forwards a message to another conversation
// POST /v1/messages/forward
func (h *ExtendedHandler) ForwardMessage(c *gin.Context) {
	var req ForwardMessageRequest
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

	// Parse IDs
	messageID, err := uuid.Parse(req.MessageID)
	if err != nil {
		response.ValidationError(c, "Invalid message ID")
		return
	}

	conversationID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}

	// Forward message
	output, err := h.chatService.ForwardMessage(c.Request.Context(), &chat.ForwardMessageInput{
		MessageID:      messageID,
		ConversationID: conversationID,
		UserID:         userID,
	})

	if err != nil {
		if err.Error() == "message not found" {
			response.NotFound(c, "Message not found")
			return
		}
		response.InternalError(c, "Failed to forward message")
		return
	}

	response.Success(c, http.StatusCreated, output.Message)
}

// GetMessage retrieves a single message by ID
// GET /v1/messages/:id
func (h *ExtendedHandler) GetMessage(c *gin.Context) {
	messageIDStr := c.Param("id")

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid message ID")
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

	// Get message
	message, err := h.chatService.GetMessage(c.Request.Context(), messageID, userID)
	if err != nil {
		if err.Error() == "message not found" {
			response.NotFound(c, "Message not found")
			return
		}
		response.InternalError(c, "Failed to get message")
		return
	}

	response.Success(c, http.StatusOK, message)
}

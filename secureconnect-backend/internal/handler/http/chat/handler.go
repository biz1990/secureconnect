package chat

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/service/chat"
	"secureconnect-backend/pkg/response"
)

// Handler handles chat HTTP requests
type Handler struct {
	chatService *chat.Service
}

// NewHandler creates a new chat handler
func NewHandler(chatService *chat.Service) *Handler {
	return &Handler{
		chatService: chatService,
	}
}

// SendMessageRequest represents send message request
type SendMessageRequest struct {
	ConversationID string                 `json:"conversation_id" binding:"required,uuid"`
	Content        string                 `json:"content" binding:"required"`
	IsEncrypted    bool                   `json:"is_encrypted"`
	MessageType    string                 `json:"message_type" binding:"required,oneof=text image video file"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// TypingIndicatorRequest represents typing indicator request
type TypingIndicatorRequest struct {
	ConversationID string `json:"conversation_id" binding:"required,uuid"`
	IsTyping       bool   `json:"is_typing" binding:"required"`
}

// GetMessagesQuery represents query parameters for listing messages
type GetMessagesQuery struct {
	ConversationID string `form:"conversation_id" binding:"required,uuid"`
	Limit          int    `form:"limit"`
	PageState      string `form:"page_state"` // Base64 encoded
}

// SendMessage handles sending a new message
// POST /v1/messages
func (h *Handler) SendMessage(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get sender ID from context (set by auth middleware)
	senderIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	senderID, ok := senderIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Parse conversation ID
	conversationID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}

	// Call service
	output, err := h.chatService.SendMessage(c.Request.Context(), &chat.SendMessageInput{
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        req.Content,
		IsEncrypted:    req.IsEncrypted,
		MessageType:    req.MessageType,
		Metadata:       req.Metadata,
	})

	if err != nil {
		response.InternalError(c, "Failed to send message")
		return
	}

	response.Success(c, http.StatusCreated, output.Message)
}

// GetMessages retrieves conversation messages
// GET /v1/messages?conversation_id=uuid&limit=20&page_state=base64
func (h *Handler) GetMessages(c *gin.Context) {
	var query GetMessagesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ValidationError(c, err.Error())
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
		query.Limit = 100 // Max limit
	}

	// Decode page state
	var pageState []byte
	if query.PageState != "" {
		pageState, err = base64.StdEncoding.DecodeString(query.PageState)
		if err != nil {
			response.ValidationError(c, "Invalid page state")
			return
		}
	}

	// Call service
	output, err := h.chatService.GetMessages(c.Request.Context(), &chat.GetMessagesInput{
		ConversationID: conversationID,
		Limit:          query.Limit,
		PageState:      pageState,
	})

	if err != nil {
		response.InternalError(c, "Failed to get messages")
		return
	}

	// Encode next page state
	var nextPageStateEncoded string
	if len(output.NextPageState) > 0 {
		nextPageStateEncoded = base64.StdEncoding.EncodeToString(output.NextPageState)
	}

	response.Success(c, http.StatusOK, gin.H{
		"messages":        output.Messages,
		"next_page_state": nextPageStateEncoded,
		"has_more":        output.HasMore,
	})
}

// UpdatePresence handles presence updates
// POST /v1/presence
func (h *Handler) UpdatePresence(c *gin.Context) {
	var req struct {
		Online bool `json:"online"`
	}

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

	// Call service
	if err := h.chatService.UpdatePresence(c.Request.Context(), userID, req.Online); err != nil {
		response.InternalError(c, "Failed to update presence")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Presence updated",
	})
}

// HandleTypingIndicator handles typing indicator updates
// POST /v1/typing
func (h *Handler) HandleTypingIndicator(c *gin.Context) {
	var req TypingIndicatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get user ID from context (set by auth middleware)
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
	conversationID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		response.ValidationError(c, "Invalid conversation ID")
		return
	}

	// Call service to broadcast typing indicator
	if err := h.chatService.BroadcastTypingIndicator(c.Request.Context(), conversationID, userID, req.IsTyping); err != nil {
		response.InternalError(c, "Failed to broadcast typing indicator")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Typing indicator sent",
	})
}

package video

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/service/video"
	"secureconnect-backend/pkg/response"
)

// Handler handles video call HTTP requests
type Handler struct {
	videoService *video.Service
}

// NewHandler creates a new video handler
func NewHandler(videoService *video.Service) *Handler {
	return &Handler{
		videoService: videoService,
	}
}

// InitiateCallRequest represents call initiation request
type InitiateCallRequest struct {
	CallType       string   `json:"call_type" binding:"required,oneof=audio video"`
	ConversationID string   `json:"conversation_id" binding:"required,uuid"`
	CalleeIDs      []string `json:"callee_ids" binding:"required,min=1"`
}

// InitiateCall starts a new call
// POST /v1/calls/initiate
func (h *Handler) InitiateCall(c *gin.Context) {
	var req InitiateCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get caller ID from context
	callerIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	callerID, ok := callerIDVal.(uuid.UUID)
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

	// Parse callee IDs
	calleeUUIDs := make([]uuid.UUID, len(req.CalleeIDs))
	for i, idStr := range req.CalleeIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.ValidationError(c, "Invalid callee ID: "+idStr)
			return
		}
		calleeUUIDs[i] = id
	}

	// Initiate call
	output, err := h.videoService.InitiateCall(c.Request.Context(), &video.InitiateCallInput{
		CallType:       video.CallType(req.CallType),
		ConversationID: conversationID,
		CallerID:       callerID,
		CalleeIDs:      calleeUUIDs,
	})

	if err != nil {
		response.InternalError(c, "Failed to initiate call")
		return
	}

	response.Success(c, http.StatusCreated, output)
}

// EndCall terminates a call
// POST /v1/calls/:id/end
func (h *Handler) EndCall(c *gin.Context) {
	callIDStr := c.Param("id")

	callID, err := uuid.Parse(callIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid call ID")
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

	// End call
	if err := h.videoService.EndCall(c.Request.Context(), callID, userID); err != nil {
		response.InternalError(c, "Failed to end call")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Call ended",
		"call_id": callID,
	})
}

// JoinCall joins an ongoing call
// POST /v1/calls/:id/join
func (h *Handler) JoinCall(c *gin.Context) {
	callIDStr := c.Param("id")

	callID, err := uuid.Parse(callIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid call ID")
		return
	}

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

	// Join call
	if err := h.videoService.JoinCall(c.Request.Context(), callID, userID); err != nil {
		response.InternalError(c, "Failed to join call")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Joined call",
		"call_id": callID,
	})
}

// GetCallStatus retrieves call information
// GET /v1/calls/:id
func (h *Handler) GetCallStatus(c *gin.Context) {
	callIDStr := c.Param("id")

	callID, err := uuid.Parse(callIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid call ID")
		return
	}

	call, err := h.videoService.GetCallStatus(c.Request.Context(), callID)
	if err != nil {
		response.NotFound(c, "Call not found")
		return
	}

	response.Success(c, http.StatusOK, call)
}

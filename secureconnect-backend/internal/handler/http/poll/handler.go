package poll

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/service/poll"
	"secureconnect-backend/pkg/response"
)

// Handler handles poll HTTP requests
type Handler struct {
	pollService *poll.Service
}

// NewHandler creates a new poll handler
func NewHandler(pollService *poll.Service) *Handler {
	return &Handler{
		pollService: pollService,
	}
}

// CreatePollRequest represents create poll request
type CreatePollRequest struct {
	ConversationID  string          `json:"conversation_id" binding:"required,uuid"`
	Question        string          `json:"question" binding:"required,min=1,max=500"`
	PollType        domain.PollType `json:"poll_type" binding:"required,oneof=single multi"`
	AllowVoteChange bool            `json:"allow_vote_change"`
	ExpiresAt       *time.Time      `json:"expires_at"`
	Options         []string        `json:"options" binding:"required,min=2,max=10"`
}

// VoteRequest represents vote request
type VoteRequest struct {
	PollID    string   `json:"poll_id" binding:"required,uuid"`
	OptionIDs []string `json:"option_ids" binding:"required,min=1,max=10"`
}

// ClosePollRequest represents close poll request
type ClosePollRequest struct {
	PollID string `json:"poll_id" binding:"required,uuid"`
	Force  bool   `json:"force"` // If true, close even if not creator (admin only)
}

// GetPollsQuery represents query parameters for listing polls
type GetPollsQuery struct {
	ConversationID string `form:"conversation_id" binding:"required,uuid"`
	Page           int    `form:"page"`
	PageSize       int    `form:"page_size"`
}

// CreatePoll handles creating a new poll
// POST /v1/polls
func (h *Handler) CreatePoll(c *gin.Context) {
	var req CreatePollRequest
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

	// Call service
	output, err := h.pollService.CreatePoll(c.Request.Context(), &poll.CreatePollInput{
		ConversationID:  conversationID,
		CreatorID:       userID,
		Question:        req.Question,
		PollType:        req.PollType,
		AllowVoteChange: req.AllowVoteChange,
		ExpiresAt:       req.ExpiresAt,
		Options:         req.Options,
	})

	if err != nil {
		// Handle specific errors
		switch err.Error() {
		case domain.ErrInvalidPollType.Error():
			response.ValidationError(c, "Invalid poll type")
		case domain.ErrInsufficientOptions.Error():
			response.ValidationError(c, "At least 2 options are required")
		case domain.ErrTooManyOptions.Error():
			response.ValidationError(c, "Maximum 10 options allowed")
		case "user is not a participant in this conversation":
			response.Forbidden(c, "You are not a participant in this conversation")
		default:
			response.InternalError(c, "Failed to create poll")
		}
		return
	}

	response.Success(c, http.StatusCreated, output.Poll)
}

// GetPoll handles retrieving a poll
// GET /v1/polls/:poll_id
func (h *Handler) GetPoll(c *gin.Context) {
	pollIDStr := c.Param("poll_id")
	pollID, err := uuid.Parse(pollIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid poll ID")
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
	output, err := h.pollService.GetPoll(c.Request.Context(), &poll.GetPollInput{
		PollID: pollID,
		UserID: userID,
	})

	if err != nil {
		if err.Error() == "poll not found" {
			response.NotFound(c, "Poll not found")
			return
		}
		response.InternalError(c, "Failed to get poll")
		return
	}

	response.Success(c, http.StatusOK, output.Poll)
}

// GetPolls handles retrieving polls for a conversation
// GET /v1/polls?conversation_id=uuid&page=1&page_size=20
func (h *Handler) GetPolls(c *gin.Context) {
	var query GetPollsQuery
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

	// Set default pagination
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	// Call service
	output, err := h.pollService.GetPolls(c.Request.Context(), &poll.GetPollsInput{
		ConversationID: conversationID,
		Page:           query.Page,
		PageSize:       query.PageSize,
		UserID:         userID,
	})

	if err != nil {
		response.InternalError(c, "Failed to get polls")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"polls":     output.Polls,
		"total":     output.Total,
		"page":      output.Page,
		"page_size": output.PageSize,
		"has_more":  output.HasMore,
	})
}

// Vote handles casting a vote in a poll
// POST /v1/polls/vote
func (h *Handler) Vote(c *gin.Context) {
	var req VoteRequest
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

	// Parse poll ID
	pollID, err := uuid.Parse(req.PollID)
	if err != nil {
		response.ValidationError(c, "Invalid poll ID")
		return
	}

	// Parse option IDs
	optionIDs := make([]uuid.UUID, len(req.OptionIDs))
	for i, optionIDStr := range req.OptionIDs {
		optionID, err := uuid.Parse(optionIDStr)
		if err != nil {
			response.ValidationError(c, "Invalid option ID")
			return
		}
		optionIDs[i] = optionID
	}

	// Call service
	output, err := h.pollService.Vote(c.Request.Context(), &poll.VoteInput{
		PollID:    pollID,
		UserID:    userID,
		OptionIDs: optionIDs,
	})

	if err != nil {
		// Handle specific errors
		switch err.Error() {
		case domain.ErrPollNotFound.Error():
			response.NotFound(c, "Poll not found")
		case domain.ErrPollClosed.Error():
			response.Conflict(c, "Poll is closed")
		case domain.ErrPollExpired.Error():
			response.Conflict(c, "Poll has expired")
		case domain.ErrAlreadyVoted.Error():
			response.Conflict(c, "You have already voted on this poll")
		case domain.ErrMultipleOptionsNotAllowed.Error():
			response.ValidationError(c, "Multiple options not allowed for single-choice polls")
		case domain.ErrAtLeastOneOptionRequired.Error():
			response.ValidationError(c, "At least one option must be selected")
		case domain.ErrOptionNotFound.Error():
			response.NotFound(c, "Poll option not found")
		default:
			response.InternalError(c, "Failed to cast vote")
		}
		return
	}

	response.Success(c, http.StatusOK, output.Poll)
}

// ClosePoll handles closing a poll
// POST /v1/polls/close
func (h *Handler) ClosePoll(c *gin.Context) {
	var req ClosePollRequest
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

	// Parse poll ID
	pollID, err := uuid.Parse(req.PollID)
	if err != nil {
		response.ValidationError(c, "Invalid poll ID")
		return
	}

	// Call service
	output, err := h.pollService.ClosePoll(c.Request.Context(), &poll.ClosePollInput{
		PollID: pollID,
		UserID: userID,
		Force:  req.Force,
	})

	if err != nil {
		// Handle specific errors
		switch err.Error() {
		case "poll not found":
			response.NotFound(c, "Poll not found")
		case domain.ErrNotPollCreator.Error():
			response.Forbidden(c, "Only the poll creator can close this poll")
		default:
			response.InternalError(c, "Failed to close poll")
		}
		return
	}

	response.Success(c, http.StatusOK, output.Poll)
}

// DeletePoll handles deleting a poll
// DELETE /v1/polls/:poll_id
func (h *Handler) DeletePoll(c *gin.Context) {
	pollIDStr := c.Param("poll_id")
	pollID, err := uuid.Parse(pollIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid poll ID")
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
	err = h.pollService.DeletePoll(c.Request.Context(), &poll.DeletePollInput{
		PollID: pollID,
		UserID: userID,
	})

	if err != nil {
		// Handle specific errors
		switch err.Error() {
		case "poll not found":
			response.NotFound(c, "Poll not found")
		case domain.ErrNotPollCreator.Error():
			response.Forbidden(c, "Only the poll creator can delete this poll")
		default:
			response.InternalError(c, "Failed to delete poll")
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Poll deleted successfully",
	})
}

// GetActivePolls handles retrieving active polls for a conversation
// GET /v1/polls/active?conversation_id=uuid&page=1&page_size=20
func (h *Handler) GetActivePolls(c *gin.Context) {
	var query GetPollsQuery
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

	// Set default pagination
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	// Call service
	output, err := h.pollService.GetActivePolls(c.Request.Context(), &poll.GetActivePollsInput{
		ConversationID: conversationID,
		Page:           query.Page,
		PageSize:       query.PageSize,
		UserID:         userID,
	})

	if err != nil {
		response.InternalError(c, "Failed to get active polls")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"polls":     output.Polls,
		"total":     output.Total,
		"page":      output.Page,
		"page_size": output.PageSize,
		"has_more":  output.HasMore,
	})
}

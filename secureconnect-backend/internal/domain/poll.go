package domain

import (
	"time"

	"github.com/google/uuid"
)

// PollType represents the type of poll (single-choice or multi-choice)
type PollType string

const (
	// PollTypeSingle represents a single-choice poll
	PollTypeSingle PollType = "single"
	// PollTypeMulti represents a multi-choice poll
	PollTypeMulti PollType = "multi"
)

// Poll represents a poll entity in the system
// Maps to CockroachDB polls table
type Poll struct {
	PollID          uuid.UUID   `json:"poll_id" db:"poll_id"`
	ConversationID  uuid.UUID   `json:"conversation_id" db:"conversation_id"`
	CreatorID       uuid.UUID   `json:"creator_id" db:"creator_id"`
	Question        string      `json:"question" db:"question"`
	PollType        PollType    `json:"poll_type" db:"poll_type"`
	AllowVoteChange bool        `json:"allow_vote_change" db:"allow_vote_change"`
	ExpiresAt       *time.Time  `json:"expires_at,omitempty" db:"expires_at"`
	IsClosed        bool        `json:"is_closed" db:"is_closed"`
	ClosedAt        *time.Time  `json:"closed_at,omitempty" db:"closed_at"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
	TotalVotes      int         `json:"total_votes,omitempty"`       // Computed field
	TotalVoters     int         `json:"total_voters,omitempty"`      // Computed field
	UserVoted       bool        `json:"user_voted,omitempty"`        // Computed field for current user
	UserVoteOptions []uuid.UUID `json:"user_vote_options,omitempty"` // Computed field for current user
}

// PollCreate represents data needed to create a new poll
type PollCreate struct {
	ConversationID  uuid.UUID  `json:"conversation_id" binding:"required"`
	Question        string     `json:"question" binding:"required,min=1,max=500"`
	PollType        PollType   `json:"poll_type" binding:"required,oneof=single multi"`
	AllowVoteChange bool       `json:"allow_vote_change"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Options         []string   `json:"options" binding:"required,min=2,max=10"` // At least 2 options, max 10
}

// PollUpdate represents data needed to update a poll
type PollUpdate struct {
	IsClosed *bool `json:"is_closed"`
}

// PollOption represents an option in a poll
// Maps to CockroachDB poll_options table
type PollOption struct {
	OptionID     uuid.UUID `json:"option_id" db:"option_id"`
	PollID       uuid.UUID `json:"poll_id" db:"poll_id"`
	OptionText   string    `json:"option_text" db:"option_text"`
	DisplayOrder int       `json:"display_order" db:"display_order"`
	VoteCount    int       `json:"vote_count,omitempty"`   // Computed field
	VotePercent  float64   `json:"vote_percent,omitempty"` // Computed field
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// PollVote represents a vote in a poll
// Maps to CockroachDB poll_votes table
type PollVote struct {
	VoteID   uuid.UUID `json:"vote_id" db:"vote_id"`
	PollID   uuid.UUID `json:"poll_id" db:"poll_id"`
	OptionID uuid.UUID `json:"option_id" db:"option_id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	VotedAt  time.Time `json:"voted_at" db:"voted_at"`
}

// VoteRequest represents data needed to cast a vote
type VoteRequest struct {
	PollID    uuid.UUID   `json:"poll_id" binding:"required"`
	OptionIDs []uuid.UUID `json:"option_ids" binding:"required,min=1,max=10"` // At least 1 option, max 10
}

// PollResponse represents the poll returned to clients
type PollResponse struct {
	PollID          uuid.UUID     `json:"poll_id"`
	ConversationID  uuid.UUID     `json:"conversation_id"`
	CreatorID       uuid.UUID     `json:"creator_id"`
	CreatorName     string        `json:"creator_name,omitempty"` // Joined from users table
	Question        string        `json:"question"`
	PollType        PollType      `json:"poll_type"`
	AllowVoteChange bool          `json:"allow_vote_change"`
	ExpiresAt       *time.Time    `json:"expires_at,omitempty"`
	IsClosed        bool          `json:"is_closed"`
	ClosedAt        *time.Time    `json:"closed_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	TotalVotes      int           `json:"total_votes"`
	TotalVoters     int           `json:"total_voters"`
	UserVoted       bool          `json:"user_voted"`
	UserVoteOptions []uuid.UUID   `json:"user_vote_options,omitempty"`
	Options         []*PollOption `json:"options"`
}

// PollListResponse represents a paginated list of polls
type PollListResponse struct {
	Polls    []*PollResponse `json:"polls"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	HasMore  bool            `json:"has_more"`
}

// ToResponse converts Poll to PollResponse
func (p *Poll) ToResponse() *PollResponse {
	return &PollResponse{
		PollID:          p.PollID,
		ConversationID:  p.ConversationID,
		CreatorID:       p.CreatorID,
		Question:        p.Question,
		PollType:        p.PollType,
		AllowVoteChange: p.AllowVoteChange,
		ExpiresAt:       p.ExpiresAt,
		IsClosed:        p.IsClosed,
		ClosedAt:        p.ClosedAt,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		TotalVotes:      p.TotalVotes,
		TotalVoters:     p.TotalVoters,
		UserVoted:       p.UserVoted,
		UserVoteOptions: p.UserVoteOptions,
		Options:         []*PollOption{},
	}
}

// IsExpired checks if the poll has expired
func (p *Poll) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return p.ExpiresAt.Before(time.Now())
}

// CanVote checks if a user can vote on this poll
func (p *Poll) CanVote(userVoted bool) bool {
	// Check if poll is closed
	if p.IsClosed {
		return false
	}

	// Check if poll has expired
	if p.IsExpired() {
		return false
	}

	// Check if user has already voted and vote change is not allowed
	if userVoted && !p.AllowVoteChange {
		return false
	}

	return true
}

// CanChangeVote checks if a user can change their vote
func (p *Poll) CanChangeVote() bool {
	return p.AllowVoteChange && !p.IsClosed && !p.IsExpired()
}

// ValidatePollType validates the poll type
func ValidatePollType(pollType PollType) bool {
	return pollType == PollTypeSingle || pollType == PollTypeMulti
}

// ValidateOptions validates poll options
func ValidateOptions(options []string, pollType PollType) error {
	if len(options) < 2 {
		return ErrInsufficientOptions
	}
	if len(options) > 10 {
		return ErrTooManyOptions
	}

	// For single-choice polls, we don't need additional validation
	// For multi-choice polls, we can add additional constraints if needed

	return nil
}

// ValidateVoteRequest validates a vote request
func ValidateVoteRequest(req *VoteRequest, pollType PollType, allowVoteChange bool, userVoted bool) error {
	// Check if user has already voted and vote change is not allowed
	if userVoted && !allowVoteChange {
		return ErrAlreadyVoted
	}

	// For single-choice polls, only one option is allowed
	if pollType == PollTypeSingle && len(req.OptionIDs) > 1 {
		return ErrMultipleOptionsNotAllowed
	}

	// For multi-choice polls, ensure at least one option is selected
	if pollType == PollTypeMulti && len(req.OptionIDs) < 1 {
		return ErrAtLeastOneOptionRequired
	}

	return nil
}

// Poll-related errors
var (
	ErrInsufficientOptions       = NewError("INSUFFICIENT_OPTIONS", "At least 2 options are required")
	ErrTooManyOptions            = NewError("TOO_MANY_OPTIONS", "Maximum 10 options allowed")
	ErrPollNotFound              = NewError("POLL_NOT_FOUND", "Poll not found")
	ErrPollExpired               = NewError("POLL_EXPIRED", "Poll has expired")
	ErrPollClosed                = NewError("POLL_CLOSED", "Poll is closed")
	ErrAlreadyVoted              = NewError("ALREADY_VOTED", "You have already voted on this poll")
	ErrMultipleOptionsNotAllowed = NewError("MULTIPLE_OPTIONS_NOT_ALLOWED", "Multiple options not allowed for single-choice polls")
	ErrAtLeastOneOptionRequired  = NewError("AT_LEAST_ONE_OPTION_REQUIRED", "At least one option must be selected")
	ErrOptionNotFound            = NewError("OPTION_NOT_FOUND", "Poll option not found")
	ErrNotPollCreator            = NewError("NOT_POLL_CREATOR", "Only the poll creator can perform this action")
	ErrInvalidPollType           = NewError("INVALID_POLL_TYPE", "Invalid poll type")
)

// Error represents a domain error
type Error struct {
	Code    string
	Message string
}

// NewError creates a new domain error
func NewError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

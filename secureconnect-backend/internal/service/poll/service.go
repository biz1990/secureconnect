package poll

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/pkg/logger"
)

// PollRepository interface for poll data operations
type PollRepository interface {
	CreatePoll(ctx context.Context, poll *domain.Poll, options []string) error
	GetPollByID(ctx context.Context, pollID uuid.UUID) (*domain.Poll, error)
	GetPollByIDWithVotes(ctx context.Context, pollID uuid.UUID) (*domain.Poll, error)
	GetPollByIDWithUserVote(ctx context.Context, pollID, userID uuid.UUID) (*domain.Poll, error)
	GetPollsByConversation(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.Poll, int, error)
	GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]*domain.PollOption, error)
	GetPollOptionsWithVotes(ctx context.Context, pollID uuid.UUID) ([]*domain.PollOption, error)
	CastVote(ctx context.Context, vote *domain.PollVote) error
	ChangeVote(ctx context.Context, pollID, userID uuid.UUID, newOptionIDs []uuid.UUID) error
	GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]*domain.PollVote, error)
	ClosePoll(ctx context.Context, pollID uuid.UUID) error
	DeletePoll(ctx context.Context, pollID uuid.UUID) error
	GetActivePolls(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.Poll, int, error)
	IsPollCreator(ctx context.Context, pollID, userID uuid.UUID) (bool, error)
	GetPollsByCreator(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]*domain.Poll, int, error)
}

// ConversationRepository interface for checking conversation membership
type ConversationRepository interface {
	GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error)
}

// UserRepository interface for getting user details
type UserRepository interface {
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

// Publisher interface for WebSocket events
type Publisher interface {
	Publish(ctx context.Context, channel string, message interface{}) error
}

// RedisAdapter adapts redis.Client to Publisher interface
type RedisAdapter struct {
	Client *redis.Client
}

// Publish publishes message to Redis
func (a *RedisAdapter) Publish(ctx context.Context, channel string, message interface{}) error {
	return a.Client.Publish(ctx, channel, message).Err()
}

// Service handles poll business logic
type Service struct {
	pollRepo         PollRepository
	conversationRepo ConversationRepository
	userRepo         UserRepository
	publisher        Publisher
}

// NewService creates a new poll service
func NewService(
	pollRepo PollRepository,
	conversationRepo ConversationRepository,
	userRepo UserRepository,
	publisher Publisher,
) *Service {
	return &Service{
		pollRepo:         pollRepo,
		conversationRepo: conversationRepo,
		userRepo:         userRepo,
		publisher:        publisher,
	}
}

// CreatePollInput contains data for creating a poll
type CreatePollInput struct {
	ConversationID  uuid.UUID
	CreatorID       uuid.UUID
	Question        string
	PollType        domain.PollType
	AllowVoteChange bool
	ExpiresAt       *time.Time
	Options         []string
}

// CreatePollOutput contains created poll info
type CreatePollOutput struct {
	Poll *domain.PollResponse
}

// CreatePoll creates a new poll with options
func (s *Service) CreatePoll(ctx context.Context, input *CreatePollInput) (*CreatePollOutput, error) {
	// Validate poll type
	if !domain.ValidatePollType(input.PollType) {
		return nil, domain.ErrInvalidPollType
	}

	// Validate options
	if err := domain.ValidateOptions(input.Options, input.PollType); err != nil {
		return nil, err
	}

	// Verify user is a participant in the conversation
	participants, err := s.conversationRepo.GetParticipants(ctx, input.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation participants: %w", err)
	}

	isParticipant := false
	for _, participantID := range participants {
		if participantID == input.CreatorID {
			isParticipant = true
			break
		}
	}

	if !isParticipant {
		return nil, fmt.Errorf("user is not a participant in this conversation")
	}

	// Create poll entity
	poll := &domain.Poll{
		PollID:          uuid.New(),
		ConversationID:  input.ConversationID,
		CreatorID:       input.CreatorID,
		Question:        input.Question,
		PollType:        input.PollType,
		AllowVoteChange: input.AllowVoteChange,
		ExpiresAt:       input.ExpiresAt,
		IsClosed:        false,
	}

	// Create poll with options in transaction
	if err := s.pollRepo.CreatePoll(ctx, poll, input.Options); err != nil {
		return nil, fmt.Errorf("failed to create poll: %w", err)
	}

	// Get poll with options for response
	options, err := s.pollRepo.GetPollOptions(ctx, poll.PollID)
	if err != nil {
		logger.Warn("Failed to get poll options after creation",
			zap.String("poll_id", poll.PollID.String()),
			zap.Error(err))
	}

	// Get creator details
	creator, err := s.userRepo.GetByID(ctx, poll.CreatorID)
	if err != nil {
		logger.Warn("Failed to get poll creator details",
			zap.String("creator_id", poll.CreatorID.String()),
			zap.Error(err))
	}

	// Build response
	response := poll.ToResponse()
	response.Options = options
	if creator != nil {
		response.CreatorName = creator.DisplayName
		if response.CreatorName == "" {
			response.CreatorName = creator.Username
		}
	}

	// Publish poll_created event (non-blocking)
	go s.publishPollCreated(context.Background(), poll.ConversationID, response)

	return &CreatePollOutput{Poll: response}, nil
}

// GetPollInput contains query parameters for getting a poll
type GetPollInput struct {
	PollID uuid.UUID
	UserID uuid.UUID
}

// GetPollOutput contains poll info
type GetPollOutput struct {
	Poll *domain.PollResponse
}

// GetPoll retrieves a poll with its options and vote counts
func (s *Service) GetPoll(ctx context.Context, input *GetPollInput) (*GetPollOutput, error) {
	// Get poll with user vote info
	poll, err := s.pollRepo.GetPollByIDWithUserVote(ctx, input.PollID, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Get options with vote counts
	options, err := s.pollRepo.GetPollOptionsWithVotes(ctx, input.PollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}

	// Get creator details
	creator, err := s.userRepo.GetByID(ctx, poll.CreatorID)
	if err != nil {
		logger.Warn("Failed to get poll creator details",
			zap.String("creator_id", poll.CreatorID.String()),
			zap.Error(err))
	}

	// Build response
	response := poll.ToResponse()
	response.Options = options
	if creator != nil {
		response.CreatorName = creator.DisplayName
		if response.CreatorName == "" {
			response.CreatorName = creator.Username
		}
	}

	return &GetPollOutput{Poll: response}, nil
}

// GetPollsInput contains query parameters for listing polls
type GetPollsInput struct {
	ConversationID uuid.UUID
	Page           int
	PageSize       int
	UserID         uuid.UUID
}

// GetPollsOutput contains poll list
type GetPollsOutput struct {
	Polls    []*domain.PollResponse
	Total    int
	Page     int
	PageSize int
	HasMore  bool
}

// GetPolls retrieves polls for a conversation with pagination
func (s *Service) GetPolls(ctx context.Context, input *GetPollsInput) (*GetPollsOutput, error) {
	// Set default pagination
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	offset := (input.Page - 1) * input.PageSize

	// Get polls
	polls, total, err := s.pollRepo.GetPollsByConversation(ctx, input.ConversationID, input.PageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get polls: %w", err)
	}

	// Build responses
	responses := make([]*domain.PollResponse, len(polls))
	for i, poll := range polls {
		// Get options
		options, err := s.pollRepo.GetPollOptions(ctx, poll.PollID)
		if err != nil {
			logger.Warn("Failed to get poll options",
				zap.String("poll_id", poll.PollID.String()),
				zap.Error(err))
			options = []*domain.PollOption{}
		}

		// Get user vote info
		pollWithVote, err := s.pollRepo.GetPollByIDWithUserVote(ctx, poll.PollID, input.UserID)
		if err != nil {
			logger.Warn("Failed to get user vote info",
				zap.String("poll_id", poll.PollID.String()),
				zap.String("user_id", input.UserID.String()),
				zap.Error(err))
		} else {
			poll.UserVoted = pollWithVote.UserVoted
			poll.UserVoteOptions = pollWithVote.UserVoteOptions
		}

		// Get creator details
		creator, err := s.userRepo.GetByID(ctx, poll.CreatorID)
		response := poll.ToResponse()
		response.Options = options
		if err == nil && creator != nil {
			response.CreatorName = creator.DisplayName
			if response.CreatorName == "" {
				response.CreatorName = creator.Username
			}
		}
		responses[i] = response
	}

	return &GetPollsOutput{
		Polls:    responses,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
		HasMore:  (input.Page * input.PageSize) < total,
	}, nil
}

// VoteInput contains data for casting a vote
type VoteInput struct {
	PollID    uuid.UUID
	UserID    uuid.UUID
	OptionIDs []uuid.UUID
}

// VoteOutput contains vote result
type VoteOutput struct {
	Poll *domain.PollResponse
}

// Vote casts a vote in a poll
func (s *Service) Vote(ctx context.Context, input *VoteInput) (*VoteOutput, error) {
	// Get poll with user vote info
	poll, err := s.pollRepo.GetPollByIDWithUserVote(ctx, input.PollID, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Check if user can vote
	if !poll.CanVote(poll.UserVoted) {
		if poll.IsClosed {
			return nil, domain.ErrPollClosed
		}
		if poll.IsExpired() {
			return nil, domain.ErrPollExpired
		}
		if poll.UserVoted && !poll.AllowVoteChange {
			return nil, domain.ErrAlreadyVoted
		}
	}

	// Validate vote request
	if err := domain.ValidateVoteRequest(&domain.VoteRequest{
		PollID:    input.PollID,
		OptionIDs: input.OptionIDs,
	}, poll.PollType, poll.AllowVoteChange, poll.UserVoted); err != nil {
		return nil, err
	}

	// Verify options belong to this poll
	options, err := s.pollRepo.GetPollOptions(ctx, input.PollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}

	optionMap := make(map[uuid.UUID]bool)
	for _, opt := range options {
		optionMap[opt.OptionID] = true
	}

	for _, optionID := range input.OptionIDs {
		if !optionMap[optionID] {
			return nil, domain.ErrOptionNotFound
		}
	}

	// Cast vote or change vote
	if poll.UserVoted && poll.AllowVoteChange {
		// Change vote
		if err := s.pollRepo.ChangeVote(ctx, input.PollID, input.UserID, input.OptionIDs); err != nil {
			return nil, fmt.Errorf("failed to change vote: %w", err)
		}
	} else {
		// Cast new vote
		for _, optionID := range input.OptionIDs {
			vote := &domain.PollVote{
				VoteID:   uuid.New(),
				PollID:   input.PollID,
				OptionID: optionID,
				UserID:   input.UserID,
				VotedAt:  time.Now(),
			}
			if err := s.pollRepo.CastVote(ctx, vote); err != nil {
				return nil, fmt.Errorf("failed to cast vote: %w", err)
			}
		}
	}

	// Get updated poll
	updatedPoll, err := s.pollRepo.GetPollByIDWithUserVote(ctx, input.PollID, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated poll: %w", err)
	}

	// Get options with vote counts
	updatedOptions, err := s.pollRepo.GetPollOptionsWithVotes(ctx, input.PollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated poll options: %w", err)
	}

	// Get creator details
	creator, err := s.userRepo.GetByID(ctx, updatedPoll.CreatorID)
	response := updatedPoll.ToResponse()
	response.Options = updatedOptions
	if err == nil && creator != nil {
		response.CreatorName = creator.DisplayName
		if response.CreatorName == "" {
			response.CreatorName = creator.Username
		}
	}

	// Publish poll_voted event (non-blocking)
	go s.publishPollVoted(context.Background(), updatedPoll.ConversationID, response)

	return &VoteOutput{Poll: response}, nil
}

// ClosePollInput contains data for closing a poll
type ClosePollInput struct {
	PollID uuid.UUID
	UserID uuid.UUID
	Force  bool // If true, close even if not creator (admin only)
}

// ClosePollOutput contains closed poll info
type ClosePollOutput struct {
	Poll *domain.PollResponse
}

// ClosePoll closes a poll
func (s *Service) ClosePoll(ctx context.Context, input *ClosePollInput) (*ClosePollOutput, error) {
	// Check if user is poll creator (unless force)
	if !input.Force {
		isCreator, err := s.pollRepo.IsPollCreator(ctx, input.PollID, input.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to check poll creator: %w", err)
		}
		if !isCreator {
			return nil, domain.ErrNotPollCreator
		}
	}

	// Close poll
	if err := s.pollRepo.ClosePoll(ctx, input.PollID); err != nil {
		return nil, fmt.Errorf("failed to close poll: %w", err)
	}

	// Get updated poll
	poll, err := s.pollRepo.GetPollByIDWithVotes(ctx, input.PollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated poll: %w", err)
	}

	// Get options with vote counts
	options, err := s.pollRepo.GetPollOptionsWithVotes(ctx, input.PollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}

	// Get creator details
	creator, err := s.userRepo.GetByID(ctx, poll.CreatorID)
	response := poll.ToResponse()
	response.Options = options
	if err == nil && creator != nil {
		response.CreatorName = creator.DisplayName
		if response.CreatorName == "" {
			response.CreatorName = creator.Username
		}
	}

	// Publish poll_closed event (non-blocking)
	go s.publishPollClosed(context.Background(), poll.ConversationID, response)

	return &ClosePollOutput{Poll: response}, nil
}

// DeletePollInput contains data for deleting a poll
type DeletePollInput struct {
	PollID uuid.UUID
	UserID uuid.UUID
}

// DeletePoll deletes a poll
func (s *Service) DeletePoll(ctx context.Context, input *DeletePollInput) error {
	// Check if user is poll creator
	isCreator, err := s.pollRepo.IsPollCreator(ctx, input.PollID, input.UserID)
	if err != nil {
		return fmt.Errorf("failed to check poll creator: %w", err)
	}
	if !isCreator {
		return domain.ErrNotPollCreator
	}

	// Delete poll (cascade will handle options and votes)
	if err := s.pollRepo.DeletePoll(ctx, input.PollID); err != nil {
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	return nil
}

// GetActivePollsInput contains query parameters for listing active polls
type GetActivePollsInput struct {
	ConversationID uuid.UUID
	Page           int
	PageSize       int
	UserID         uuid.UUID
}

// GetActivePolls retrieves active (not closed and not expired) polls
func (s *Service) GetActivePolls(ctx context.Context, input *GetActivePollsInput) (*GetPollsOutput, error) {
	// Set default pagination
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	offset := (input.Page - 1) * input.PageSize

	// Get active polls
	polls, total, err := s.pollRepo.GetActivePolls(ctx, input.ConversationID, input.PageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get active polls: %w", err)
	}

	// Build responses
	responses := make([]*domain.PollResponse, len(polls))
	for i, poll := range polls {
		// Get options
		options, err := s.pollRepo.GetPollOptions(ctx, poll.PollID)
		if err != nil {
			logger.Warn("Failed to get poll options",
				zap.String("poll_id", poll.PollID.String()),
				zap.Error(err))
			options = []*domain.PollOption{}
		}

		// Get user vote info
		pollWithVote, err := s.pollRepo.GetPollByIDWithUserVote(ctx, poll.PollID, input.UserID)
		if err != nil {
			logger.Warn("Failed to get user vote info",
				zap.String("poll_id", poll.PollID.String()),
				zap.String("user_id", input.UserID.String()),
				zap.Error(err))
		} else {
			poll.UserVoted = pollWithVote.UserVoted
			poll.UserVoteOptions = pollWithVote.UserVoteOptions
		}

		// Get creator details
		creator, err := s.userRepo.GetByID(ctx, poll.CreatorID)
		response := poll.ToResponse()
		response.Options = options
		if err == nil && creator != nil {
			response.CreatorName = creator.DisplayName
			if response.CreatorName == "" {
				response.CreatorName = creator.Username
			}
		}
		responses[i] = response
	}

	return &GetPollsOutput{
		Polls:    responses,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
		HasMore:  (input.Page * input.PageSize) < total,
	}, nil
}

// publishPollCreated publishes a poll_created event to Redis
func (s *Service) publishPollCreated(ctx context.Context, conversationID uuid.UUID, poll *domain.PollResponse) {
	channel := fmt.Sprintf("poll:%s", conversationID)
	event := map[string]interface{}{
		"type": "poll_created",
		"data": poll,
	}

	messageJSON, err := json.Marshal(event)
	if err != nil {
		logger.Warn("Failed to marshal poll_created event",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
		return
	}

	if err := s.publisher.Publish(ctx, channel, messageJSON); err != nil {
		logger.Warn("Failed to publish poll_created event",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

// publishPollVoted publishes a poll_voted event to Redis
func (s *Service) publishPollVoted(ctx context.Context, conversationID uuid.UUID, poll *domain.PollResponse) {
	channel := fmt.Sprintf("poll:%s", conversationID)
	event := map[string]interface{}{
		"type": "poll_voted",
		"data": poll,
	}

	messageJSON, err := json.Marshal(event)
	if err != nil {
		logger.Warn("Failed to marshal poll_voted event",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
		return
	}

	if err := s.publisher.Publish(ctx, channel, messageJSON); err != nil {
		logger.Warn("Failed to publish poll_voted event",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

// publishPollClosed publishes a poll_closed event to Redis
func (s *Service) publishPollClosed(ctx context.Context, conversationID uuid.UUID, poll *domain.PollResponse) {
	channel := fmt.Sprintf("poll:%s", conversationID)
	event := map[string]interface{}{
		"type": "poll_closed",
		"data": poll,
	}

	messageJSON, err := json.Marshal(event)
	if err != nil {
		logger.Warn("Failed to marshal poll_closed event",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
		return
	}

	if err := s.publisher.Publish(ctx, channel, messageJSON); err != nil {
		logger.Warn("Failed to publish poll_closed event",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

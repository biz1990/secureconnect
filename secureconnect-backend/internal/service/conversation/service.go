package conversation

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	
	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
)

// Service handles conversation business logic
type Service struct {
	conversationRepo *cockroach.ConversationRepository
}

// NewService creates a new conversation service
func NewService(conversationRepo *cockroach.ConversationRepository) *Service {
	return &Service{
		conversationRepo: conversationRepo,
	}
}

// CreateConversationInput contains conversation creation data
type CreateConversationInput struct {
	Title        string
	Type         string // "direct" or "group"
	CreatedBy    uuid.UUID
	Participants []uuid.UUID
	IsE2EEEnabled bool
}

// CreateConversation creates a new conversation
func (s *Service) CreateConversation(ctx context.Context, input *CreateConversationInput) (*domain.Conversation, error) {
	// Validate
	if input.Type != "direct" && input.Type != "group" {
		return nil, fmt.Errorf("invalid conversation type")
	}
	
	if input.Type == "direct" && len(input.Participants) != 2 {
		return nil, fmt.Errorf("direct conversation must have exactly 2 participants")
	}
	
	// Create conversation
	conversation := &domain.Conversation{
		ConversationID: uuid.New(),
		Title:          input.Title,
		Type:           input.Type,
		CreatedBy:      input.CreatedBy,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	
	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	
	// Add participants
	for _, userID := range input.Participants {
		role := "member"
		if userID == input.CreatedBy {
			role = "admin"
		}
		
		if err := s.conversationRepo.AddParticipant(ctx, conversation.ConversationID, userID, role); err != nil {
			return nil, fmt.Errorf("failed to add participant: %w", err)
		}
	}
	
	// Set E2EE settings
	settings := &domain.ConversationSettings{
		ConversationID:  conversation.ConversationID,
		IsE2EEEnabled:   input.IsE2EEEnabled,
	}
	
	if err := s.conversationRepo.UpdateSettings(ctx, conversation.ConversationID, settings); err != nil {
		return nil, fmt.Errorf("failed to set settings: %w", err)
	}
	
	return conversation, nil
}

// GetConversation retrieves a conversation by ID
func (s *Service) GetConversation(ctx context.Context, conversationID uuid.UUID) (*domain.Conversation, error) {
	return s.conversationRepo.GetByID(ctx, conversationID)
}

// GetUserConversations retrieves all conversations for a user
func (s *Service) GetUserConversations(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Conversation, error) {
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	
	return s.conversationRepo.GetUserConversations(ctx, userID, limit, offset)
}

// UpdateE2EESettings updates conversation E2EE settings
func (s *Service) UpdateE2EESettings(ctx context.Context, conversationID uuid.UUID, enabled bool) error {
	settings := &domain.ConversationSettings{
		ConversationID:  conversationID,
		IsE2EEEnabled:   enabled,
	}
	
	return s.conversationRepo.UpdateSettings(ctx, conversationID, settings)
}

// GetSettings retrieves conversation settings
func (s *Service) GetSettings(ctx context.Context, conversationID uuid.UUID) (*domain.ConversationSettings, error) {
	return s.conversationRepo.GetSettings(ctx, conversationID)
}

// AddParticipants adds users to a conversation
func (s *Service) AddParticipants(ctx context.Context, conversationID uuid.UUID, userIDs []uuid.UUID) error {
	for _, userID := range userIDs {
		if err := s.conversationRepo.AddParticipant(ctx, conversationID, userID, "member"); err != nil {
			return fmt.Errorf("failed to add participant %s: %w", userID, err)
		}
	}
	return nil
}

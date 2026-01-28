package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/pkg/logger"
)

// MessageRepository interface
type MessageRepository interface {
	Save(ctx context.Context, message *domain.Message) error
	GetByConversation(ctx context.Context, conversationID uuid.UUID, limit int, pageState []byte) ([]*domain.Message, []byte, error)
}

// PresenceRepository interface
type PresenceRepository interface {
	SetUserOnline(ctx context.Context, userID uuid.UUID) error
	SetUserOffline(ctx context.Context, userID uuid.UUID) error
	RefreshPresence(ctx context.Context, userID uuid.UUID) error
	IsUserOnline(ctx context.Context, userID uuid.UUID) (bool, error)
	IsDegraded() bool
}

// Publisher interface
type Publisher interface {
	Publish(ctx context.Context, channel string, message interface{}) error
}

// NotificationService interface for triggering notifications
type NotificationService interface {
	CreateMessageNotification(ctx context.Context, userID uuid.UUID, senderName string, conversationID uuid.UUID) error
}

// ConversationRepository interface for getting participants
type ConversationRepository interface {
	GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error)
}

// UserRepository interface for getting sender details
type UserRepository interface {
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

// RedisAdapter adapts redis.Client to Publisher interface
type RedisAdapter struct {
	Client *redis.Client
}

// Publish publishes message to Redis
func (a *RedisAdapter) Publish(ctx context.Context, channel string, message interface{}) error {
	return a.Client.Publish(ctx, channel, message).Err()
}

// Service handles chat business logic
type Service struct {
	messageRepo         MessageRepository
	presenceRepo        PresenceRepository
	publisher           Publisher
	notificationService NotificationService
	conversationRepo    ConversationRepository
	userRepo            UserRepository
	notificationSem     chan struct{} // Semaphore for rate limiting notifications
}

// NewService creates a new chat service
func NewService(
	messageRepo MessageRepository,
	presenceRepo PresenceRepository,
	publisher Publisher,
	notificationService NotificationService,
	conversationRepo ConversationRepository,
	userRepo UserRepository,
) *Service {
	return &Service{
		messageRepo:         messageRepo,
		presenceRepo:        presenceRepo,
		publisher:           publisher,
		notificationService: notificationService,
		conversationRepo:    conversationRepo,
		userRepo:            userRepo,
		notificationSem:     make(chan struct{}, 100), // Limit to 100 concurrent notification routines
	}
}

// SendMessageInput contains message data
type SendMessageInput struct {
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Content        string
	IsEncrypted    bool
	MessageType    string
	Metadata       map[string]interface{}
}

// SendMessageOutput contains sent message info
type SendMessageOutput struct {
	Message *domain.MessageResponse
}

// SendMessage stores a message and publishes to real-time channel
func (s *Service) SendMessage(ctx context.Context, input *SendMessageInput) (*SendMessageOutput, error) {
	// Create message entity
	message := &domain.Message{
		MessageID:      uuid.New(),
		ConversationID: input.ConversationID,
		SenderID:       input.SenderID,
		Content:        input.Content,
		IsEncrypted:    input.IsEncrypted,
		MessageType:    input.MessageType,
		Metadata:       input.Metadata,
		SentAt:         time.Now(),
	}

	// Save to Cassandra
	if err := s.messageRepo.Save(ctx, message); err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// Trigger push notifications for conversation participants (non-blocking)
	// Create a new context with timeout for the goroutine
	notifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	go func() {
		defer cancel()

		// Acquire semaphore
		select {
		case s.notificationSem <- struct{}{}:
			defer func() { <-s.notificationSem }() // Release
			s.notifyMessageRecipients(notifyCtx, input.SenderID, input.ConversationID, input.Content)
		default:
			// If semaphore full, log warning and skip notification to preserve system stability
			logger.Warn("Notification queue full, skipping push notification",
				zap.String("conversation_id", input.ConversationID.String()))
		}
	}()

	// Publish to Redis Pub/Sub for real-time delivery
	channel := fmt.Sprintf("chat:%s", input.ConversationID)
	messageJSON, err := json.Marshal(message)
	if err != nil {
		// Log error but don't fail the request
		logger.Warn("Failed to marshal message for pub/sub",
			zap.String("conversation_id", input.ConversationID.String()),
			zap.String("sender_id", input.SenderID.String()),
			zap.Error(err))
	} else {
		if err := s.publisher.Publish(ctx, channel, messageJSON); err != nil {
			// Log error but don't fail the request
			logger.Warn("Failed to publish message to Redis",
				zap.String("conversation_id", input.ConversationID.String()),
				zap.String("sender_id", input.SenderID.String()),
				zap.Error(err))
		}
	}

	// Convert to response
	response := &domain.MessageResponse{
		MessageID:      message.MessageID,
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		Content:        message.Content,
		IsEncrypted:    message.IsEncrypted,
		MessageType:    message.MessageType,
		Metadata:       message.Metadata,
		SentAt:         message.SentAt,
	}

	return &SendMessageOutput{Message: response}, nil
}

// GetMessagesInput contains query parameters
type GetMessagesInput struct {
	ConversationID uuid.UUID
	Limit          int
	PageState      []byte
}

// GetMessagesOutput contains message list
type GetMessagesOutput struct {
	Messages      []*domain.MessageResponse
	NextPageState []byte
	HasMore       bool
}

// GetMessages retrieves conversation messages with pagination
func (s *Service) GetMessages(ctx context.Context, input *GetMessagesInput) (*GetMessagesOutput, error) {
	// Fetch messages from Cassandra
	messages, nextPageState, err := s.messageRepo.GetByConversation(
		ctx,
		input.ConversationID,
		input.Limit,
		input.PageState,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Convert to response format
	responses := make([]*domain.MessageResponse, len(messages))
	for i, msg := range messages {
		responses[i] = &domain.MessageResponse{
			MessageID:      msg.MessageID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			Content:        msg.Content,
			IsEncrypted:    msg.IsEncrypted,
			MessageType:    msg.MessageType,
			Metadata:       msg.Metadata,
			SentAt:         msg.SentAt,
		}
	}

	return &GetMessagesOutput{
		Messages:      responses,
		NextPageState: nextPageState,
		HasMore:       len(nextPageState) > 0,
	}, nil
}

// UpdatePresence updates user online/offline status
func (s *Service) UpdatePresence(ctx context.Context, userID uuid.UUID, online bool) error {
	// DEGRADED MODE: Skip presence updates when Redis is degraded
	if s.presenceRepo.IsDegraded() {
		logger.Warn("Presence update skipped (Redis degraded)",
			zap.String("service", "chat-service"),
			zap.String("user_id", userID.String()),
			zap.Bool("online", online))
		return nil // Return success to avoid breaking the flow
	}

	if online {
		return s.presenceRepo.SetUserOnline(ctx, userID)
	}
	return s.presenceRepo.SetUserOffline(ctx, userID)
}

// RefreshPresence keeps user status alive (heartbeat)
func (s *Service) RefreshPresence(ctx context.Context, userID uuid.UUID) error {
	// DEGRADED MODE: Skip presence refresh when Redis is degraded
	if s.presenceRepo.IsDegraded() {
		logger.Warn("Presence refresh skipped (Redis degraded)",
			zap.String("service", "chat-service"),
			zap.String("user_id", userID.String()))
		return nil // Return success to avoid breaking the flow
	}

	return s.presenceRepo.RefreshPresence(ctx, userID)
}

// notifyMessageRecipients sends push notifications to all conversation participants except sender
// This runs in a goroutine to avoid blocking the message send operation
func (s *Service) notifyMessageRecipients(ctx context.Context, senderID, conversationID uuid.UUID, _ string) {
	// Get sender details for notification
	sender, err := s.userRepo.GetByID(ctx, senderID)
	if err != nil {
		logger.Warn("Failed to get sender for notification",
			zap.String("sender_id", senderID.String()),
			zap.Error(err))
		return
	}

	senderName := sender.DisplayName
	if senderName == "" {
		senderName = sender.Username
	}

	// Get conversation participants
	participants, err := s.conversationRepo.GetParticipants(ctx, conversationID)
	if err != nil {
		logger.Warn("Failed to get conversation participants for notification",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
		return
	}

	// Send notification to each participant except sender
	for _, participantID := range participants {
		if participantID == senderID {
			continue // Don't notify the sender
		}

		// Create notification for this participant
		err := s.notificationService.CreateMessageNotification(ctx, participantID, senderName, conversationID)
		if err != nil {
			// Log error but continue with other participants
			logger.Warn("Failed to create message notification",
				zap.String("user_id", participantID.String()),
				zap.String("conversation_id", conversationID.String()),
				zap.String("sender_id", senderID.String()),
				zap.Error(err))
		}
	}
}

// truncateMessage truncates message content for preview in notifications
func truncateMessage(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return strings.TrimSpace(content[:maxLength]) + "..."
}

// BroadcastTypingIndicator broadcasts typing indicator to conversation participants
func (s *Service) BroadcastTypingIndicator(ctx context.Context, conversationID, userID uuid.UUID, isTyping bool) error {
	// Create typing indicator message
	typingMessage := map[string]interface{}{
		"type":            "typing_indicator",
		"conversation_id": conversationID.String(),
		"user_id":         userID.String(),
		"is_typing":       isTyping,
		"timestamp":       time.Now().Unix(),
	}

	messageJSON, err := json.Marshal(typingMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal typing indicator: %w", err)
	}

	// Publish to Redis Pub/Sub for real-time delivery
	channel := fmt.Sprintf("chat:%s", conversationID)
	if err := s.publisher.Publish(ctx, channel, messageJSON); err != nil {
		// Log error but don't fail the request
		logger.Warn("Failed to publish typing indicator",
			zap.String("conversation_id", conversationID.String()),
			zap.String("user_id", userID.String()),
			zap.Bool("is_typing", isTyping),
			zap.Error(err))
	}

	return nil
}

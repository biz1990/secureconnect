package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	
	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cassandra"
	redisRepo "secureconnect-backend/internal/repository/redis"
)

// Service handles chat business logic
type Service struct {
	messageRepo   *cassandra.MessageRepository
	presenceRepo  *redisRepo.PresenceRepository
	redisClient   *redis.Client // For Pub/Sub
}

// NewService creates a new chat service
func NewService(
	messageRepo *cassandra.MessageRepository,
	presenceRepo *redisRepo.PresenceRepository,
	redisClient *redis.Client,
) *Service {
	return &Service{
		messageRepo:  messageRepo,
		presenceRepo: presenceRepo,
		redisClient:  redisClient,
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
		CreatedAt:      time.Now(),
		Bucket:         domain.CalculateBucket(time.Now()),
	}
	
	// Save to Cassandra
	if err := s.messageRepo.Save(message); err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}
	
	// Publish to Redis Pub/Sub for real-time delivery
	channel := fmt.Sprintf("chat:%s", input.ConversationID)
	messageJSON, err := json.Marshal(message)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to marshal message for pub/sub: %v\n", err)
	} else {
		if err := s.redisClient.Publish(ctx, channel, messageJSON).Err(); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to publish message to Redis: %v\n", err)
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
		CreatedAt:      message.CreatedAt,
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
	// Get current bucket
	currentBucket := domain.CalculateBucket(time.Now())
	
	// Fetch messages from Cassandra
	messages, nextPageState, err := s.messageRepo.GetByConversation(
		input.ConversationID,
		currentBucket,
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
			CreatedAt:      msg.CreatedAt,
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
	if online {
		return s.presenceRepo.SetUserOnline(ctx, userID)
	}
	return s.presenceRepo.SetUserOffline(ctx, userID)
}

// RefreshPresence keeps user status alive (heartbeat)
func (s *Service) RefreshPresence(ctx context.Context, userID uuid.UUID) error {
	return s.presenceRepo.RefreshPresence(ctx, userID)
}

package chat

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"secureconnect-backend/internal/domain"
)

// Mocks
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) Save(message *domain.Message) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockMessageRepository) GetByConversation(conversationID uuid.UUID, limit int, pageState []byte) ([]*domain.Message, []byte, error) {
	args := m.Called(conversationID, limit, pageState)
	return args.Get(0).([]*domain.Message), args.Get(1).([]byte), args.Error(2)
}

type MockPresenceRepository struct {
	mock.Mock
}

func (m *MockPresenceRepository) SetUserOnline(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockPresenceRepository) SetUserOffline(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockPresenceRepository) RefreshPresence(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockPresenceRepository) IsUserOnline(ctx context.Context, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, channel string, message interface{}) error {
	args := m.Called(ctx, channel, message)
	return args.Error(0)
}

type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) CreateMessageNotification(ctx context.Context, userID uuid.UUID, senderName string, conversationID uuid.UUID) error {
	args := m.Called(ctx, userID, senderName, conversationID)
	return args.Error(0)
}

type MockConversationRepository struct {
	mock.Mock
}

func (m *MockConversationRepository) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	args := m.Called(ctx, conversationID)
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func TestSendMessage(t *testing.T) {
	mockMsgRepo := new(MockMessageRepository)
	mockPresenceRepo := new(MockPresenceRepository)
	mockPublisher := new(MockPublisher)
	mockNotificationSvc := new(MockNotificationService)
	mockConversationRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)

	service := NewService(mockMsgRepo, mockPresenceRepo, mockPublisher, mockNotificationSvc, mockConversationRepo, mockUserRepo)

	conversationID := uuid.New()
	senderID := uuid.New()
	input := &SendMessageInput{
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        "Hello World",
		MessageType:    "text",
	}

	ctx := context.Background()

	// Expectations
	mockMsgRepo.On("Save", mock.AnythingOfType("*domain.Message")).Return(nil)
	mockPublisher.On("Publish", ctx, "chat:"+conversationID.String(), mock.Anything).Return(nil)

	// Execute
	output, err := service.SendMessage(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, input.Content, output.Message.Content)
	assert.Equal(t, input.ConversationID, output.Message.ConversationID)

	mockMsgRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestGetMessages(t *testing.T) {
	mockMsgRepo := new(MockMessageRepository)
	mockPresenceRepo := new(MockPresenceRepository)
	mockPublisher := new(MockPublisher)
	mockNotificationSvc := new(MockNotificationService)
	mockConversationRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)

	service := NewService(mockMsgRepo, mockPresenceRepo, mockPublisher, mockNotificationSvc, mockConversationRepo, mockUserRepo)

	conversationID := uuid.New()
	input := &GetMessagesInput{
		ConversationID: conversationID,
		Limit:          20,
	}

	mockMessages := []*domain.Message{
		{
			MessageID:      uuid.New(),
			ConversationID: conversationID,
			Content:        "Msg 1",
			SentAt:         time.Now(),
		},
	}

	ctx := context.Background()

	// Expectations
	mockMsgRepo.On("GetByConversation", conversationID, 20, []byte(nil)).Return(mockMessages, []byte(nil), nil)

	// Execute
	output, err := service.GetMessages(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Len(t, output.Messages, 1)
	assert.Equal(t, "Msg 1", output.Messages[0].Content)

	mockMsgRepo.AssertExpectations(t)
}

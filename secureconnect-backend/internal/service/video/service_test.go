package video

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"secureconnect-backend/internal/domain"
)

// MockCallRepository is a mock implementation of CallRepository
type MockCallRepository struct {
	mock.Mock
}

// MockUserRepository is a mock implementation of UserRepository
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

func (m *MockCallRepository) Create(ctx context.Context, call *domain.Call) error {
	args := m.Called(ctx, call)
	return args.Error(0)
}

func (m *MockCallRepository) GetByID(ctx context.Context, callID uuid.UUID) (*domain.Call, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Call), args.Error(1)
}

func (m *MockCallRepository) AddParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	args := m.Called(ctx, callID, userID)
	return args.Error(0)
}

func (m *MockCallRepository) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	args := m.Called(ctx, callID, userID)
	return args.Error(0)
}

func (m *MockCallRepository) UpdateStatus(ctx context.Context, callID uuid.UUID, status string) error {
	args := m.Called(ctx, callID, status)
	return args.Error(0)
}

func (m *MockCallRepository) EndCall(ctx context.Context, callID uuid.UUID) error {
	args := m.Called(ctx, callID)
	return args.Error(0)
}

func (m *MockCallRepository) GetParticipants(ctx context.Context, callID uuid.UUID) ([]*domain.CallParticipant, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CallParticipant), args.Error(1)
}

func (m *MockCallRepository) GetUserCalls(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Call, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Call), args.Error(1)
}

// MockConversationRepository is a mock implementation of ConversationRepository
type MockConversationRepository struct {
	mock.Mock
}

func (m *MockConversationRepository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, conversationID, userID)
	return args.Bool(0), args.Error(1)
}

// TestInitiateCall tests the InitiateCall method
func TestInitiateCall(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	conversationID := uuid.New()
	callerID := uuid.New()
	calleeID := uuid.New()

	input := &InitiateCallInput{
		CallType:       CallTypeVideo,
		ConversationID: conversationID,
		CallerID:       callerID,
		CalleeIDs:      []uuid.UUID{calleeID},
	}

	// Setup expectations
	mockCallRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Call")).Return(nil)
	mockCallRepo.On("AddParticipant", mock.Anything, mock.AnythingOfType("uuid.UUID"), callerID).Return(nil)

	// Execute
	output, err := service.InitiateCall(context.Background(), input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, CallTypeVideo, output.CallType)
	assert.Equal(t, "ringing", output.Status)
	assert.Equal(t, conversationID, output.ConversationID)

	mockCallRepo.AssertExpectations(t)
}

// TestEndCall tests the EndCall method
func TestEndCall(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	callID := uuid.New()
	userID := uuid.New()

	// Setup expectations
	mockCallRepo.On("EndCall", mock.Anything, callID).Return(nil)
	mockCallRepo.On("RemoveParticipant", mock.Anything, callID, userID).Return(nil)

	// Execute
	err := service.EndCall(context.Background(), callID, userID)

	// Assert
	assert.NoError(t, err)
	mockCallRepo.AssertExpectations(t)
}

// TestJoinCall tests the JoinCall method
func TestJoinCall(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	callID := uuid.New()
	userID := uuid.New()

	existingCall := &domain.Call{
		CallID: callID,
		Status: "ringing",
	}

	// Setup expectations
	mockCallRepo.On("GetByID", mock.Anything, callID).Return(existingCall, nil)
	mockConvRepo.On("IsParticipant", mock.Anything, mock.AnythingOfType("uuid.UUID"), userID).Return(true, nil)
	mockCallRepo.On("AddParticipant", mock.Anything, callID, userID).Return(nil)
	mockCallRepo.On("UpdateStatus", mock.Anything, callID, "active").Return(nil)

	// Execute
	err := service.JoinCall(context.Background(), callID, userID)

	// Assert
	assert.NoError(t, err)
	mockCallRepo.AssertExpectations(t)
	mockConvRepo.AssertExpectations(t)
}

// TestJoinCall_CallEnded tests joining an ended call
func TestJoinCall_CallEnded(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	callID := uuid.New()
	userID := uuid.New()

	endedCall := &domain.Call{
		CallID: callID,
		Status: "ended",
	}

	// Setup expectations
	mockCallRepo.On("GetByID", mock.Anything, callID).Return(endedCall, nil)

	// Execute
	err := service.JoinCall(context.Background(), callID, userID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "call has ended")
	mockCallRepo.AssertExpectations(t)
}

// TestLeaveCall tests the LeaveCall method
func TestLeaveCall(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	callID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()

	now := time.Now()
	participants := []*domain.CallParticipant{
		{
			CallID:   callID,
			UserID:   userID,
			JoinedAt: now,
			LeftAt:   &now, // This user is leaving
		},
		{
			CallID:   callID,
			UserID:   otherUserID,
			JoinedAt: now,
			LeftAt:   nil, // This user is still active
		},
	}

	// Setup expectations
	mockCallRepo.On("RemoveParticipant", mock.Anything, callID, userID).Return(nil)
	mockCallRepo.On("GetParticipants", mock.Anything, callID).Return(participants, nil)

	// Execute
	err := service.LeaveCall(context.Background(), callID, userID)

	// Assert
	assert.NoError(t, err)
	mockCallRepo.AssertExpectations(t)
	mockCallRepo.AssertNotCalled(t, "EndCall") // Call should not end, other user still active
}

// TestLeaveCall_LastParticipant tests leaving when you're the last participant
func TestLeaveCall_LastParticipant(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	callID := uuid.New()
	userID := uuid.New()

	now := time.Now()
	participants := []*domain.CallParticipant{
		{
			CallID:   callID,
			UserID:   userID,
			JoinedAt: now,
			LeftAt:   &now, // Last user leaving
		},
	}

	// Setup expectations
	mockCallRepo.On("RemoveParticipant", mock.Anything, callID, userID).Return(nil)
	mockCallRepo.On("GetParticipants", mock.Anything, callID).Return(participants, nil)
	mockCallRepo.On("EndCall", mock.Anything, callID).Return(nil)

	// Execute
	err := service.LeaveCall(context.Background(), callID, userID)

	// Assert
	assert.NoError(t, err)
	mockCallRepo.AssertExpectations(t)
}

// TestGetUserCallHistory tests retrieving call history
func TestGetUserCallHistory(t *testing.T) {
	mockCallRepo := new(MockCallRepository)
	mockConvRepo := new(MockConversationRepository)
	mockUserRepo := new(MockUserRepository)
	service := NewService(mockCallRepo, mockConvRepo, mockUserRepo, nil)

	userID := uuid.New()
	calls := []*domain.Call{
		{
			CallID:   uuid.New(),
			CallerID: userID,
			CallType: "video",
			Status:   "ended",
		},
	}

	// Setup expectations
	mockCallRepo.On("GetUserCalls", mock.Anything, userID, 20, 0).Return(calls, nil)

	// Execute
	result, err := service.GetUserCallHistory(context.Background(), userID, 0, 0)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockCallRepo.AssertExpectations(t)
}

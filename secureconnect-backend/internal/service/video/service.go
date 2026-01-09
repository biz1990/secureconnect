package video

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	
	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
)

// Service handles video call business logic
type Service struct {
	callRepo *cockroach.CallRepository
	// TODO: Add Pion WebRTC SFU in future
	// sfu *webrtc.SFU
}

// NewService creates a new video service
func NewService(callRepo *cockroach.CallRepository) *Service {
	return &Service{
		callRepo: callRepo,
	}
}

// CallType represents type of call
type CallType string

const (
	CallTypeAudio CallType = "audio"
	CallTypeVideo CallType = "video"
)

// InitiateCallInput contains call initiation data
type InitiateCallInput struct {
	CallType       CallType
	ConversationID uuid.UUID
	CallerID       uuid.UUID
	CalleeIDs      []uuid.UUID
}

// InitiateCallOutput contains call session info
type InitiateCallOutput struct {
	CallID         uuid.UUID
	ConversationID uuid.UUID
	CallType       CallType
	Status         string
	CreatedAt      time.Time
}

// InitiateCall starts a new call session
func (s *Service) InitiateCall(ctx context.Context, input *InitiateCallInput) (*InitiateCallOutput, error) {
	// Generate call ID
	callID := uuid.New()
	
	// Create call record in database
	call := &domain.Call{
		CallID:         callID,
		ConversationID: input.ConversationID,
		CallerID:       input.CallerID,
		CallType:       string(input.CallType),
		Status:         "ringing",
		StartedAt:      time.Now(),
	}
	
	if err := s.callRepo.Create(ctx, call); err != nil {
		return nil, fmt.Errorf("failed to create call record: %w", err)
	}
	
	// Add caller as first participant
	if err := s.callRepo.AddParticipant(ctx, callID, input.CallerID); err != nil {
		return nil, fmt.Errorf("failed to add caller: %w", err)
	}
	
	// TODO: Send push notifications to callees
	// TODO: Initialize SFU room
	
	return &InitiateCallOutput{
		CallID:         callID,
		ConversationID: input.ConversationID,
		CallType:       input.CallType,
		Status:         "ringing",
		CreatedAt:      time.Now(),
	}, nil
}

// EndCall terminates a call session
func (s *Service) EndCall(ctx context.Context, callID uuid.UUID, userID uuid.UUID) error {
	// Update call status to "ended" and calculate duration
	if err := s.callRepo.EndCall(ctx, callID); err != nil {
		return fmt.Errorf("failed to end call: %w", err)
	}
	
	// Mark user as left
	if err := s.callRepo.RemoveParticipant(ctx, callID, userID); err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}
	
	// TODO: Clean up SFU resources
	// TODO: Stop call recording if enabled
	
	return nil
}

// JoinCall allows a user to join an ongoing call
func (s *Service) JoinCall(ctx context.Context, callID uuid.UUID, userID uuid.UUID) error {
	// Verify call exists
	call, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return fmt.Errorf("call not found: %w", err)
	}
	
	// Check if call is still active
	if call.Status == "ended" {
		return fmt.Errorf("call has ended")
	}
	
	// Add user to participants
	if err := s.callRepo.AddParticipant(ctx, callID, userID); err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}
	
	// Update call status to active if it was ringing
	if call.Status == "ringing" {
		if err := s.callRepo.UpdateStatus(ctx, callID, "active"); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}
	
	// TODO: Add user to SFU room
	
	return nil
}

// LeaveCall removes a user from a call
func (s *Service) LeaveCall(ctx context.Context, callID uuid.UUID, userID uuid.UUID) error {
	// Remove from participants
	if err := s.callRepo.RemoveParticipant(ctx, callID, userID); err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}
	
	// Check if any participants left
	participants, err := s.callRepo.GetParticipants(ctx, callID)
	if err != nil {
		return fmt.Errorf("failed to get participants: %w", err)
	}
	
	// Count active participants
	activeCount := 0
	for _, p := range participants {
		if p.LeftAt == nil {
			activeCount++
		}
	}
	
	// End call if no participants left
	if activeCount == 0 {
		if err := s.callRepo.EndCall(ctx, callID); err != nil {
			return fmt.Errorf("failed to end call: %w", err)
		}
	}
	
	// TODO: Remove from SFU
	
	return nil
}

// GetCallStatus retrieves current call information
func (s *Service) GetCallStatus(ctx context.Context, callID uuid.UUID) (*domain.Call, error) {
	return s.callRepo.GetByID(ctx, callID)
}

// GetUserCallHistory retrieves call history for a user
func (s *Service) GetUserCallHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Call, error) {
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	
	return s.callRepo.GetUserCalls(ctx, userID, limit, offset)
}

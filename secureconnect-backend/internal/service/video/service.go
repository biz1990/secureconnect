package video

import (
	"context"
	"fmt"
	"time"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/push"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CallRepository defines interface for call persistence
type CallRepository interface {
	Create(ctx context.Context, call *domain.Call) error
	GetByID(ctx context.Context, callID uuid.UUID) (*domain.Call, error)
	AddParticipant(ctx context.Context, callID, userID uuid.UUID) error
	RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error
	UpdateStatus(ctx context.Context, callID uuid.UUID, status string) error
	EndCall(ctx context.Context, callID uuid.UUID) error
	GetParticipants(ctx context.Context, callID uuid.UUID) ([]*domain.CallParticipant, error)
	GetUserCalls(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Call, error)
}

// ConversationRepository defines interface for conversation membership verification
type ConversationRepository interface {
	IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error)
}

// UserRepository defines interface for getting user information
type UserRepository interface {
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

// Service handles video call business logic
type Service struct {
	callRepo         CallRepository
	conversationRepo ConversationRepository
	userRepo         UserRepository
	pushService      *push.Service
	// TODO: Add Pion WebRTC SFU in future
	// sfu *webrtc.SFU
}

// NewService creates a new video service
func NewService(callRepo CallRepository, conversationRepo ConversationRepository, userRepo UserRepository, pushService *push.Service) *Service {
	return &Service{
		callRepo:         callRepo,
		conversationRepo: conversationRepo,
		userRepo:         userRepo,
		pushService:      pushService,
	}
}

// CallType represents type of call
type CallType string

const (
	CallTypeAudio CallType = constants.CallTypeAudio
	CallTypeVideo CallType = constants.CallTypeVideo
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
		Status:         constants.CallStatusRinging,
		StartedAt:      time.Now(),
	}

	if err := s.callRepo.Create(ctx, call); err != nil {
		return nil, fmt.Errorf("failed to create call record: %w", err)
	}

	// Add caller as first participant
	if err := s.callRepo.AddParticipant(ctx, callID, input.CallerID); err != nil {
		return nil, fmt.Errorf("failed to add caller: %w", err)
	}

	// Get caller information for push notification
	caller, err := s.userRepo.GetByID(ctx, input.CallerID)
	if err != nil {
		logger.Warn("Failed to get caller information for push notification",
			zap.String("caller_id", input.CallerID.String()),
			zap.Error(err))
	} else {
		// Send push notifications to callees
		pushData := &push.CallNotificationData{
			CallID:         callID,
			ConversationID: input.ConversationID,
			CallerID:       input.CallerID,
			CallerName:     caller.Username,
			CallType:       string(input.CallType),
			CallStatus:     "ringing",
			Timestamp:      time.Now().Unix(),
		}

		if err := s.pushService.SendCallNotification(ctx, pushData, input.CalleeIDs); err != nil {
			logger.Warn("Failed to send call notification",
				zap.String("call_id", callID.String()),
				zap.Error(err))
		}
	}

	// TODO: Initialize SFU room

	return &InitiateCallOutput{
		CallID:         callID,
		ConversationID: input.ConversationID,
		CallType:       input.CallType,
		Status:         constants.CallStatusRinging,
		CreatedAt:      time.Now(),
	}, nil
}

// EndCall terminates a call session
func (s *Service) EndCall(ctx context.Context, callID uuid.UUID, userID uuid.UUID) error {
	// Get call information before ending
	call, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return fmt.Errorf("failed to get call: %w", err)
	}

	// Get user who ended the call
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Warn("Failed to get user who ended the call",
			zap.String("user_id", userID.String()),
			zap.Error(err))
	}

	// Update call status to "ended" and calculate duration
	if err := s.callRepo.EndCall(ctx, callID); err != nil {
		return fmt.Errorf("failed to end call: %w", err)
	}

	// Mark user as left
	if err := s.callRepo.RemoveParticipant(ctx, callID, userID); err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	// Send call ended notification to all participants
	if user != nil {
		participants, err := s.callRepo.GetParticipants(ctx, callID)
		if err != nil {
			logger.Warn("Failed to get participants for call ended notification",
				zap.String("call_id", callID.String()),
				zap.Error(err))
		} else {
			// Collect participant IDs
			var participantIDs []uuid.UUID
			for _, p := range participants {
				participantIDs = append(participantIDs, p.UserID)
			}

			// Calculate duration
			duration := int64(0)
			if !call.StartedAt.IsZero() {
				duration = int64(time.Since(call.StartedAt).Seconds())
			}

			// Send call ended notification
			if err := s.pushService.SendCallEndedNotification(ctx, callID, call.ConversationID, user.Username, duration, participantIDs); err != nil {
				logger.Warn("Failed to send call ended notification",
					zap.String("call_id", callID.String()),
					zap.Error(err))
			}
		}
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
	if call.Status == constants.CallStatusEnded {
		return fmt.Errorf("call has ended")
	}

	// Verify user is a participant in the conversation
	isParticipant, err := s.conversationRepo.IsParticipant(ctx, call.ConversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to verify conversation membership: %w", err)
	}
	if !isParticipant {
		return fmt.Errorf("user is not a participant in this conversation")
	}

	// Add user to participants
	if err := s.callRepo.AddParticipant(ctx, callID, userID); err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}

	// Update call status to active if it was ringing
	if call.Status == constants.CallStatusRinging {
		if err := s.callRepo.UpdateStatus(ctx, callID, constants.CallStatusActive); err != nil {
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
		// Get call information before ending
		call, err := s.callRepo.GetByID(ctx, callID)
		if err != nil {
			logger.Warn("Failed to get call for missed call notification",
				zap.String("call_id", callID.String()),
				zap.Error(err))
		} else {
			// Get caller information
			caller, err := s.userRepo.GetByID(ctx, call.CallerID)
			if err != nil {
				logger.Warn("Failed to get caller for missed call notification",
					zap.String("caller_id", call.CallerID.String()),
					zap.Error(err))
			} else {
				// Send missed call notification to participants who never joined
				var missedCalleeIDs []uuid.UUID
				for _, p := range participants {
					// If participant left at same time they joined (never really joined)
					if p.LeftAt != nil && p.LeftAt.Sub(p.JoinedAt) < time.Second {
						missedCalleeIDs = append(missedCalleeIDs, p.UserID)
					}
				}

				// Send missed call notifications
				if len(missedCalleeIDs) > 0 {
					if err := s.pushService.SendMissedCallNotification(ctx, callID, call.ConversationID, call.CallerID, caller.Username, missedCalleeIDs); err != nil {
						logger.Warn("Failed to send missed call notification",
							zap.String("call_id", callID.String()),
							zap.Error(err))
					}
				}
			}
		}

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
		limit = constants.DefaultPageSize
	}
	if limit > constants.MaxPageSize {
		limit = constants.MaxPageSize
	}

	return s.callRepo.GetUserCalls(ctx, userID, limit, offset)
}

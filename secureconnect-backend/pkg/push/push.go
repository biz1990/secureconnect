package push

import (
	"context"
	"encoding/json"
	"fmt"

	"secureconnect-backend/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Provider defines interface for sending push notifications
type Provider interface {
	Send(ctx context.Context, notification *Notification, tokens []string) (*SendResult, error)
	SendToUser(ctx context.Context, notification *Notification, userID uuid.UUID) (*SendResult, error)
}

// SendResult contains the result of a push notification send operation
type SendResult struct {
	SuccessCount  int
	FailureCount  int
	InvalidTokens []string
	Errors        []error
}

// Notification represents a push notification
type Notification struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Data        map[string]string `json:"data,omitempty"`
	Priority    string            `json:"priority,omitempty"` // high, normal, low
	Sound       string            `json:"sound,omitempty"`
	Badge       *int              `json:"badge,omitempty"`
	Category    string            `json:"category,omitempty"`
	ClickAction string            `json:"click_action,omitempty"`
}

// CallNotificationData contains data for call-related notifications
type CallNotificationData struct {
	CallID         uuid.UUID `json:"call_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	CallerID       uuid.UUID `json:"caller_id"`
	CallerName     string    `json:"caller_name"`
	CallType       string    `json:"call_type"`
	CallStatus     string    `json:"call_status"`
	Timestamp      int64     `json:"timestamp"`
}

// TokenType represents the type of push notification token
type TokenType string

const (
	TokenTypeFCM  TokenType = "fcm"  // Firebase Cloud Messaging
	TokenTypeAPNs TokenType = "apns" // Apple Push Notification Service
	TokenTypeWeb  TokenType = "web"  // Web Push
)

// Token represents a push notification token for a user
type Token struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"token"`
	Type      TokenType `json:"type"`
	DeviceID  string    `json:"device_id,omitempty"`
	Platform  string    `json:"platform,omitempty"` // ios, android, web
	Active    bool      `json:"active"`
	CreatedAt int64     `json:"created_at"`
	UpdatedAt int64     `json:"updated_at"`
}

// TokenRepository defines interface for storing and retrieving push tokens
type TokenRepository interface {
	Store(ctx context.Context, token *Token) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*Token, error)
	GetByToken(ctx context.Context, token string) (*Token, error)
	Update(ctx context.Context, token *Token) error
	Delete(ctx context.Context, tokenID uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	MarkInactive(ctx context.Context, tokenID uuid.UUID) error
	GetActiveTokensCount(ctx context.Context, userID uuid.UUID) (int, error)
}

// Service handles push notification operations
type Service struct {
	provider Provider
	repo     TokenRepository
}

// NewService creates a new push notification service
func NewService(provider Provider, repo TokenRepository) *Service {
	return &Service{
		provider: provider,
		repo:     repo,
	}
}

// RegisterToken registers a new push notification token for a user
func (s *Service) RegisterToken(ctx context.Context, token *Token) error {
	// Check if token already exists
	existing, err := s.repo.GetByToken(ctx, token.Token)
	if err == nil && existing != nil {
		// Update existing token
		existing.Active = true
		existing.UpdatedAt = token.UpdatedAt
		existing.DeviceID = token.DeviceID
		existing.Platform = token.Platform
		return s.repo.Update(ctx, existing)
	}

	// Store new token
	return s.repo.Store(ctx, token)
}

// UnregisterToken removes a push notification token
func (s *Service) UnregisterToken(ctx context.Context, tokenID uuid.UUID) error {
	return s.repo.Delete(ctx, tokenID)
}

// UnregisterAllTokens removes all tokens for a user
func (s *Service) UnregisterAllTokens(ctx context.Context, userID uuid.UUID) error {
	return s.repo.DeleteByUserID(ctx, userID)
}

// SendCallNotification sends a push notification for a call
func (s *Service) SendCallNotification(ctx context.Context, data *CallNotificationData, calleeIDs []uuid.UUID) error {
	// Create notification payload
	notification := &Notification{
		Title:    "Incoming Call",
		Body:     fmt.Sprintf("%s is calling you", data.CallerName),
		Priority: "high",
		Sound:    "default",
		Category: "INCOMING_CALL",
		Data: map[string]string{
			"type":            "call",
			"call_id":         data.CallID.String(),
			"conversation_id": data.ConversationID.String(),
			"caller_id":       data.CallerID.String(),
			"caller_name":     data.CallerName,
			"call_type":       data.CallType,
			"call_status":     data.CallStatus,
			"timestamp":       fmt.Sprintf("%d", data.Timestamp),
		},
	}

	// Collect all tokens for callees
	var allTokens []string
	for _, userID := range calleeIDs {
		tokens, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			logger.Warn("Failed to get push tokens for user",
				zap.String("user_id", userID.String()),
				zap.Error(err))
			continue
		}

		for _, token := range tokens {
			if token.Active {
				allTokens = append(allTokens, token.Token)
			}
		}
	}

	if len(allTokens) == 0 {
		logger.Info("No active push tokens found for callees",
			zap.Int("callee_count", len(calleeIDs)))
		return nil
	}

	// Send push notification
	result, err := s.provider.Send(ctx, notification, allTokens)
	if err != nil {
		logger.Error("Failed to send call notification",
			zap.String("call_id", data.CallID.String()),
			zap.Int("token_count", len(allTokens)),
			zap.Error(err))
		return fmt.Errorf("failed to send call notification: %w", err)
	}

	// Log results
	logger.Info("Call notification sent",
		zap.String("call_id", data.CallID.String()),
		zap.Int("success_count", result.SuccessCount),
		zap.Int("failure_count", result.FailureCount),
		zap.Int("invalid_tokens", len(result.InvalidTokens)))

	// Handle invalid tokens
	if len(result.InvalidTokens) > 0 {
		s.handleInvalidTokens(ctx, result.InvalidTokens)
	}

	return nil
}

// SendCallEndedNotification sends a notification when a call ends
func (s *Service) SendCallEndedNotification(ctx context.Context, callID uuid.UUID, conversationID uuid.UUID, endedBy string, duration int64, participantIDs []uuid.UUID) error {
	notification := &Notification{
		Title:    "Call Ended",
		Body:     fmt.Sprintf("Call ended by %s. Duration: %s", endedBy, formatDuration(duration)),
		Priority: "normal",
		Sound:    "default",
		Data: map[string]string{
			"type":            "call_ended",
			"call_id":         callID.String(),
			"conversation_id": conversationID.String(),
			"ended_by":        endedBy,
			"duration":        fmt.Sprintf("%d", duration),
		},
	}

	// Collect all tokens for participants
	var allTokens []string
	for _, userID := range participantIDs {
		tokens, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			logger.Warn("Failed to get push tokens for user",
				zap.String("user_id", userID.String()),
				zap.Error(err))
			continue
		}

		for _, token := range tokens {
			if token.Active {
				allTokens = append(allTokens, token.Token)
			}
		}
	}

	if len(allTokens) == 0 {
		return nil
	}

	result, err := s.provider.Send(ctx, notification, allTokens)
	if err != nil {
		logger.Error("Failed to send call ended notification",
			zap.String("call_id", callID.String()),
			zap.Error(err))
		return err
	}

	logger.Info("Call ended notification sent",
		zap.String("call_id", callID.String()),
		zap.Int("success_count", result.SuccessCount),
		zap.Int("failure_count", result.FailureCount))

	if len(result.InvalidTokens) > 0 {
		s.handleInvalidTokens(ctx, result.InvalidTokens)
	}

	return nil
}

// SendMissedCallNotification sends a notification for missed calls
func (s *Service) SendMissedCallNotification(ctx context.Context, callID uuid.UUID, conversationID uuid.UUID, callerID uuid.UUID, callerName string, calleeIDs []uuid.UUID) error {
	notification := &Notification{
		Title:    "Missed Call",
		Body:     fmt.Sprintf("You missed a call from %s", callerName),
		Priority: "normal",
		Sound:    "default",
		Data: map[string]string{
			"type":            "missed_call",
			"call_id":         callID.String(),
			"conversation_id": conversationID.String(),
			"caller_id":       callerID.String(),
			"caller_name":     callerName,
		},
	}

	// Collect all tokens for callees
	var allTokens []string
	for _, userID := range calleeIDs {
		tokens, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			logger.Warn("Failed to get push tokens for user",
				zap.String("user_id", userID.String()),
				zap.Error(err))
			continue
		}

		for _, token := range tokens {
			if token.Active {
				allTokens = append(allTokens, token.Token)
			}
		}
	}

	if len(allTokens) == 0 {
		return nil
	}

	result, err := s.provider.Send(ctx, notification, allTokens)
	if err != nil {
		logger.Error("Failed to send missed call notification",
			zap.String("call_id", callID.String()),
			zap.Error(err))
		return err
	}

	logger.Info("Missed call notification sent",
		zap.String("call_id", callID.String()),
		zap.Int("success_count", result.SuccessCount),
		zap.Int("failure_count", result.FailureCount))

	if len(result.InvalidTokens) > 0 {
		s.handleInvalidTokens(ctx, result.InvalidTokens)
	}

	return nil
}

// SendCustomNotification sends a custom notification
func (s *Service) SendCustomNotification(ctx context.Context, notification *Notification, userIDs []uuid.UUID) error {
	// Collect all tokens for users
	var allTokens []string
	for _, userID := range userIDs {
		tokens, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			logger.Warn("Failed to get push tokens for user",
				zap.String("user_id", userID.String()),
				zap.Error(err))
			continue
		}

		for _, token := range tokens {
			if token.Active {
				allTokens = append(allTokens, token.Token)
			}
		}
	}

	if len(allTokens) == 0 {
		return nil
	}

	result, err := s.provider.Send(ctx, notification, allTokens)
	if err != nil {
		logger.Error("Failed to send custom notification",
			zap.Int("user_count", len(userIDs)),
			zap.Error(err))
		return err
	}

	logger.Info("Custom notification sent",
		zap.Int("user_count", len(userIDs)),
		zap.Int("success_count", result.SuccessCount),
		zap.Int("failure_count", result.FailureCount))

	if len(result.InvalidTokens) > 0 {
		s.handleInvalidTokens(ctx, result.InvalidTokens)
	}

	return nil
}

// handleInvalidTokens marks invalid tokens as inactive
func (s *Service) handleInvalidTokens(ctx context.Context, invalidTokens []string) {
	for _, tokenStr := range invalidTokens {
		token, err := s.repo.GetByToken(ctx, tokenStr)
		if err == nil && token != nil {
			if err := s.repo.MarkInactive(ctx, token.ID); err != nil {
				logger.Warn("Failed to mark token as inactive",
					zap.String("token_id", token.ID.String()),
					zap.Error(err))
			}
		}
	}
}

// GetTokenByValue retrieves a token by its value
func (s *Service) GetTokenByValue(ctx context.Context, tokenStr string) (*Token, error) {
	return s.repo.GetByToken(ctx, tokenStr)
}

// GetTokensByUserID retrieves all tokens for a user
func (s *Service) GetTokensByUserID(ctx context.Context, userID uuid.UUID) ([]*Token, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// GetActiveTokensCount returns the count of active tokens for a user
func (s *Service) GetActiveTokensCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.GetActiveTokensCount(ctx, userID)
}

// formatDuration formats duration in seconds to human-readable format
func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// MockProvider is a mock implementation for development/testing
type MockProvider struct {
	// For testing purposes
	NotificationsSent int
}

// Send implements Provider interface
func (m *MockProvider) Send(ctx context.Context, notification *Notification, tokens []string) (*SendResult, error) {
	m.NotificationsSent++

	logger.Debug("MockProvider: Sending notification",
		zap.String("title", notification.Title),
		zap.String("body", notification.Body),
		zap.Int("token_count", len(tokens)))

	// Return success for all tokens
	return &SendResult{
		SuccessCount:  len(tokens),
		FailureCount:  0,
		InvalidTokens: nil,
		Errors:        nil,
	}, nil
}

// SendToUser implements Provider interface
func (m *MockProvider) SendToUser(ctx context.Context, notification *Notification, userID uuid.UUID) (*SendResult, error) {
	m.NotificationsSent++

	logger.Debug("MockProvider: Sending notification to user",
		zap.String("title", notification.Title),
		zap.String("body", notification.Body),
		zap.String("user_id", userID.String()))

	return &SendResult{
		SuccessCount:  1,
		FailureCount:  0,
		InvalidTokens: nil,
		Errors:        nil,
	}, nil
}

// ToJSON converts notification to JSON
func (n *Notification) ToJSON() ([]byte, error) {
	return json.Marshal(n)
}

// FromJSON creates notification from JSON
func FromJSON(data []byte) (*Notification, error) {
	var notification Notification
	err := json.Unmarshal(data, &notification)
	return &notification, err
}

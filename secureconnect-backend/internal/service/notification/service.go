package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
)

// Service handles notification business logic
type Service struct {
	notificationRepo *cockroach.NotificationRepository
}

// NewService creates a new notification service
func NewService(notificationRepo *cockroach.NotificationRepository) *Service {
	return &Service{
		notificationRepo: notificationRepo,
	}
}

// CreateNotificationInput represents input for creating a notification
type CreateNotificationInput struct {
	UserID uuid.UUID
	Type   string
	Title  string
	Body   string
	Data   map[string]interface{}
}

// Create creates a new notification
func (s *Service) Create(ctx context.Context, input *CreateNotificationInput) (*domain.Notification, error) {
	create := &domain.NotificationCreate{
		UserID: input.UserID,
		Type:   input.Type,
		Title:  input.Title,
		Body:   input.Body,
		Data:   input.Data,
	}

	notification, err := s.notificationRepo.Create(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	return notification, nil
}

// GetNotifications retrieves notifications for a user
func (s *Service) GetNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) (*domain.NotificationListResponse, error) {
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	notifications, totalCount, err := s.notificationRepo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}

	unreadCount, err := s.notificationRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread count: %w", err)
	}

	hasMore := (offset + len(notifications)) < totalCount

	return &domain.NotificationListResponse{
		Notifications: notifications,
		UnreadCount:   unreadCount,
		TotalCount:    totalCount,
		HasMore:       hasMore,
	}, nil
}

// MarkAsRead marks a notification as read
func (s *Service) MarkAsRead(ctx context.Context, notificationID uuid.UUID, userID uuid.UUID) error {
	err := s.notificationRepo.MarkAsRead(ctx, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (s *Service) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	err := s.notificationRepo.MarkAllAsRead(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}
	return nil
}

// DeleteNotification deletes a notification
func (s *Service) DeleteNotification(ctx context.Context, notificationID uuid.UUID, userID uuid.UUID) error {
	err := s.notificationRepo.Delete(ctx, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}
	return nil
}

// GetPreferences retrieves notification preferences for a user
func (s *Service) GetPreferences(ctx context.Context, userID uuid.UUID) (*domain.NotificationPreference, error) {
	pref, err := s.notificationRepo.GetPreference(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification preferences: %w", err)
	}
	return pref, nil
}

// UpdatePreferences updates notification preferences for a user
func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, update *domain.NotificationPreferenceUpdate) (*domain.NotificationPreference, error) {
	// Get current preferences
	current, err := s.notificationRepo.GetPreference(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current preferences: %w", err)
	}

	// Apply updates
	if update.EmailEnabled != nil {
		current.EmailEnabled = *update.EmailEnabled
	}
	if update.PushEnabled != nil {
		current.PushEnabled = *update.PushEnabled
	}
	if update.MessageEnabled != nil {
		current.MessageEnabled = *update.MessageEnabled
	}
	if update.CallEnabled != nil {
		current.CallEnabled = *update.CallEnabled
	}
	if update.FriendRequestEnabled != nil {
		current.FriendRequestEnabled = *update.FriendRequestEnabled
	}
	if update.SystemEnabled != nil {
		current.SystemEnabled = *update.SystemEnabled
	}

	// Save updated preferences
	err = s.notificationRepo.UpdatePreference(ctx, current)
	if err != nil {
		return nil, fmt.Errorf("failed to update preferences: %w", err)
	}

	return current, nil
}

// GetUnreadCount returns the count of unread notifications
func (s *Service) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := s.notificationRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// CreateMessageNotification creates a notification for a new message
func (s *Service) CreateMessageNotification(ctx context.Context, userID uuid.UUID, senderName string, conversationID uuid.UUID) error {
	input := &CreateNotificationInput{
		UserID: userID,
		Type:   "message",
		Title:  "New Message",
		Body:   fmt.Sprintf("You received a new message from %s", senderName),
		Data: map[string]interface{}{
			"conversation_id": conversationID,
			"sender_name":     senderName,
		},
	}
	_, err := s.Create(ctx, input)
	return err
}

// CreateCallNotification creates a notification for an incoming call
func (s *Service) CreateCallNotification(ctx context.Context, userID uuid.UUID, callerName string, callID uuid.UUID) error {
	input := &CreateNotificationInput{
		UserID: userID,
		Type:   "call",
		Title:  "Incoming Call",
		Body:   fmt.Sprintf("%s is calling you", callerName),
		Data: map[string]interface{}{
			"call_id":     callID,
			"caller_name": callerName,
		},
	}
	_, err := s.Create(ctx, input)
	return err
}

// CreateFriendRequestNotification creates a notification for a friend request
func (s *Service) CreateFriendRequestNotification(ctx context.Context, userID uuid.UUID, senderName string, senderID uuid.UUID) error {
	input := &CreateNotificationInput{
		UserID: userID,
		Type:   "friend_request",
		Title:  "New Friend Request",
		Body:   fmt.Sprintf("%s sent you a friend request", senderName),
		Data: map[string]interface{}{
			"sender_id":   senderID,
			"sender_name": senderName,
		},
	}
	_, err := s.Create(ctx, input)
	return err
}

// CreateSystemNotification creates a system notification
func (s *Service) CreateSystemNotification(ctx context.Context, userID uuid.UUID, title, body string, data map[string]interface{}) error {
	input := &CreateNotificationInput{
		UserID: userID,
		Type:   "system",
		Title:  title,
		Body:   body,
		Data:   data,
	}
	_, err := s.Create(ctx, input)
	return err
}

// ProcessUnpushedNotifications retrieves and marks notifications as pushed
// This is meant to be called by a background worker for push notifications
func (s *Service) ProcessUnpushedNotifications(ctx context.Context, limit int) ([]domain.Notification, error) {
	notifications, err := s.notificationRepo.GetUnpushed(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unpushed notifications: %w", err)
	}

	// Mark them as pushed
	for _, n := range notifications {
		err := s.notificationRepo.MarkAsPushed(ctx, n.NotificationID)
		if err != nil {
			// Log error but continue processing others
			continue
		}
	}

	return notifications, nil
}

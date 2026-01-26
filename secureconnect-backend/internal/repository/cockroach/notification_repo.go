package cockroach

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"secureconnect-backend/internal/domain"
)

// NotificationRepository handles notification data operations
type NotificationRepository struct {
	db *pgxpool.Pool
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create creates a new notification
func (r *NotificationRepository) Create(ctx context.Context, notification *domain.NotificationCreate) (*domain.Notification, error) {
	query := `
		INSERT INTO notifications (user_id, type, title, body, data, is_read, is_pushed, created_at)
		VALUES ($1, $2, $3, $4, $5, false, false, NOW())
		RETURNING notification_id, user_id, type, title, body, data, is_read, is_pushed, created_at, read_at
	`

	var n domain.Notification
	err := r.db.QueryRow(ctx, query,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Body,
		notification.Data,
	).Scan(
		&n.NotificationID,
		&n.UserID,
		&n.Type,
		&n.Title,
		&n.Body,
		&n.Data,
		&n.IsRead,
		&n.IsPushed,
		&n.CreatedAt,
		&n.ReadAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	return &n, nil
}

// GetByUserID retrieves notifications for a user with pagination
func (r *NotificationRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, int, error) {
	// Get notifications
	query := `
		SELECT notification_id, user_id, type, title, body, data, is_read, is_pushed, created_at, read_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []domain.Notification
	for rows.Next() {
		var n domain.Notification
		err := rows.Scan(
			&n.NotificationID,
			&n.UserID,
			&n.Type,
			&n.Title,
			&n.Body,
			&n.Data,
			&n.IsRead,
			&n.IsPushed,
			&n.CreatedAt,
			&n.ReadAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	var totalCount int
	err = r.db.QueryRow(ctx, countQuery, userID).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	return notifications, totalCount, nil
}

// GetUnreadCount returns the count of unread notifications for a user
func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`
	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// MarkAsRead marks a notification as read
func (r *NotificationRepository) MarkAsRead(ctx context.Context, notificationID uuid.UUID, userID uuid.UUID) error {
	query := `
		UPDATE notifications
		SET is_read = true, read_at = NOW()
		WHERE notification_id = $1 AND user_id = $2
	`
	result, err := r.db.Exec(ctx, query, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE notifications
		SET is_read = true, read_at = NOW()
		WHERE user_id = $1 AND is_read = false
	`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}
	return nil
}

// Delete deletes a notification
func (r *NotificationRepository) Delete(ctx context.Context, notificationID uuid.UUID, userID uuid.UUID) error {
	query := `DELETE FROM notifications WHERE notification_id = $1 AND user_id = $2`
	result, err := r.db.Exec(ctx, query, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

// GetPreference retrieves notification preferences for a user
func (r *NotificationRepository) GetPreference(ctx context.Context, userID uuid.UUID) (*domain.NotificationPreference, error) {
	query := `
		SELECT user_id, email_enabled, push_enabled, message_enabled, call_enabled,
		       friend_request_enabled, system_enabled, updated_at
		FROM notification_preferences
		WHERE user_id = $1
	`

	var pref domain.NotificationPreference
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&pref.UserID,
		&pref.EmailEnabled,
		&pref.PushEnabled,
		&pref.MessageEnabled,
		&pref.CallEnabled,
		&pref.FriendRequestEnabled,
		&pref.SystemEnabled,
		&pref.UpdatedAt,
	)

	if err != nil {
		// Return default preferences if not found
		return &domain.NotificationPreference{
			UserID:               userID,
			EmailEnabled:         true,
			PushEnabled:          true,
			MessageEnabled:       true,
			CallEnabled:          true,
			FriendRequestEnabled: true,
			SystemEnabled:        true,
			UpdatedAt:            time.Now(),
		}, nil
	}

	return &pref, nil
}

// UpdatePreference updates notification preferences for a user
func (r *NotificationRepository) UpdatePreference(ctx context.Context, pref *domain.NotificationPreference) error {
	query := `
		INSERT INTO notification_preferences
		(user_id, email_enabled, push_enabled, message_enabled, call_enabled,
		 friend_request_enabled, system_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET
			email_enabled = EXCLUDED.email_enabled,
			push_enabled = EXCLUDED.push_enabled,
			message_enabled = EXCLUDED.message_enabled,
			call_enabled = EXCLUDED.call_enabled,
			friend_request_enabled = EXCLUDED.friend_request_enabled,
			system_enabled = EXCLUDED.system_enabled,
			updated_at = NOW()
	`

	_, err := r.db.Exec(ctx, query,
		pref.UserID,
		pref.EmailEnabled,
		pref.PushEnabled,
		pref.MessageEnabled,
		pref.CallEnabled,
		pref.FriendRequestEnabled,
		pref.SystemEnabled,
	)

	if err != nil {
		return fmt.Errorf("failed to update notification preferences: %w", err)
	}

	return nil
}

// MarkAsPushed marks a notification as pushed (for push notification tracking)
func (r *NotificationRepository) MarkAsPushed(ctx context.Context, notificationID uuid.UUID) error {
	query := `UPDATE notifications SET is_pushed = true WHERE notification_id = $1`
	_, err := r.db.Exec(ctx, query, notificationID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as pushed: %w", err)
	}
	return nil
}

// GetUnpushed retrieves notifications that haven't been pushed yet
func (r *NotificationRepository) GetUnpushed(ctx context.Context, limit int) ([]domain.Notification, error) {
	query := `
		SELECT notification_id, user_id, type, title, body, data, is_read, is_pushed, created_at, read_at
		FROM notifications
		WHERE is_pushed = false
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unpushed notifications: %w", err)
	}
	defer rows.Close()

	var notifications []domain.Notification
	for rows.Next() {
		var n domain.Notification
		err := rows.Scan(
			&n.NotificationID,
			&n.UserID,
			&n.Type,
			&n.Title,
			&n.Body,
			&n.Data,
			&n.IsRead,
			&n.IsPushed,
			&n.CreatedAt,
			&n.ReadAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}

	return notifications, nil
}

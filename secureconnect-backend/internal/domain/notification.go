package domain

import (
	"time"

	"github.com/google/uuid"
)

// Notification represents a user notification
// Maps to CockroachDB notifications table
type Notification struct {
	NotificationID uuid.UUID              `json:"notification_id" db:"notification_id"`
	UserID         uuid.UUID              `json:"user_id" db:"user_id"`
	Type           string                 `json:"type" db:"type"` // message, call, friend_request, system
	Title          string                 `json:"title" db:"title"`
	Body           string                 `json:"body" db:"body"`
	Data           map[string]interface{} `json:"data,omitempty" db:"data"` // Additional metadata
	IsRead         bool                   `json:"is_read" db:"is_read"`
	IsPushed       bool                   `json:"is_pushed" db:"is_pushed"` // Whether push notification was sent
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	ReadAt         *time.Time             `json:"read_at,omitempty" db:"read_at"`
}

// NotificationPreference represents user's notification settings
// Maps to CockroachDB notification_preferences table
type NotificationPreference struct {
	UserID               uuid.UUID `json:"user_id" db:"user_id"`
	EmailEnabled         bool      `json:"email_enabled" db:"email_enabled"`
	PushEnabled          bool      `json:"push_enabled" db:"push_enabled"`
	MessageEnabled       bool      `json:"message_enabled" db:"message_enabled"`
	CallEnabled          bool      `json:"call_enabled" db:"call_enabled"`
	FriendRequestEnabled bool      `json:"friend_request_enabled" db:"friend_request_enabled"`
	SystemEnabled        bool      `json:"system_enabled" db:"system_enabled"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

// NotificationCreate represents data needed to create a notification
type NotificationCreate struct {
	UserID uuid.UUID
	Type   string
	Title  string
	Body   string
	Data   map[string]interface{}
}

// NotificationListResponse represents paginated notification list
type NotificationListResponse struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	TotalCount    int            `json:"total_count"`
	HasMore       bool           `json:"has_more"`
}

// NotificationPreferenceUpdate represents update request for notification preferences
type NotificationPreferenceUpdate struct {
	EmailEnabled         *bool `json:"email_enabled,omitempty"`
	PushEnabled          *bool `json:"push_enabled,omitempty"`
	MessageEnabled       *bool `json:"message_enabled,omitempty"`
	CallEnabled          *bool `json:"call_enabled,omitempty"`
	FriendRequestEnabled *bool `json:"friend_request_enabled,omitempty"`
	SystemEnabled        *bool `json:"system_enabled,omitempty"`
}

package domain

import (
	"time"
	
	"github.com/google/uuid"
)

// Conversation represents conversation metadata
// Maps to CockroachDB conversations table
type Conversation struct {
	ConversationID uuid.UUID  `json:"conversation_id" db:"conversation_id"`
	Type           string     `json:"type" db:"type"` // direct, group
	Name           *string    `json:"name,omitempty" db:"name"` // For group chats
	AvatarURL      *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	CreatedBy      uuid.UUID  `json:"created_by" db:"created_by"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

// ConversationParticipant represents a user in a conversation
// Maps to CockroachDB conversation_participants table
type ConversationParticipant struct {
	ConversationID uuid.UUID `json:"conversation_id" db:"conversation_id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	Role           string    `json:"role" db:"role"` // admin, member
	JoinedAt       time.Time `json:"joined_at" db:"joined_at"`
}

// ConversationSettings represents security and AI settings for a conversation
// This controls the Hybrid E2EE model
// Maps to CockroachDB conversation_settings table
type ConversationSettings struct {
	ConversationID        uuid.UUID `json:"conversation_id" db:"conversation_id"`
	IsE2EEEnabled         bool      `json:"is_e2ee_enabled" db:"is_e2ee_enabled"` // Default TRUE
	AIEnabled             bool      `json:"ai_enabled" db:"ai_enabled"` // Only when E2EE=false or Edge AI
	RecordingEnabled      bool      `json:"recording_enabled" db:"recording_enabled"`
	MessageRetentionDays  int       `json:"message_retention_days" db:"message_retention_days"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

// ConversationCreate represents data to create a new conversation
type ConversationCreate struct {
	Type          string      `json:"type" binding:"required,oneof=direct group"`
	Name          *string     `json:"name,omitempty"`
	ParticipantIDs []uuid.UUID `json:"participant_ids" binding:"required,min=1"`
}

// ConversationResponse is the full conversation data with participants
type ConversationResponse struct {
	ConversationID uuid.UUID                  `json:"conversation_id"`
	Type           string                     `json:"type"`
	Name           *string                    `json:"name,omitempty"`
	AvatarURL      *string                    `json:"avatar_url,omitempty"`
	Participants   []UserResponse             `json:"participants"`
	Settings       *ConversationSettings      `json:"settings"`
	LastMessage    *MessageResponse           `json:"last_message,omitempty"`
	UnreadCount    int                        `json:"unread_count"`
	CreatedAt      time.Time                  `json:"created_at"`
}

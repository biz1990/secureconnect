package domain

import (
	"time"
	
	"github.com/google/uuid"
)

// Call represents a video/audio call entity
type Call struct {
	CallID         uuid.UUID `json:"call_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	CallerID       uuid.UUID `json:"caller_id"`
	CallType       string    `json:"call_type"` // audio, video
	Status         string    `json:"status"`    // ringing, active, ended
	StartedAt      time.Time `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	Duration       int       `json:"duration,omitempty"` // in seconds
}

// CallParticipant represents a participant in a call
type CallParticipant struct {
	CallID    uuid.UUID  `json:"call_id"`
	UserID    uuid.UUID  `json:"user_id"`
	JoinedAt  time.Time  `json:"joined_at"`
	LeftAt    *time.Time `json:"left_at,omitempty"`
	IsMuted   bool       `json:"is_muted"`
	IsVideoOn bool       `json:"is_video_on"`
}

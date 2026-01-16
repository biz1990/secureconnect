package domain

import (
	"time"

	"github.com/google/uuid"
)

// Message represents a chat message entity
// Maps to Cassandra messages table
// Supports Hybrid E2EE model - can be encrypted or plaintext
type Message struct {
	MessageID      uuid.UUID              `json:"message_id" cql:"message_id"`
	ConversationID uuid.UUID              `json:"conversation_id" cql:"conversation_id"`
	SenderID       uuid.UUID              `json:"sender_id" cql:"sender_id"`
	Content        string                 `json:"content" cql:"content"`             // Can be Base64 ciphertext or plaintext
	IsEncrypted    bool                   `json:"is_encrypted" cql:"is_encrypted"`   // CRITICAL FLAG
	MessageType    string                 `json:"message_type" cql:"message_type"`   // text, image, video, file
	Metadata       map[string]interface{} `json:"metadata,omitempty" cql:"metadata"` // AI results or file info
	SentAt         time.Time              `json:"sent_at" cql:"sent_at"`
}

// MessageCreate represents data needed to send a message
type MessageCreate struct {
	ConversationID uuid.UUID              `json:"conversation_id" binding:"required"`
	Content        string                 `json:"content" binding:"required"`
	IsEncrypted    bool                   `json:"is_encrypted"`
	MessageType    string                 `json:"message_type" binding:"required,oneof=text image video file"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// MessageResponse represents the message returned to clients
type MessageResponse struct {
	MessageID      uuid.UUID              `json:"message_id"`
	ConversationID uuid.UUID              `json:"conversation_id"`
	SenderID       uuid.UUID              `json:"sender_id"`
	SenderName     string                 `json:"sender_name,omitempty"` // Joined from users table
	Content        string                 `json:"content"`
	IsEncrypted    bool                   `json:"is_encrypted"`
	MessageType    string                 `json:"message_type"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"` // AI metadata only if is_encrypted=false
	SentAt         time.Time              `json:"sent_at"`
}

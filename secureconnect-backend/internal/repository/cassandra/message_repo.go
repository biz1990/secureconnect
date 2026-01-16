package cassandra

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
)

// Helper function to convert uuid.UUID to gocql.UUID
func toGocqlUUID(u uuid.UUID) gocql.UUID {
	var g [16]byte
	copy(g[:], u[:])
	return g
}

// MessageRepository handles message storage in Cassandra
type MessageRepository struct {
	session *gocql.Session
}

// NewMessageRepository creates a new MessageRepository
func NewMessageRepository(session *gocql.Session) *MessageRepository {
	return &MessageRepository{session: session}
}

// Save inserts a new message into Cassandra
func (r *MessageRepository) Save(message *domain.Message) error {
	// Generate message_id if not set
	if message.MessageID == uuid.Nil {
		message.MessageID = uuid.New()
	}

	// Handle metadata - convert to map[string]string for Cassandra MAP<TEXT, TEXT>
	metadataMap := make(map[string]string)
	if message.Metadata != nil {
		for k, v := range message.Metadata {
			// Convert value to string
			switch val := v.(type) {
			case string:
				metadataMap[k] = val
			case int, int8, int16, int32, int64:
				metadataMap[k] = fmt.Sprintf("%d", val)
			case float32, float64:
				metadataMap[k] = fmt.Sprintf("%f", val)
			case bool:
				metadataMap[k] = fmt.Sprintf("%t", val)
			default:
				metadataMap[k] = fmt.Sprintf("%v", val)
			}
		}
	} else {
		// Use empty map if metadata is nil
		metadataMap = map[string]string{}
	}

	query := `INSERT INTO messages (conversation_id, message_id, sender_id, content, is_encrypted, message_type, metadata, sent_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	err := r.session.Query(query,
		toGocqlUUID(message.ConversationID),
		toGocqlUUID(message.MessageID),
		toGocqlUUID(message.SenderID),
		message.Content,
		message.IsEncrypted,
		message.MessageType,
		metadataMap,
		message.SentAt,
	).Exec()

	if err != nil {
		fmt.Printf("CASSANDRA ERROR: failed to save message: %v\n", err)
		return fmt.Errorf("failed to save message: %w", err)
	}

	fmt.Printf("CASSANDRA SUCCESS: message saved: conversation_id=%s, message_id=%s\n", message.ConversationID, message.MessageID)
	return nil
}

// GetByConversation retrieves messages for a conversation with pagination
func (r *MessageRepository) GetByConversation(
	conversationID uuid.UUID,
	limit int,
	pageState []byte,
) ([]*domain.Message, []byte, error) {
	query := `
		SELECT conversation_id, message_id, sender_id, content,
		       is_encrypted, message_type, metadata, sent_at
		FROM messages
		WHERE conversation_id = ?
		ORDER BY sent_at DESC
		LIMIT ?
	`

	iter := r.session.Query(query, toGocqlUUID(conversationID), limit).PageState(pageState).Iter()
	defer iter.Close()

	var messages []*domain.Message

	for {
		message := &domain.Message{}
		if !iter.Scan(
			&message.ConversationID,
			&message.MessageID,
			&message.SenderID,
			&message.Content,
			&message.IsEncrypted,
			&message.MessageType,
			&message.Metadata,
			&message.SentAt,
		) {
			break
		}
		messages = append(messages, message)
	}

	if err := iter.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Get next page state for cursor-based pagination
	nextPageState := iter.PageState()

	return messages, nextPageState, nil
}

// GetMultipleBuckets retrieves messages across multiple buckets
// Used when time range spans multiple months
func (r *MessageRepository) GetMultipleBuckets(
	conversationID uuid.UUID,
	buckets []int,
	limit int,
) ([]*domain.Message, error) {
	// Simplified implementation - just get recent messages
	// Bucketing is no longer used with the new schema
	messages, _, err := r.GetByConversation(conversationID, limit, nil)
	if err != nil {
		return nil, err
	}

	// Limit total results
	if len(messages) > limit {
		messages = messages[:limit]
	}

	return messages, nil
}

// GetRecentMessages gets messages from current bucket (most common case)
func (r *MessageRepository) GetRecentMessages(conversationID uuid.UUID, limit int) ([]*domain.Message, error) {
	messages, _, err := r.GetByConversation(conversationID, limit, nil)
	return messages, err
}

// GetByID retrieves a specific message
func (r *MessageRepository) GetByID(conversationID uuid.UUID, bucket int, messageID uuid.UUID) (*domain.Message, error) {
	query := `
		SELECT conversation_id, message_id, sender_id, content,
		       is_encrypted, message_type, metadata, sent_at
		FROM messages
		WHERE conversation_id = ? AND message_id = ?
		LIMIT 1
	`

	message := &domain.Message{}
	err := r.session.Query(query, toGocqlUUID(conversationID), toGocqlUUID(messageID)).Scan(
		&message.ConversationID,
		&message.MessageID,
		&message.SenderID,
		&message.Content,
		&message.IsEncrypted,
		&message.MessageType,
		&message.Metadata,
		&message.SentAt,
	)

	if err != nil {
		if err == gocql.ErrNotFound {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return message, nil
}

// Delete removes a message (if needed for GDPR compliance)
func (r *MessageRepository) Delete(conversationID uuid.UUID, bucket int, messageID uuid.UUID) error {
	query := `DELETE FROM messages WHERE conversation_id = ? AND message_id = ?`

	err := r.session.Query(query, toGocqlUUID(conversationID), toGocqlUUID(messageID)).Exec()
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

// CountMessages counts total messages in a conversation (expensive, use sparingly)
func (r *MessageRepository) CountMessages(conversationID uuid.UUID, bucket int) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`

	var count int
	err := r.session.Query(query, toGocqlUUID(conversationID)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}

	return count, nil
}

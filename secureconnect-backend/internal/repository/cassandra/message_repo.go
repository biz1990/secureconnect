package cassandra

import (
	"fmt"
	"time"
	
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	
	"secureconnect-backend/internal/domain"
)

// MessageRepository handles message storage in Cassandra
// Implements bucketing strategy for scalability
type MessageRepository struct {
	session *gocql.Session
}

// NewMessageRepository creates a new MessageRepository
func NewMessageRepository(session *gocql.Session) *MessageRepository {
	return &MessageRepository{session: session}
}

// Save inserts a new message into Cassandra
func (r *MessageRepository) Save(message *domain.Message) error {
	// Calculate bucket if not already set
	if message.Bucket == 0 {
		message.Bucket = domain.CalculateBucket(message.CreatedAt)
	}
	
	// Generate message_id if not set (TIMEUUID)
	if message.MessageID == uuid.Nil {
		message.MessageID = uuid.New()
	}
	
	query := `
		INSERT INTO messages (
			conversation_id, bucket, message_id, sender_id, content,
			is_encrypted, message_type, metadata, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	err := r.session.Query(query,
		message.ConversationID,
		message.Bucket,
		message.MessageID,
		message.SenderID,
		message.Content,
		message.IsEncrypted,
		message.MessageType,
		message.Metadata,
		message.CreatedAt,
	).Exec()
	
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	
	return nil
}

// GetByConversation retrieves messages for a conversation with pagination
// Uses bucket + created_at for efficient querying
func (r *MessageRepository) GetByConversation(
	conversationID uuid.UUID,
	bucket int,
	limit int,
	pageState []byte,
) ([]*domain.Message, []byte, error) {
	query := `
		SELECT conversation_id, bucket, message_id, sender_id, content,
		       is_encrypted, message_type, metadata, created_at
		FROM messages
		WHERE conversation_id = ? AND bucket = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	
	iter := r.session.Query(query, conversationID, bucket, limit).PageState(pageState).Iter()
	defer iter.Close()
	
	var messages []*domain.Message
	
	for {
		message := &domain.Message{}
		if !iter.Scan(
			&message.ConversationID,
			&message.Bucket,
			&message.MessageID,
			&message.SenderID,
			&message.Content,
			&message.IsEncrypted,
			&message.MessageType,
			&message.Metadata,
			&message.CreatedAt,
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
	var allMessages []*domain.Message
	
	for _, bucket := range buckets {
		messages, _, err := r.GetByConversation(conversationID, bucket, limit, nil)
		if err != nil {
			return nil, err
		}
		allMessages = append(allMessages, messages...)
		
		// Stop if we have enough messages
		if len(allMessages) >= limit {
			break
		}
	}
	
	// Limit total results
	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}
	
	return allMessages, nil
}

// GetRecentMessages gets messages from current bucket (most common case)
func (r *MessageRepository) GetRecentMessages(conversationID uuid.UUID, limit int) ([]*domain.Message, error) {
	currentBucket := domain.CalculateBucket(time.Now())
	messages, _, err := r.GetByConversation(conversationID, currentBucket, limit, nil)
	return messages, err
}

// GetByID retrieves a specific message
func (r *MessageRepository) GetByID(conversationID uuid.UUID, bucket int, messageID uuid.UUID) (*domain.Message, error) {
	query := `
		SELECT conversation_id, bucket, message_id, sender_id, content,
		       is_encrypted, message_type, metadata, created_at
		FROM messages
		WHERE conversation_id = ? AND bucket = ? AND message_id = ?
		LIMIT 1
	`
	
	message := &domain.Message{}
	err := r.session.Query(query, conversationID, bucket, messageID).Scan(
		&message.ConversationID,
		&message.Bucket,
		&message.MessageID,
		&message.SenderID,
		&message.Content,
		&message.IsEncrypted,
		&message.MessageType,
		&message.Metadata,
		&message.CreatedAt,
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
	query := `DELETE FROM messages WHERE conversation_id = ? AND bucket = ? AND message_id = ?`
	
	err := r.session.Query(query, conversationID, bucket, messageID).Exec()
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	
	return nil
}

// CountMessages counts total messages in a conversation (expensive, use sparingly)
func (r *MessageRepository) CountMessages(conversationID uuid.UUID, bucket int) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE conversation_id = ? AND bucket = ?`
	
	var count int
	err := r.session.Query(query, conversationID, bucket).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	
	return count, nil
}

// CalculateBucketsForRange generates bucket list for a time range
func CalculateBucketsForRange(startTime, endTime time.Time) []int {
	var buckets []int
	
	current := startTime
	for current.Before(endTime) || current.Equal(endTime) {
		bucket := domain.CalculateBucket(current)
		buckets = append(buckets, bucket)
		
		// Move to next month
		current = current.AddDate(0, 1, 0)
	}
	
	return buckets
}

package cassandra

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"secureconnect-backend/internal/database"
	"secureconnect-backend/internal/domain"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

// Default retry configuration
const (
	MaxRetries   = 3
	RetryDelay   = 100 * time.Millisecond
	RetryBackoff = 2
)

// Helper function to convert uuid.UUID to gocql.UUID
func toGocqlUUID(u uuid.UUID) gocql.UUID {
	var g [16]byte
	copy(g[:], u[:])
	return g
}

// MessageRepository handles message storage in Cassandra with timeout and retry support
type MessageRepository struct {
	db *database.CassandraDB
}

// NewMessageRepository creates a new MessageRepository
func NewMessageRepository(db *database.CassandraDB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Save inserts a new message into Cassandra with timeout and retry logic
func (r *MessageRepository) Save(ctx context.Context, message *domain.Message) error {
	startTime := time.Now()
	operation := "save"
	table := "messages"

	// Generate message_id if not set
	if message.MessageID == uuid.Nil {
		message.MessageID = uuid.New()
	}

	// Handle metadata - convert to map[string]string for Cassandra MAP<TEXT, TEXT>
	metadataMap := make(map[string]string)
	if message.Metadata != nil {
		const MaxMetadataKeyLen = 100
		const MaxMetadataValueLen = 1000

		for k, v := range message.Metadata {
			// Defense in depth: validate lengths
			if len(k) > MaxMetadataKeyLen {
				return fmt.Errorf("metadata key too long: %s", k)
			}

			var strVal string
			// Convert value to string
			switch val := v.(type) {
			case string:
				strVal = val
			case int, int8, int16, int32, int64:
				strVal = fmt.Sprintf("%d", val)
			case float32, float64:
				strVal = fmt.Sprintf("%f", val)
			case bool:
				strVal = fmt.Sprintf("%t", val)
			default:
				strVal = fmt.Sprintf("%v", val)
			}

			if len(strVal) > MaxMetadataValueLen {
				return fmt.Errorf("metadata value too long for key %s", k)
			}
			metadataMap[k] = strVal
		}
	} else {
		// Use empty map if metadata is nil
		metadataMap = map[string]string{}
	}

	query := `INSERT INTO messages (conversation_id, message_id, sender_id, content, is_encrypted, message_type, metadata, sent_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	// Execute with retry logic that respects context cancellation
	err := r.executeWithRetry(ctx, operation, table, func() error {
		return r.db.ExecWithContext(ctx, query,
			toGocqlUUID(message.ConversationID),
			toGocqlUUID(message.MessageID),
			toGocqlUUID(message.SenderID),
			message.Content,
			message.IsEncrypted,
			message.MessageType,
			metadataMap,
			message.SentAt,
		)
	})

	// Record metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordCassandraQueryDuration(operation, table, duration)
	if err != nil {
		metrics.RecordCassandraQueryError(operation, table, classifyError(err))
		metrics.RecordCassandraWriteError(table, classifyError(err))
		logger.Error("Failed to save message",
			zap.String("conversation_id", message.ConversationID.String()),
			zap.String("message_id", message.MessageID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to save message: %w", err)
	}

	metrics.RecordCassandraQuery(operation, table, "success")
	logger.Debug("Message saved successfully",
		zap.String("conversation_id", message.ConversationID.String()),
		zap.String("message_id", message.MessageID.String()))
	return nil
}

// GetByConversation retrieves messages for a conversation with pagination and timeout
func (r *MessageRepository) GetByConversation(
	ctx context.Context,
	conversationID uuid.UUID,
	limit int,
	pageState []byte,
) ([]*domain.Message, []byte, error) {
	startTime := time.Now()
	operation := "get_by_conversation"
	table := "messages"

	query := `
		SELECT conversation_id, message_id, sender_id, content,
		       is_encrypted, message_type, metadata, sent_at
		FROM messages
		WHERE conversation_id = ?
		ORDER BY sent_at DESC
		LIMIT ?
	`

	var messages []*domain.Message
	var nextPageState []byte

	// Execute with retry logic that respects context cancellation
	err := r.executeWithRetry(ctx, operation, table, func() error {
		iter := r.db.QueryWithContext(ctx, query, toGocqlUUID(conversationID), limit).PageState(pageState).Iter()
		defer iter.Close()

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

		nextPageState = iter.PageState()
		return iter.Close()
	})

	// Record metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordCassandraQueryDuration(operation, table, duration)
	if err != nil {
		metrics.RecordCassandraQueryError(operation, table, classifyError(err))
		metrics.RecordCassandraReadError(table, classifyError(err))
		logger.Error("Failed to fetch messages",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
		return nil, nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	metrics.RecordCassandraQuery(operation, table, "success")
	return messages, nextPageState, nil
}

// GetMultipleBuckets retrieves messages across multiple buckets
// Used when time range spans multiple months
func (r *MessageRepository) GetMultipleBuckets(
	ctx context.Context,
	conversationID uuid.UUID,
	buckets []int,
	limit int,
) ([]*domain.Message, []byte, error) {
	startTime := time.Now()
	operation := "get_multiple_buckets"
	table := "messages"

	// Simplified implementation - just get recent messages
	// Bucketing is no longer used with new schema
	messages, _, err := r.GetByConversation(ctx, conversationID, limit, nil)

	// Record metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordCassandraQueryDuration(operation, table, duration)
	if err != nil {
		metrics.RecordCassandraQueryError(operation, table, classifyError(err))
		metrics.RecordCassandraReadError(table, classifyError(err))
		return messages, nil, nil
	}

	// Limit total results
	if len(messages) > limit {
		messages = messages[:limit]
	}

	return messages, nil, nil
}

// GetRecentMessages gets messages from current bucket (most common case)
func (r *MessageRepository) GetRecentMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]*domain.Message, []byte, error) {
	return r.GetByConversation(ctx, conversationID, limit, nil)
}

// GetByID retrieves a specific message with timeout
func (r *MessageRepository) GetByID(ctx context.Context, conversationID uuid.UUID, bucket int, messageID uuid.UUID) (*domain.Message, error) {
	startTime := time.Now()
	operation := "get_by_id"
	table := "messages"

	query := `
		SELECT conversation_id, message_id, sender_id, content,
		       is_encrypted, message_type, metadata, sent_at
		FROM messages
		WHERE conversation_id = ? AND message_id = ?
		LIMIT 1
	`

	message := &domain.Message{}

	// Execute with retry logic that respects context cancellation
	err := r.executeWithRetry(ctx, operation, table, func() error {
		return r.db.QueryWithContext(ctx, query, toGocqlUUID(conversationID), toGocqlUUID(messageID)).Scan(
			&message.ConversationID,
			&message.MessageID,
			&message.SenderID,
			&message.Content,
			&message.IsEncrypted,
			&message.MessageType,
			&message.Metadata,
			&message.SentAt,
		)
	})

	// Record metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordCassandraQueryDuration(operation, table, duration)
	if err != nil {
		if err == gocql.ErrNotFound {
			metrics.RecordCassandraQuery(operation, table, "not_found")
			return nil, fmt.Errorf("message not found")
		}
		metrics.RecordCassandraQueryError(operation, table, classifyError(err))
		metrics.RecordCassandraReadError(table, classifyError(err))
		logger.Error("Failed to get message",
			zap.String("conversation_id", conversationID.String()),
			zap.String("message_id", messageID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	metrics.RecordCassandraQuery(operation, table, "success")
	return message, nil
}

// Delete removes a message (if needed for GDPR compliance)
func (r *MessageRepository) Delete(ctx context.Context, conversationID uuid.UUID, bucket int, messageID uuid.UUID) error {
	startTime := time.Now()
	operation := "delete"
	table := "messages"

	query := `DELETE FROM messages WHERE conversation_id = ? AND message_id = ?`

	// Execute with retry logic that respects context cancellation
	err := r.executeWithRetry(ctx, operation, table, func() error {
		return r.db.ExecWithContext(ctx, query, toGocqlUUID(conversationID), toGocqlUUID(messageID))
	})

	// Record metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordCassandraQueryDuration(operation, table, duration)
	if err != nil {
		metrics.RecordCassandraQueryError(operation, table, classifyError(err))
		metrics.RecordCassandraWriteError(table, classifyError(err))
		logger.Error("Failed to delete message",
			zap.String("conversation_id", conversationID.String()),
			zap.String("message_id", messageID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to delete message: %w", err)
	}

	metrics.RecordCassandraQuery(operation, table, "success")
	return nil
}

// CountMessages counts total messages in a conversation (expensive, use sparingly)
func (r *MessageRepository) CountMessages(ctx context.Context, conversationID uuid.UUID, bucket int) (int, error) {
	startTime := time.Now()
	operation := "count"
	table := "messages"

	query := `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`

	var count int

	// Execute with retry logic that respects context cancellation
	err := r.executeWithRetry(ctx, operation, table, func() error {
		return r.db.QueryWithContext(ctx, query, toGocqlUUID(conversationID)).Scan(&count)
	})

	// Record metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordCassandraQueryDuration(operation, table, duration)
	if err != nil {
		metrics.RecordCassandraQueryError(operation, table, classifyError(err))
		metrics.RecordCassandraReadError(table, classifyError(err))
		logger.Error("Failed to count messages",
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}

	metrics.RecordCassandraQuery(operation, table, "success")
	return count, nil
}

// executeWithRetry executes a function with retry logic that respects context cancellation
// It will retry on transient errors but abort immediately on context cancellation or timeout
func (r *MessageRepository) executeWithRetry(ctx context.Context, operation, table string, fn func() error) error {
	var lastErr error
	delay := RetryDelay

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		// Check if context is cancelled before attempting
		select {
		case <-ctx.Done():
			// Context cancelled, return immediately
			metrics.RecordCassandraQueryTimeout(operation, table)
			logger.Warn("Cassandra query cancelled by context",
				zap.String("operation", operation),
				zap.String("table", table),
				zap.Int("attempt", attempt))
			return domain.ErrCassandraTimeout
		default:
			// Context not cancelled, proceed with attempt
		}

		// Execute the function
		err := fn()
		if err == nil {
			// Success, no retry needed
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			// Non-retryable error, return immediately
			return err
		}

		// Check if this was the last attempt
		if attempt == MaxRetries {
			metrics.RecordCassandraQueryRetryExhausted(operation, table)
			logger.Error("Cassandra query retries exhausted",
				zap.String("operation", operation),
				zap.String("table", table),
				zap.Int("attempts", attempt),
				zap.Error(err))
			return fmt.Errorf("max retries (%d) exhausted: %w", MaxRetries, err)
		}

		// Record retry metric
		metrics.RecordCassandraQueryRetry(operation, table, classifyError(err))

		// Wait before retrying with exponential backoff
		logger.Debug("Retrying Cassandra query",
			zap.String("operation", operation),
			zap.String("table", table),
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay),
			zap.Error(err))

		select {
		case <-ctx.Done():
			// Context cancelled during backoff
			metrics.RecordCassandraQueryTimeout(operation, table)
			return domain.ErrCassandraTimeout
		case <-time.After(delay):
			// Backoff completed, proceed with next attempt
		}

		// Exponential backoff
		delay *= time.Duration(RetryBackoff)
	}

	return lastErr
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for timeout errors
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false
	}

	// Check for gocql specific errors
	// gocql errors are typically of type error, not RequestErr
	// We'll use error string matching for common Cassandra errors
	errStr := err.Error()

	// Timeout errors are not retryable
	if errStr == "request timeout" || errStr == "timeout" {
		return false
	}

	// Host unavailable errors may be retryable
	if errStr == "no hosts available" || errStr == "unavailable" {
		return true
	}

	// Other errors are generally retryable for Cassandra
	return true
}

// classifyError classifies an error for metrics
func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	// Check for timeout
	if err == context.DeadlineExceeded {
		return "timeout"
	}
	if err == context.Canceled {
		return "cancelled"
	}

	// Check for domain errors
	if domainErr, ok := err.(*domain.CassandraError); ok {
		return domainErr.Code
	}

	// Check for gocql errors by string matching
	errStr := err.Error()

	// Common Cassandra error codes
	switch errStr {
	case "request timeout", "timeout":
		return "timeout"
	case "no hosts available", "unavailable":
		return "unavailable"
	case "not found":
		return "not_found"
	default:
		return "unknown"
	}
}

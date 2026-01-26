package cockroach

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"secureconnect-backend/internal/domain"
)

// Transaction provides transaction support
type Transaction struct {
	tx pgx.Tx
}

// BeginTx starts a new transaction
func (r *ConversationRepository) BeginTx(ctx context.Context) (*Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &Transaction{tx: tx}, nil
}

// Commit commits the transaction
func (t *Transaction) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

// Rollback rolls back the transaction
func (t *Transaction) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// ConversationRepository handles conversation operations
type ConversationRepository struct {
	pool *pgxpool.Pool
}

// NewConversationRepository creates a new conversation repository
func NewConversationRepository(pool *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{pool: pool}
}

// Create creates a new conversation
func (r *ConversationRepository) Create(ctx context.Context, conversation *domain.Conversation) error {
	query := `
		INSERT INTO conversations (
			conversation_id, title, type, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING conversation_id
	`

	_, err := r.pool.Exec(ctx, query,
		conversation.ConversationID,
		conversation.Title,
		conversation.Type,
		conversation.CreatedBy,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}

	return nil
}

// CreateTx creates a new conversation within a transaction
func (r *ConversationRepository) CreateTx(ctx context.Context, tx *Transaction, conversation *domain.Conversation) error {
	query := `
		INSERT INTO conversations (
			conversation_id, title, type, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING conversation_id
	`

	_, err := tx.tx.Exec(ctx, query,
		conversation.ConversationID,
		conversation.Title,
		conversation.Type,
		conversation.CreatedBy,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}

	return nil
}

// AddParticipant adds a user to conversation
func (r *ConversationRepository) AddParticipant(ctx context.Context, conversationID, userID uuid.UUID, role string) error {
	query := `
		INSERT INTO conversation_participants (
			conversation_id, user_id, role, joined_at
		) VALUES ($1, $2, $3, $4)
	`

	_, err := r.pool.Exec(ctx, query, conversationID, userID, role, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}

	return nil
}

// AddParticipantTx adds a user to conversation within a transaction
func (r *ConversationRepository) AddParticipantTx(ctx context.Context, tx *Transaction, conversationID, userID uuid.UUID, role string) error {
	query := `
		INSERT INTO conversation_participants (
			conversation_id, user_id, role, joined_at
		) VALUES ($1, $2, $3, $4)
	`

	_, err := tx.tx.Exec(ctx, query, conversationID, userID, role, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}

	return nil
}

// GetByID retrieves a conversation by ID
func (r *ConversationRepository) GetByID(ctx context.Context, conversationID uuid.UUID) (*domain.Conversation, error) {
	query := `
		SELECT conversation_id, title, type, created_by, created_at, updated_at
		FROM conversations
		WHERE conversation_id = $1
	`

	conversation := &domain.Conversation{}
	err := r.pool.QueryRow(ctx, query, conversationID).Scan(
		&conversation.ConversationID,
		&conversation.Title,
		&conversation.Type,
		&conversation.CreatedBy,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return conversation, nil
}

// GetUserConversations retrieves all conversations for a user
func (r *ConversationRepository) GetUserConversations(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.Conversation, error) {
	query := `
		SELECT c.conversation_id, c.title, c.type, c.created_by, c.created_at, c.updated_at
		FROM conversations c
		INNER JOIN conversation_participants cp ON c.conversation_id = cp.conversation_id
		WHERE cp.user_id = $1
		ORDER BY c.updated_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*domain.Conversation
	for rows.Next() {
		conversation := &domain.Conversation{}
		err := rows.Scan(
			&conversation.ConversationID,
			&conversation.Title,
			&conversation.Type,
			&conversation.CreatedBy,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, conversation)
	}

	return conversations, nil
}

// GetParticipants retrieves all participants in a conversation
func (r *ConversationRepository) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT user_id FROM conversation_participants WHERE conversation_id = $1
	`

	rows, err := r.pool.Query(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}
	defer rows.Close()

	var participants []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		participants = append(participants, userID)
	}

	return participants, nil
}

// UpdateSettings updates conversation settings
func (r *ConversationRepository) UpdateSettings(ctx context.Context, conversationID uuid.UUID, settings *domain.ConversationSettings) error {
	// First check if settings exist
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM conversation_settings WHERE conversation_id = $1)`
	err := r.pool.QueryRow(ctx, checkQuery, conversationID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check settings existence: %w", err)
	}

	if exists {
		// Update
		query := `
			UPDATE conversation_settings
			SET is_e2ee_enabled = $2, updated_at = $3
			WHERE conversation_id = $1
		`
		_, err = r.pool.Exec(ctx, query, conversationID, settings.IsE2EEEnabled, time.Now())
	} else {
		// Insert
		query := `
			INSERT INTO conversation_settings (conversation_id, is_e2ee_enabled, updated_at)
			VALUES ($1, $2, $3)
		`
		_, err = r.pool.Exec(ctx, query, conversationID, settings.IsE2EEEnabled, time.Now())
	}

	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	return nil
}

// UpdateSettingsTx updates conversation settings within a transaction
func (r *ConversationRepository) UpdateSettingsTx(ctx context.Context, tx *Transaction, conversationID uuid.UUID, settings *domain.ConversationSettings) error {
	// First check if settings exist
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM conversation_settings WHERE conversation_id = $1)`
	err := tx.tx.QueryRow(ctx, checkQuery, conversationID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check settings existence: %w", err)
	}

	if exists {
		// Update
		query := `
			UPDATE conversation_settings
			SET is_e2ee_enabled = $2, updated_at = $3
			WHERE conversation_id = $1
		`
		_, err = tx.tx.Exec(ctx, query, conversationID, settings.IsE2EEEnabled, time.Now())
	} else {
		// Insert
		query := `
			INSERT INTO conversation_settings (conversation_id, is_e2ee_enabled, updated_at)
			VALUES ($1, $2, $3)
		`
		_, err = tx.tx.Exec(ctx, query, conversationID, settings.IsE2EEEnabled, time.Now())
	}

	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	return nil
}

// GetSettings retrieves conversation settings
func (r *ConversationRepository) GetSettings(ctx context.Context, conversationID uuid.UUID) (*domain.ConversationSettings, error) {
	query := `
		SELECT conversation_id, is_e2ee_enabled
		FROM conversation_settings
		WHERE conversation_id = $1
	`

	settings := &domain.ConversationSettings{}
	err := r.pool.QueryRow(ctx, query, conversationID).Scan(
		&settings.ConversationID,
		&settings.IsE2EEEnabled,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Default settings if not found
			return &domain.ConversationSettings{
				ConversationID: conversationID,
				IsE2EEEnabled:  true, // Default to E2EE enabled
			}, nil
		}
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	return settings, nil
}

// Delete deletes a conversation
func (r *ConversationRepository) Delete(ctx context.Context, conversationID uuid.UUID) error {
	query := `DELETE FROM conversations WHERE conversation_id = $1`

	_, err := r.pool.Exec(ctx, query, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// IsParticipant checks if a user is a participant in a conversation
func (r *ConversationRepository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM conversation_participants WHERE conversation_id = $1 AND user_id = $2)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, conversationID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check participant: %w", err)
	}

	return exists, nil
}

// GetParticipantsWithDetails retrieves all participants in a conversation with user details
func (r *ConversationRepository) GetParticipantsWithDetails(ctx context.Context, conversationID uuid.UUID) ([]*domain.ConversationParticipantDetail, error) {
	query := `
		SELECT
			cp.conversation_id, cp.user_id, cp.role, cp.joined_at,
			u.email, u.username, u.display_name, u.avatar_url, u.status
		FROM conversation_participants cp
		INNER JOIN users u ON cp.user_id = u.user_id
		WHERE cp.conversation_id = $1
		ORDER BY cp.joined_at ASC
	`

	rows, err := r.pool.Query(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants with details: %w", err)
	}
	defer rows.Close()

	var participants []*domain.ConversationParticipantDetail
	for rows.Next() {
		participant := &domain.ConversationParticipantDetail{}
		err := rows.Scan(
			&participant.ConversationID,
			&participant.UserID,
			&participant.Role,
			&participant.JoinedAt,
			&participant.Email,
			&participant.Username,
			&participant.DisplayName,
			&participant.AvatarURL,
			&participant.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		participants = append(participants, participant)
	}

	return participants, nil
}

// RemoveParticipant removes a user from a conversation
func (r *ConversationRepository) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	query := `DELETE FROM conversation_participants WHERE conversation_id = $1 AND user_id = $2`

	cmdTag, err := r.pool.Exec(ctx, query, conversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("participant not found")
	}

	return nil
}

// UpdateConversation updates conversation metadata
func (r *ConversationRepository) UpdateConversation(ctx context.Context, conversationID uuid.UUID, title *string, avatarURL *string) error {
	query := `
		UPDATE conversations
		SET title = COALESCE($2, title),
		    avatar_url = COALESCE($3, avatar_url),
		    updated_at = NOW()
		WHERE conversation_id = $1
	`

	cmdTag, err := r.pool.Exec(ctx, query, conversationID, title, avatarURL)
	if err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("conversation not found")
	}

	return nil
}

// IsUserInConversation checks if a user is a participant in a conversation
func (r *ConversationRepository) IsUserInConversation(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	return r.IsParticipant(ctx, conversationID, userID)
}

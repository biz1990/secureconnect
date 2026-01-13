package cockroach

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"secureconnect-backend/internal/domain"
)

// BlockedUserRepository handles blocked user data operations in CockroachDB
type BlockedUserRepository struct {
	pool *pgxpool.Pool
}

// NewBlockedUserRepository creates a new BlockedUserRepository
func NewBlockedUserRepository(pool *pgxpool.Pool) *BlockedUserRepository {
	return &BlockedUserRepository{pool: pool}
}

// BlockedUser represents a blocked user relationship
type BlockedUser struct {
	BlockerID uuid.UUID `json:"blocker_id" db:"blocker_id"`
	BlockedID uuid.UUID `json:"blocked_id" db:"blocked_id"`
	Reason    *string   `json:"reason,omitempty" db:"reason"`
	CreatedAt string    `json:"created_at" db:"created_at"`
}

// BlockUser blocks another user
func (r *BlockedUserRepository) BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID, reason *string) error {
	query := `
		INSERT INTO blocked_users (blocker_id, blocked_id, reason, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (blocker_id, blocked_id) DO UPDATE SET
			reason = EXCLUDED.reason,
			created_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query, blockerID, blockedID, reason)
	if err != nil {
		return fmt.Errorf("failed to block user: %w", err)
	}

	return nil
}

// UnblockUser unblocks a user
func (r *BlockedUserRepository) UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	query := `
		DELETE FROM blocked_users
		WHERE blocker_id = $1 AND blocked_id = $2
		RETURNING blocker_id, blocked_id
	`

	var bid1, bid2 uuid.UUID
	err := r.pool.QueryRow(ctx, query, blockerID, blockedID).Scan(&bid1, &bid2)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("blocked user relationship not found")
		}
		return fmt.Errorf("failed to unblock user: %w", err)
	}

	return nil
}

// GetBlockedUsers retrieves list of blocked users for a user
func (r *BlockedUserRepository) GetBlockedUsers(ctx context.Context, blockerID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	query := `
		SELECT u.user_id, u.email, u.username, u.password_hash, u.display_name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM users u
		INNER JOIN blocked_users b ON b.blocked_id = u.user_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, blockerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user := &domain.User{}
		err := rows.Scan(
			&user.UserID,
			&user.Email,
			&user.Username,
			&user.PasswordHash,
			&user.DisplayName,
			&user.AvatarURL,
			&user.Status,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// IsBlocked checks if a user is blocked by another user
func (r *BlockedUserRepository) IsBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM blocked_users WHERE blocker_id = $1 AND blocked_id = $2)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, blockerID, blockedID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is blocked: %w", err)
	}

	return exists, nil
}

// GetBlockedBy retrieves users who have blocked a specific user
func (r *BlockedUserRepository) GetBlockedBy(ctx context.Context, blockedID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	query := `
		SELECT u.user_id, u.email, u.username, u.password_hash, u.display_name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM users u
		INNER JOIN blocked_users b ON b.blocker_id = u.user_id
		WHERE b.blocked_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, blockedID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get users who blocked this user: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user := &domain.User{}
		err := rows.Scan(
			&user.UserID,
			&user.Email,
			&user.Username,
			&user.PasswordHash,
			&user.DisplayName,
			&user.AvatarURL,
			&user.Status,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

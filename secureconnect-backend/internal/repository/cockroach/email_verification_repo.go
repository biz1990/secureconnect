package cockroach

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmailVerificationRepository handles email verification token operations in CockroachDB
type EmailVerificationRepository struct {
	pool *pgxpool.Pool
}

// NewEmailVerificationRepository creates a new EmailVerificationRepository
func NewEmailVerificationRepository(pool *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{pool: pool}
}

// EmailVerificationToken represents an email verification token
type EmailVerificationToken struct {
	TokenID   uuid.UUID  `json:"token_id" db:"token_id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	NewEmail  string     `json:"new_email" db:"new_email"`
	Token     string     `json:"token" db:"token"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UsedAt    *time.Time `json:"used_at,omitempty" db:"used_at"`
}

// CreateToken creates a new email verification token
func (r *EmailVerificationRepository) CreateToken(ctx context.Context, userID uuid.UUID, newEmail, token string, expiresAt time.Time) error {
	query := `
		INSERT INTO email_verification_tokens (user_id, new_email, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING token_id, created_at
	`

	var tokenID uuid.UUID
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, query, userID, newEmail, token, expiresAt).Scan(&tokenID, &createdAt)
	if err != nil {
		return fmt.Errorf("failed to create email verification token: %w", err)
	}

	return nil
}

// GetToken retrieves an email verification token
func (r *EmailVerificationRepository) GetToken(ctx context.Context, token string) (*EmailVerificationToken, error) {
	query := `
		SELECT token_id, user_id, new_email, token, expires_at, created_at, used_at
		FROM email_verification_tokens
		WHERE token = $1
	`

	evt := &EmailVerificationToken{}
	err := r.pool.QueryRow(ctx, query, token).Scan(
		&evt.TokenID,
		&evt.UserID,
		&evt.NewEmail,
		&evt.Token,
		&evt.ExpiresAt,
		&evt.CreatedAt,
		&evt.UsedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("failed to get email verification token: %w", err)
	}

	return evt, nil
}

// MarkTokenUsed marks a token as used
func (r *EmailVerificationRepository) MarkTokenUsed(ctx context.Context, token string) error {
	query := `
		UPDATE email_verification_tokens
		SET used_at = NOW()
		WHERE token = $1 AND used_at IS NULL
		RETURNING token_id
	`

	var tokenID uuid.UUID
	err := r.pool.QueryRow(ctx, query, token).Scan(&tokenID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("token not found or already used")
		}
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	return nil
}

// DeleteExpiredTokens deletes expired tokens
func (r *EmailVerificationRepository) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM email_verification_tokens
		WHERE expires_at < NOW()
		RETURNING token_id
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tokens: %w", err)
	}
	defer rows.Close()

	count := int64(0)
	for rows.Next() {
		count++
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("error iterating deleted tokens: %w", err)
	}

	return count, nil
}

// DeleteUserTokens deletes all tokens for a user
func (r *EmailVerificationRepository) DeleteUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM email_verification_tokens WHERE user_id = $1`

	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user tokens: %w", err)
	}

	return nil
}

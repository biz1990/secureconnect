package cockroach

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"secureconnect-backend/internal/domain"
)

// KeysRepository handles E2EE keys storage in CockroachDB
// Implements Signal Protocol key management
type KeysRepository struct {
	pool *pgxpool.Pool
}

// NewKeysRepository creates a new KeysRepository
func NewKeysRepository(pool *pgxpool.Pool) *KeysRepository {
	return &KeysRepository{pool: pool}
}

// SaveIdentityKey stores user's identity key (long-term Ed25519)
func (r *KeysRepository) SaveIdentityKey(ctx context.Context, key *domain.IdentityKey) error {
	query := `
		INSERT INTO identity_keys (user_id, public_key_ed25519) 
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET public_key_ed25519 = EXCLUDED.public_key_ed25519
	`

	_, err := r.pool.Exec(ctx, query, key.UserID, key.PublicKeyEd25519)
	if err != nil {
		return fmt.Errorf("failed to save identity key: %w", err)
	}

	return nil
}

// GetIdentityKey retrieves user's identity key
func (r *KeysRepository) GetIdentityKey(ctx context.Context, userID uuid.UUID) (*domain.IdentityKey, error) {
	query := `SELECT user_id, public_key_ed25519, created_at FROM identity_keys WHERE user_id = $1`

	key := &domain.IdentityKey{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&key.UserID,
		&key.PublicKeyEd25519,
		&key.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("identity key not found")
		}
		return nil, fmt.Errorf("failed to get identity key: %w", err)
	}

	return key, nil
}

// SaveSignedPreKey stores a new signed pre-key
func (r *KeysRepository) SaveSignedPreKey(ctx context.Context, key *domain.SignedPreKey) error {
	query := `
		INSERT INTO signed_pre_keys (key_id, user_id, public_key, signature)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, key_id) DO UPDATE
		SET public_key = EXCLUDED.public_key, signature = EXCLUDED.signature
	`

	_, err := r.pool.Exec(ctx, query, key.KeyID, key.UserID, key.PublicKey, key.Signature)
	if err != nil {
		return fmt.Errorf("failed to save signed pre-key: %w", err)
	}

	return nil
}

// GetLatestSignedPreKey retrieves the most recent signed pre-key
func (r *KeysRepository) GetLatestSignedPreKey(ctx context.Context, userID uuid.UUID) (*domain.SignedPreKey, error) {
	query := `
		SELECT key_id, user_id, public_key, signature, created_at
		FROM signed_pre_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	key := &domain.SignedPreKey{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&key.KeyID,
		&key.UserID,
		&key.PublicKey,
		&key.Signature,
		&key.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("signed pre-key not found")
		}
		return nil, fmt.Errorf("failed to get signed pre-key: %w", err)
	}

	return key, nil
}

// SaveOneTimePreKeys stores multiple one-time pre-keys (batch insert)
func (r *KeysRepository) SaveOneTimePreKeys(ctx context.Context, userID uuid.UUID, keys []domain.OneTimePreKey) error {
	if len(keys) == 0 {
		return nil
	}

	// Use transaction for batch insert
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO one_time_pre_keys (key_id, user_id, public_key, used)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, key_id) DO NOTHING
	`

	for _, key := range keys {
		_, err := tx.Exec(ctx, query, key.KeyID, userID, key.PublicKey, false)
		if err != nil {
			return fmt.Errorf("failed to save one-time pre-key: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetUnusedOneTimePreKey retrieves and marks one-time pre-key as used
// This is atomic operation to prevent race conditions
func (r *KeysRepository) GetUnusedOneTimePreKey(ctx context.Context, userID uuid.UUID) (*domain.OneTimePreKey, error) {
	// Begin transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Select one unused key
	selectQuery := `
		SELECT key_id, user_id, public_key, used, created_at
		FROM one_time_pre_keys
		WHERE user_id = $1 AND used = FALSE
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE
	`

	key := &domain.OneTimePreKey{}
	err = tx.QueryRow(ctx, selectQuery, userID).Scan(
		&key.KeyID,
		&key.UserID,
		&key.PublicKey,
		&key.Used,
		&key.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No unused keys available
		}
		return nil, fmt.Errorf("failed to get one-time pre-key: %w", err)
	}

	// Mark as used
	updateQuery := `UPDATE one_time_pre_keys SET used = TRUE WHERE user_id = $1 AND key_id = $2`
	_, err = tx.Exec(ctx, updateQuery, userID, key.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark key as used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return key, nil
}

// GetPreKeyBundle retrieves complete pre-key bundle for initiating E2EE session
func (r *KeysRepository) GetPreKeyBundle(ctx context.Context, userID uuid.UUID) (*domain.PreKeyBundle, error) {
	bundle := &domain.PreKeyBundle{
		UserID: userID,
	}

	// Get identity key
	identityKey, err := r.GetIdentityKey(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity key: %w", err)
	}
	bundle.IdentityKey = identityKey.PublicKeyEd25519

	// Get signed pre-key
	signedPreKey, err := r.GetLatestSignedPreKey(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signed pre-key: %w", err)
	}
	bundle.SignedPreKey = signedPreKey

	// Get one-time pre-key (may be nil if exhausted)
	oneTimeKey, err := r.GetUnusedOneTimePreKey(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get one-time pre-key: %w", err)
	}
	bundle.OneTimePreKey = oneTimeKey // May be nil

	return bundle, nil
}

// CountUnusedOneTimeKeys returns count of available one-time keys
func (r *KeysRepository) CountUnusedOneTimeKeys(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM one_time_pre_keys WHERE user_id = $1 AND used = FALSE`

	var count int
	err := r.pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unused keys: %w", err)
	}

	return count, nil
}

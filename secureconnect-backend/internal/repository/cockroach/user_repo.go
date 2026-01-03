package cockroach

import (
	"context"
	"fmt"
	
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"secureconnect-backend/internal/domain"
)

// UserRepository handles user data operations in CockroachDB
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a new user
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (user_id, email, username, password_hash, display_name, avatar_url, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	
	err := r.pool.QueryRow(ctx, query,
		user.UserID,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.DisplayName,
		user.AvatarURL,
		user.Status,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	
	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	query := `
		SELECT user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
		FROM users
		WHERE user_id = $1
	`
	
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
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
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
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
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
		FROM users
		WHERE username = $1
	`
	
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, username).Scan(
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
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// Update updates user information
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET display_name = $1, avatar_url = $2, status = $3, updated_at = NOW()
		WHERE user_id = $4
	`
	
	cmdTag, err := r.pool.Exec(ctx, query,
		user.DisplayName,
		user.AvatarURL,
		user.Status,
		user.UserID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	
	return nil
}

// UpdateStatus updates user online status
func (r *UserRepository) UpdateStatus(ctx context.Context, userID uuid.UUID, status string) error {
	query := `
		UPDATE users
		SET status = $1, updated_at = NOW()
		WHERE user_id = $2
	`
	
	_, err := r.pool.Exec(ctx, query, status, userID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	
	return nil
}

// Delete deletes a user (soft delete could be implemented)
func (r *UserRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM users WHERE user_id = $1`
	
	cmdTag, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	
	return nil
}

// EmailExists checks if email already exists
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	
	var exists bool
	err := r.pool.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	
	return exists, nil
}

// UsernameExists checks if username already exists
func (r *UserRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	
	var exists bool
	err := r.pool.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check username existence: %w", err)
	}
	
	return exists, nil
}

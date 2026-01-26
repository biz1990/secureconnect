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

// UsersExist checks if multiple users exist by their IDs
// Returns a map of user ID to existence status, and an error if the query fails
func (r *UserRepository) UsersExist(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(userIDs) == 0 {
		return make(map[uuid.UUID]bool), nil
	}

	query := `SELECT user_id FROM users WHERE user_id = ANY($1)`

	rows, err := r.pool.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to check users existence: %w", err)
	}
	defer rows.Close()

	existingUsers := make(map[uuid.UUID]bool)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		existingUsers[userID] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	// Set false for non-existing users
	for _, id := range userIDs {
		if !existingUsers[id] {
			existingUsers[id] = false
		}
	}

	return existingUsers, nil
}

// GetByIDs retrieves multiple users by their IDs
func (r *UserRepository) GetByIDs(ctx context.Context, userIDs []uuid.UUID) ([]*domain.User, error) {
	if len(userIDs) == 0 {
		return []*domain.User{}, nil
	}

	// Build IN clause
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := `
		SELECT user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
		FROM users
		WHERE user_id = ANY($1)
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by IDs: %w", err)
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

// SearchUsers searches users by username or email
func (r *UserRepository) SearchUsers(ctx context.Context, query string, limit int, offset int) ([]*domain.User, error) {
	searchPattern := "%" + query + "%"

	sqlQuery := `
		SELECT user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
		FROM users
		WHERE (email ILIKE $1 OR username ILIKE $1)
			AND status != 'deleted'
		ORDER BY username ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, sqlQuery, searchPattern, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
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

// GetOnlineUsers retrieves users with online status
func (r *UserRepository) GetOnlineUsers(ctx context.Context, limit int, offset int) ([]*domain.User, error) {
	sqlQuery := `
		SELECT user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
		FROM users
		WHERE status = 'online'
		ORDER BY username ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, sqlQuery, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get online users: %w", err)
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

// GetFriends retrieves user's friends
func (r *UserRepository) GetFriends(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	sqlQuery := `
		SELECT u.user_id, u.email, u.username, u.password_hash, u.display_name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM users u
		INNER JOIN friendships f ON (
			(f.user_id_1 = $1 AND f.user_id_2 = u.user_id)
			OR (f.user_id_2 = $1 AND f.user_id_1 = u.user_id)
		)
		WHERE f.status = 'accepted'
		ORDER BY u.username ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, sqlQuery, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get friends: %w", err)
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

// GetFriendRequests retrieves incoming friend requests
func (r *UserRepository) GetFriendRequests(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	sqlQuery := `
		SELECT u.user_id, u.email, u.username, u.password_hash, u.display_name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM users u
		INNER JOIN friendships f ON f.user_id_1 = u.user_id
		WHERE f.user_id_2 = $1 AND f.status = 'pending'
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, sqlQuery, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get friend requests: %w", err)
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

// GetMutualFriends retrieves mutual friends between two users
func (r *UserRepository) GetMutualFriends(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.User, error) {
	sqlQuery := `
		SELECT u.user_id, u.email, u.username, u.password_hash, u.display_name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM users u
		WHERE u.user_id IN (
			SELECT CASE
				WHEN f1.user_id_1 = $1 THEN f1.user_id_2
				ELSE f1.user_id_1
			END
			FROM friendships f1
			WHERE (f1.user_id_1 = $1 OR f1.user_id_2 = $1) AND f1.status = 'accepted'
			AND EXISTS (
				SELECT 1 FROM friendships f2
				WHERE (f2.user_id_1 = CASE WHEN f1.user_id_1 = $1 THEN f1.user_id_2 ELSE f1.user_id_1 END
					OR f2.user_id_2 = CASE WHEN f1.user_id_1 = $1 THEN f1.user_id_2 ELSE f1.user_id_1 END)
					AND f2.status = 'accepted'
					AND (f2.user_id_1 = $1 OR f2.user_id_2 = $1)
			)
		)
		ORDER BY u.username ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, sqlQuery, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get mutual friends: %w", err)
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

// CreateFriendRequest creates a new friend request
func (r *UserRepository) CreateFriendRequest(ctx context.Context, requestingUserID, targetUserID uuid.UUID) error {
	query := `
		INSERT INTO friendships (user_id_1, user_id_2, status, created_at, updated_at)
		VALUES ($1, $2, 'pending', NOW(), NOW())
		ON CONFLICT (user_id_1, user_id_2) DO NOTHING
	`

	_, err := r.pool.Exec(ctx, query, requestingUserID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to create friend request: %w", err)
	}

	return nil
}

// UpdateFriendshipStatus updates friendship status
func (r *UserRepository) UpdateFriendshipStatus(ctx context.Context, userID, friendID uuid.UUID, status string) error {
	query := `
		UPDATE friendships
		SET status = $1, updated_at = NOW()
		WHERE ((user_id_1 = $2 AND user_id_2 = $3) OR (user_id_1 = $3 AND user_id_2 = $2))
		RETURNING user_id_1, user_id_2
	`

	var id1, id2 uuid.UUID
	err := r.pool.QueryRow(ctx, query, status, userID, friendID).Scan(&id1, &id2)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("friendship not found")
		}
		return fmt.Errorf("failed to update friendship status: %w", err)
	}

	return nil
}

// DeleteFriendship removes a friendship
func (r *UserRepository) DeleteFriendship(ctx context.Context, userID, friendID uuid.UUID) error {
	query := `
		DELETE FROM friendships
		WHERE ((user_id_1 = $1 AND user_id_2 = $2) OR (user_id_1 = $2 AND user_id_2 = $1))
		RETURNING user_id_1, user_id_2
	`

	var id1, id2 uuid.UUID
	err := r.pool.QueryRow(ctx, query, userID, friendID).Scan(&id1, &id2)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("friendship not found")
		}
		return fmt.Errorf("failed to delete friendship: %w", err)
	}

	return nil
}

// GetFriendship retrieves friendship status between two users
func (r *UserRepository) GetFriendship(ctx context.Context, userID, friendID uuid.UUID) (string, error) {
	query := `
		SELECT status
		FROM friendships
		WHERE ((user_id_1 = $1 AND user_id_2 = $2) OR (user_id_1 = $2 AND user_id_2 = $1))
	`

	var status string
	err := r.pool.QueryRow(ctx, query, userID, friendID).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil // No friendship exists
		}
		return "", fmt.Errorf("failed to get friendship: %w", err)
	}

	return status, nil
}

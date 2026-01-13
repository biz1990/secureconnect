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

// FileRepository handles file metadata operations
type FileRepository struct {
	pool *pgxpool.Pool
}

// NewFileRepository creates a new file repository
func NewFileRepository(pool *pgxpool.Pool) *FileRepository {
	return &FileRepository{pool: pool}
}

// Create creates a new file metadata record
func (r *FileRepository) Create(ctx context.Context, file *domain.File) error {
	query := `
		INSERT INTO files (
			file_id, user_id, file_name, file_size, content_type,
			minio_object_key, is_encrypted, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.pool.Exec(ctx, query,
		file.FileID,
		file.UserID,
		file.FileName,
		file.FileSize,
		file.ContentType,
		file.MinIOObjectKey,
		file.IsEncrypted,
		file.Status,
		file.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	return nil
}

// GetByID retrieves a file by ID
func (r *FileRepository) GetByID(ctx context.Context, fileID uuid.UUID) (*domain.File, error) {
	query := `
		SELECT file_id, user_id, file_name, file_size, content_type,
		       minio_object_key, is_encrypted, status, created_at
		FROM files
		WHERE file_id = $1
	`

	file := &domain.File{}
	err := r.pool.QueryRow(ctx, query, fileID).Scan(
		&file.FileID,
		&file.UserID,
		&file.FileName,
		&file.FileSize,
		&file.ContentType,
		&file.MinIOObjectKey,
		&file.IsEncrypted,
		&file.Status,
		&file.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return file, nil
}

// UpdateStatus updates file status
func (r *FileRepository) UpdateStatus(ctx context.Context, fileID uuid.UUID, status string) error {
	query := `
		UPDATE files
		SET status = $2, updated_at = $3
		WHERE file_id = $1
	`

	_, err := r.pool.Exec(ctx, query, fileID, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	return nil
}

// GetUserFiles retrieves all files for a user
func (r *FileRepository) GetUserFiles(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.File, error) {
	query := `
		SELECT file_id, user_id, file_name, file_size, content_type,
		       minio_object_key, is_encrypted, status, created_at
		FROM files
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user files: %w", err)
	}
	defer rows.Close()

	var files []*domain.File
	for rows.Next() {
		file := &domain.File{}
		err := rows.Scan(
			&file.FileID,
			&file.UserID,
			&file.FileName,
			&file.FileSize,
			&file.ContentType,
			&file.MinIOObjectKey,
			&file.IsEncrypted,
			&file.Status,
			&file.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		files = append(files, file)
	}

	return files, nil
}

// GetUserStorageUsage calculates total storage used by user
func (r *FileRepository) GetUserStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(file_size), 0)
		FROM files
		WHERE user_id = $1 AND status = 'completed'
	`

	var totalSize int64
	err := r.pool.QueryRow(ctx, query, userID).Scan(&totalSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get storage usage: %w", err)
	}

	return totalSize, nil
}

// Delete deletes a file metadata record
func (r *FileRepository) Delete(ctx context.Context, fileID uuid.UUID) error {
	query := `DELETE FROM files WHERE file_id = $1`

	_, err := r.pool.Exec(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// CheckFileAccess checks if a user has access to a file via conversation
func (r *FileRepository) CheckFileAccess(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
	// Get the file to check ownership
	file, err := r.GetByID(ctx, fileID)
	if err != nil {
		return false, fmt.Errorf("file not found: %w", err)
	}

	// Check if user owns the file
	if file.UserID != userID {
		return false, nil
	}

	// Check if user is a participant in the conversation that contains this file
	// This would require joining with the conversation repository, which is complex
	// For now, we'll return true if user owns the file
	return true, nil
}

package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/resilience"
)

// FileRepository interface
type FileRepository interface {
	Create(ctx context.Context, file *domain.File) error
	GetByID(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
	UpdateStatus(ctx context.Context, fileID uuid.UUID, status string) error
	GetUserStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error)
	CheckFileAccess(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (bool, error)
	GetExpiredUploads(ctx context.Context, expiryDuration time.Duration) ([]*domain.File, error)
}

// ObjectStorage interface
type ObjectStorage interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error)
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

// MinioAdapter implements ObjectStorage
type MinioAdapter struct {
	Client *minio.Client
}

func (m *MinioAdapter) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return m.Client.BucketExists(ctx, bucketName)
}

func (m *MinioAdapter) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return m.Client.MakeBucket(ctx, bucketName, opts)
}

func (m *MinioAdapter) PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
	return m.Client.PresignedPutObject(ctx, bucketName, objectName, expires)
}

func (m *MinioAdapter) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	return m.Client.PresignedGetObject(ctx, bucketName, objectName, expires, reqParams)
}

func (m *MinioAdapter) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return m.Client.RemoveObject(ctx, bucketName, objectName, opts)
}

// Service handles file storage operations
type Service struct {
	storage    ObjectStorage
	bucketName string
	fileRepo   FileRepository
	resilience *resilience.MinIOResilience
}

// NewService creates a new storage service
func NewService(storage ObjectStorage, bucketName string, fileRepo FileRepository) (*Service, error) {
	// Ensure bucket exists with resilience
	ctx := context.Background()

	// Initialize resilience layer
	resilienceLayer := resilience.NewMinIOResilience()

	var exists bool
	err := resilienceLayer.Execute(ctx, "bucket_exists", func() error {
		var err error
		exists, err = storage.BucketExists(ctx, bucketName)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}

	if !exists {
		err = resilienceLayer.Execute(ctx, "create_bucket", func() error {
			return storage.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &Service{
		storage:    storage,
		bucketName: bucketName,
		fileRepo:   fileRepo,
		resilience: resilienceLayer,
	}, nil
}

// GenerateUploadURLInput contains file upload request
type GenerateUploadURLInput struct {
	FileName    string
	FileSize    int64
	ContentType string
	IsEncrypted bool
}

// GenerateUploadURLOutput contains presigned upload URL
type GenerateUploadURLOutput struct {
	FileID    uuid.UUID
	UploadURL string
	ExpiresAt time.Time
}

// GenerateUploadURL creates presigned URL for file upload
func (s *Service) GenerateUploadURL(ctx context.Context, userID uuid.UUID, input *GenerateUploadURLInput) (*GenerateUploadURLOutput, error) {
	// Check storage quota before allowing upload
	used, quota, err := s.GetUserQuota(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check storage quota: %w", err)
	}

	// Calculate new total after upload
	newTotal := used + input.FileSize
	if newTotal > quota {
		return nil, fmt.Errorf("storage quota exceeded: %d bytes used, %d bytes quota, %d bytes requested",
			used, quota, input.FileSize)
	}

	// Generate file ID
	fileID := uuid.New()

	// Generate object key (path in MinIO)
	objectKey := fmt.Sprintf("users/%s/%s", userID, fileID)

	// Generate presigned URL (valid for 15 minutes) with resilience
	var presignedURL *url.URL
	err = s.resilience.Execute(ctx, "presigned_put_object", func() error {
		var err error
		presignedURL, err = s.storage.PresignedPutObject(ctx, s.bucketName, objectKey, constants.PresignedURLExpiry)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	// Save file metadata to CockroachDB
	file := &domain.File{
		FileID:           fileID,
		UserID:           userID,
		FileName:         input.FileName,
		FileSize:         input.FileSize,
		ContentType:      input.ContentType,
		MinIOObjectKey:   objectKey,
		IsEncrypted:      input.IsEncrypted,
		Status:           "uploading",
		StorageQuotaUsed: input.FileSize,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.fileRepo.Create(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	return &GenerateUploadURLOutput{
		FileID:    fileID,
		UploadURL: presignedURL.String(),
		ExpiresAt: time.Now().Add(constants.PresignedURLExpiry),
	}, nil
}

// CompleteUpload marks file upload as completed
func (s *Service) CompleteUpload(ctx context.Context, fileID uuid.UUID) error {
	return s.fileRepo.UpdateStatus(ctx, fileID, "completed")
}

// GenerateDownloadURL creates presigned URL for file download
func (s *Service) GenerateDownloadURL(ctx context.Context, userID, fileID uuid.UUID) (string, error) {
	// Fetch file metadata from CockroachDB
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// Verify user owns the file
	if file.UserID != userID {
		return "", fmt.Errorf("unauthorized access to file")
	}

	// Generate presigned download URL (valid for 1 hour) with resilience
	var presignedURL *url.URL
	err = s.resilience.Execute(ctx, "presigned_get_object", func() error {
		var err error
		presignedURL, err = s.storage.PresignedGetObject(ctx, s.bucketName, file.MinIOObjectKey, time.Hour, nil)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %w", err)
	}

	return presignedURL.String(), nil
}

// DeleteFile removes file from storage
func (s *Service) DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	// Get file metadata
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify ownership
	if file.UserID != userID {
		return fmt.Errorf("unauthorized access to file")
	}

	// Remove from MinIO with resilience
	err = s.resilience.Execute(ctx, "remove_object", func() error {
		return s.storage.RemoveObject(ctx, s.bucketName, file.MinIOObjectKey, minio.RemoveObjectOptions{})
	})
	if err != nil {
		return fmt.Errorf("failed to delete file from storage: %w", err)
	}

	// Update status in database
	err = s.fileRepo.UpdateStatus(ctx, fileID, "deleted")
	if err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	return nil
}

// GetUserQuota returns user's storage usage
func (s *Service) GetUserQuota(ctx context.Context, userID uuid.UUID) (int64, int64, error) {
	used, err := s.fileRepo.GetUserStorageUsage(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get storage usage: %w", err)
	}

	// Default quota: 10GB
	const defaultQuota int64 = 10 * 1024 * 1024 * 1024

	return used, defaultQuota, nil
}

// CleanupExpiredUploads removes files stuck in "uploading" status for longer than expiry
// This should be called periodically (e.g., every hour) to clean up orphaned uploads
func (s *Service) CleanupExpiredUploads(ctx context.Context) (int, error) {
	// Use presigned URL expiry as threshold for expired uploads
	expiryDuration := constants.PresignedURLExpiry

	// Get expired uploads
	expiredUploads, err := s.fileRepo.GetExpiredUploads(ctx, expiryDuration)
	if err != nil {
		return 0, fmt.Errorf("failed to get expired uploads: %w", err)
	}

	cleanedCount := 0
	for _, file := range expiredUploads {
		// Attempt to remove from MinIO with resilience (may fail if file was never uploaded)
		err := s.resilience.Execute(ctx, "remove_expired_object", func() error {
			return s.storage.RemoveObject(ctx, s.bucketName, file.MinIOObjectKey, minio.RemoveObjectOptions{})
		})
		if err != nil {
			logger.Warn("Failed to remove expired upload from MinIO",
				zap.String("fileID", file.FileID.String()),
				zap.String("objectKey", file.MinIOObjectKey),
				zap.Error(err))
		}

		// Update status to "failed" to mark as cleaned up
		err = s.fileRepo.UpdateStatus(ctx, file.FileID, "failed")
		if err != nil {
			logger.Warn("Failed to update expired upload status",
				zap.Error(err))
			continue
		}

		cleanedCount++
		logger.Info("Cleaned up expired upload",
			zap.String("fileID", file.FileID.String()),
			zap.String("userID", file.UserID.String()),
			zap.String("fileName", file.FileName),
			zap.Duration("age", time.Since(file.CreatedAt)))
	}

	return cleanedCount, nil
}

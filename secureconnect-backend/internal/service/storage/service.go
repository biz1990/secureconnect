package storage

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	
	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
)

// Service handles file storage operations
type Service struct {
	minioClient *minio.Client
	bucketName  string
	fileRepo    *cockroach.FileRepository
}

// NewService creates a new storage service
func NewService(endpoint, accessKey, secretKey, bucketName string, fileRepo *cockroach.FileRepository) (*Service, error) {
	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // Use HTTPS in production
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}
	
	// Ensure bucket exists
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	
	return &Service{
		minioClient: minioClient,
		bucketName:  bucketName,
		fileRepo:    fileRepo,
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
	// Generate file ID
	fileID := uuid.New()
	
	// Generate object key (path in MinIO)
	objectKey := fmt.Sprintf("users/%s/%s", userID, fileID)
	
	// Generate presigned URL (valid for 15 minutes)
	presignedURL, err := s.minioClient.PresignedPutObject(ctx, s.bucketName, objectKey, 15*time.Minute)
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
		ExpiresAt: time.Now().Add(15 * time.Minute),
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
	
	// Generate presigned download URL (valid for 1 hour)
	presignedURL, err := s.minioClient.PresignedGetObject(ctx, s.bucketName, file.MinIOObjectKey, time.Hour, nil)
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
		return fmt.Errorf("unauthorized")
	}
	
	// Remove from MinIO
	err = s.minioClient.RemoveObject(ctx, s.bucketName, file.MinIOObjectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file from storage: %w", err)
	}
	
	// Update status in database
	if err := s.fileRepo.UpdateStatus(ctx, fileID, "deleted"); err != nil {
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

package domain

import (
	"time"

	"github.com/google/uuid"
)

// File represents file metadata stored in the system
// Actual file content is stored in MinIO
// Maps to CockroachDB files table
type File struct {
	FileID             uuid.UUID              `json:"file_id" db:"file_id"`
	UserID             uuid.UUID              `json:"user_id" db:"user_id"`
	FileName           string                 `json:"file_name" db:"file_name"`
	FileSize           int64                  `json:"file_size" db:"file_size"` // Bytes
	ContentType        string                 `json:"content_type" db:"content_type"`
	MinIOObjectKey     string                 `json:"-" db:"minio_object_key"`                                // Internal, don't expose
	IsEncrypted        bool                   `json:"is_encrypted" db:"is_encrypted"`                         // Client-side encryption
	EncryptionMetadata map[string]interface{} `json:"encryption_metadata,omitempty" db:"encryption_metadata"` // Client encryption info
	Status             string                 `json:"status" db:"status"`                                     // uploading, completed, deleted
	StorageQuotaUsed   int64                  `json:"storage_quota_used" db:"storage_quota_used"`             // Bytes counted against quota
	CreatedAt          time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at" db:"updated_at"`
	DeletedAt          *time.Time             `json:"deleted_at,omitempty" db:"deleted_at"` // Soft delete
}

// FileUploadURLRequest represents request for presigned upload URL
type FileUploadURLRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required,min=1"`
	ContentType string `json:"content_type" binding:"required"`
	IsEncrypted bool   `json:"is_encrypted"` // Indicates client-side encryption
}

// FileUploadURLResponse contains the presigned URL for upload
type FileUploadURLResponse struct {
	FileID    uuid.UUID `json:"file_id"`
	UploadURL string    `json:"upload_url"` // MinIO presigned PUT URL
	ExpiresAt time.Time `json:"expires_at"` // URL expiration
}

// FileUploadCompleteRequest marks upload as complete
type FileUploadCompleteRequest struct {
	FileID uuid.UUID `json:"file_id" binding:"required"`
}

// FileDownloadURLResponse contains presigned download URL
type FileDownloadURLResponse struct {
	DownloadURL string    `json:"download_url"` // MinIO presigned GET URL
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	ContentType string    `json:"content_type"`
	IsEncrypted bool      `json:"is_encrypted"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// UserStorageQuota represents user's storage usage
type UserStorageQuota struct {
	UserID      uuid.UUID `json:"user_id"`
	TotalUsed   int64     `json:"total_used"`  // Bytes
	QuotaLimit  int64     `json:"quota_limit"` // Bytes, based on subscription plan
	FileCount   int       `json:"file_count"`
	PercentUsed float64   `json:"percent_used"`
}

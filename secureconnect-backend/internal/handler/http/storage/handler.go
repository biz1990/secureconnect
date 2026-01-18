package storage

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"secureconnect-backend/internal/service/storage"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/response"
	"secureconnect-backend/pkg/sanitize"
)

// Metrics for storage service validation
var (
	storageUploadRejectedSizeExceeded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "storage_upload_rejected_size_exceeded_total",
		Help: "Total number of upload requests rejected due to file size exceeding limit",
	})

	storageUploadRejectedInvalidMIME = promauto.NewCounter(prometheus.CounterOpts{
		Name: "storage_upload_rejected_invalid_mime_total",
		Help: "Total number of upload requests rejected due to invalid MIME type",
	})

	storageUploadRejectedInvalidFilename = promauto.NewCounter(prometheus.CounterOpts{
		Name: "storage_upload_rejected_invalid_filename_total",
		Help: "Total number of upload requests rejected due to invalid filename",
	})

	storageUploadSizeBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "storage_upload_size_bytes",
		Help:    "Histogram of uploaded file sizes in bytes",
		Buckets: []float64{1024, 10240, 102400, 1048576, 10485760, 52428800}, // 1KB, 10KB, 100KB, 1MB, 10MB, 50MB
	})

	storageUploadByMIMEType = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "storage_upload_by_mime_type_total",
		Help: "Total number of uploads by MIME type",
	}, []string{"mime_type"})

	storageCleanupExpiredUploadsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "storage_cleanup_expired_uploads_total",
		Help: "Total number of expired uploads cleaned up",
	})

	storageCleanupFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "storage_cleanup_failed_total",
		Help: "Total number of cleanup operations that failed",
	})

	storageCleanupDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "storage_cleanup_duration_seconds",
		Help:    "Duration of cleanup operations",
		Buckets: []float64{0.1, 0.5, 1, 5, 10, 30},
	})
)

// Handler handles storage HTTP requests
type Handler struct {
	storageService *storage.Service
}

// NewHandler creates a new storage handler
func NewHandler(storageService *storage.Service) *Handler {
	return &Handler{
		storageService: storageService,
	}
}

// GenerateUploadURLRequest represents upload URL request
type GenerateUploadURLRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required,min=1"`
	ContentType string `json:"content_type" binding:"required"`
	IsEncrypted bool   `json:"is_encrypted"`
}

// GenerateUploadURL creates presigned upload URL
// POST /v1/storage/upload-url
func (h *Handler) GenerateUploadURL(c *gin.Context) {
	var req GenerateUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// VALIDATION #1: File size validation - enforce MaxAttachmentSize
	if req.FileSize > constants.MaxAttachmentSize {
		storageUploadRejectedSizeExceeded.Inc()
		response.ValidationError(c, fmt.Sprintf("File size exceeds maximum allowed size of %d MB", constants.MaxAttachmentSize/(1024*1024)))
		return
	}

	// Record upload size in histogram
	storageUploadSizeBytes.Observe(float64(req.FileSize))

	// VALIDATION #2: MIME type validation - enforce allowlist
	if !constants.AllowedMIMETypes[req.ContentType] {
		storageUploadRejectedInvalidMIME.Inc()
		response.ValidationError(c, "Invalid content type: "+req.ContentType)
		return
	}

	// Record upload by MIME type
	storageUploadByMIMEType.WithLabelValues(req.ContentType).Inc()

	// VALIDATION #3: File name sanitization - prevent path traversal
	sanitizedFileName := sanitize.SanitizeFilename(req.FileName)
	if sanitizedFileName == "" {
		storageUploadRejectedInvalidFilename.Inc()
		response.ValidationError(c, "Invalid file name: file name cannot be empty after sanitization")
		return
	}
	// Additional check to ensure no path traversal characters remain
	if containsPathTraversal(sanitizedFileName) {
		storageUploadRejectedInvalidFilename.Inc()
		response.ValidationError(c, "Invalid file name: contains path traversal characters")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Call service with sanitized filename
	output, err := h.storageService.GenerateUploadURL(c.Request.Context(), userID, &storage.GenerateUploadURLInput{
		FileName:    sanitizedFileName,
		FileSize:    req.FileSize,
		ContentType: req.ContentType,
		IsEncrypted: req.IsEncrypted,
	})

	if err != nil {
		response.InternalError(c, "Failed to generate upload URL")
		return
	}

	response.Success(c, http.StatusOK, output)
}

// containsPathTraversal checks if a filename contains path traversal patterns
func containsPathTraversal(filename string) bool {
	// Check for common path traversal patterns
	traversalPatterns := []string{"../", "./", "..\\", ".\\", "..", "\\", "/"}
	for _, pattern := range traversalPatterns {
		if pattern == ".." {
			// Only reject standalone ".." at the end of the filename
			if filename == ".." || filename == "../" || filename == "..\\" {
				return true
			}
			continue
		}
		if len(filename) >= len(pattern) && filename[:len(pattern)] == pattern {
			return true
		}
		if len(filename) >= len(pattern) && filename[len(filename)-len(pattern):] == pattern {
			return true
		}
	}
	return false
}

// GenerateDownloadURL creates presigned download URL
// GET /v1/storage/download-url/:file_id
func (h *Handler) GenerateDownloadURL(c *gin.Context) {
	fileIDParam := c.Param("file_id")

	fileID, err := uuid.Parse(fileIDParam)
	if err != nil {
		response.ValidationError(c, "Invalid file ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Call service
	downloadURL, err := h.storageService.GenerateDownloadURL(c.Request.Context(), userID, fileID)
	if err != nil {
		response.NotFound(c, "File not found")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"download_url": downloadURL,
	})
}

// DeleteFile removes a file
// DELETE /v1/storage/files/:file_id
func (h *Handler) DeleteFile(c *gin.Context) {
	fileIDParam := c.Param("file_id")

	fileID, err := uuid.Parse(fileIDParam)
	if err != nil {
		response.ValidationError(c, "Invalid file ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Call service
	if err := h.storageService.DeleteFile(c.Request.Context(), userID, fileID); err != nil {
		response.InternalError(c, "Failed to delete file")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "File deleted successfully",
	})
}

// CompleteUpload marks file upload as completed
// POST /v1/storage/upload-complete
func (h *Handler) CompleteUpload(c *gin.Context) {
	var req struct {
		FileID string `json:"file_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	fileID, err := uuid.Parse(req.FileID)
	if err != nil {
		response.ValidationError(c, "Invalid file ID")
		return
	}

	if err := h.storageService.CompleteUpload(c.Request.Context(), fileID); err != nil {
		response.InternalError(c, "Failed to complete upload")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Upload completed",
	})
}

// GetQuota returns user's storage quota
// GET /v1/storage/quota
func (h *Handler) GetQuota(c *gin.Context) {
	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	used, total, err := h.storageService.GetUserQuota(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to get quota")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"used":             used,
		"total":            total,
		"available":        total - used,
		"usage_percentage": float64(used) / float64(total) * 100,
	})
}

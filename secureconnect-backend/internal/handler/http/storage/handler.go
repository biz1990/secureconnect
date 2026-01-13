package storage

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/service/storage"
	"secureconnect-backend/pkg/response"
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
	output, err := h.storageService.GenerateUploadURL(c.Request.Context(), userID, &storage.GenerateUploadURLInput{
		FileName:    req.FileName,
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

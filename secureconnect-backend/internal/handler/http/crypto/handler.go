package crypto

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/service/crypto"
	"secureconnect-backend/pkg/response"
)

// Handler handles E2EE crypto HTTP requests
type Handler struct {
	cryptoService *crypto.Service
}

// NewHandler creates a new crypto handler
func NewHandler(cryptoService *crypto.Service) *Handler {
	return &Handler{
		cryptoService: cryptoService,
	}
}

// UploadKeysRequest represents keys upload request
type UploadKeysRequest struct {
	IdentityKey    string                    `json:"identity_key" binding:"required"`
	SignedPreKey   domain.SignedPreKeyUpload `json:"signed_pre_key" binding:"required"`
	OneTimePreKeys []domain.OneTimeKeyUpload `json:"one_time_pre_keys" binding:"required,min=20,max=100"`
}

// UploadKeys handles public keys upload
// POST /v1/keys/upload
func (h *Handler) UploadKeys(c *gin.Context) {
	var req UploadKeysRequest
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
	err := h.cryptoService.UploadKeys(c.Request.Context(), &crypto.UploadKeysInput{
		UserID:         userID,
		IdentityKey:    req.IdentityKey,
		SignedPreKey:   req.SignedPreKey,
		OneTimePreKeys: req.OneTimePreKeys,
	})

	if err != nil {
		response.InternalError(c, "Failed to upload keys")
		return
	}

	response.Success(c, http.StatusCreated, gin.H{
		"message":       "Keys uploaded successfully",
		"one_time_keys": len(req.OneTimePreKeys),
	})
}

// GetPreKeyBundle retrieves public keys for a user
// GET /v1/keys/:user_id
func (h *Handler) GetPreKeyBundle(c *gin.Context) {
	userIDParam := c.Param("user_id")

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		response.ValidationError(c, "Invalid user ID")
		return
	}

	// Call service
	bundle, err := h.cryptoService.GetPreKeyBundle(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "Pre-key bundle not found")
		return
	}

	response.Success(c, http.StatusOK, bundle)
}

// RotateKeys handles signed pre-key rotation
// POST /v1/keys/rotate
func (h *Handler) RotateKeys(c *gin.Context) {
	var req struct {
		NewSignedPreKey domain.SignedPreKeyUpload `json:"new_signed_pre_key" binding:"required"`
		NewOneTimeKeys  []domain.OneTimeKeyUpload `json:"new_one_time_keys"`
	}

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
	err := h.cryptoService.RotateSignedPreKey(c.Request.Context(), userID, req.NewSignedPreKey, req.NewOneTimeKeys)
	if err != nil {
		response.InternalError(c, "Failed to rotate keys")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Keys rotated successfully",
	})
}

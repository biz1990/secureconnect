package domain

import (
	"time"
	
	"github.com/google/uuid"
)

// IdentityKey represents a user's long-term identity key pair (Ed25519)
// Only public key is stored on server - private key never leaves client
// Maps to CockroachDB identity_keys table
type IdentityKey struct {
	UserID            uuid.UUID `json:"user_id" db:"user_id"`
	PublicKeyEd25519  string    `json:"public_key_ed25519" db:"public_key_ed25519"` // Base64 encoded
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// SignedPreKey represents a medium-term pre-key (X25519)
// Rotated every 7 days according to spec
// Maps to CockroachDB signed_pre_keys table
type SignedPreKey struct {
	KeyID     int       `json:"key_id" db:"key_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	PublicKey string    `json:"public_key" db:"public_key"` // Base64 encoded X25519 public key
	Signature string    `json:"signature" db:"signature"` // Signed by identity key
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// OneTimePreKey represents a one-time use pre-key
// Used for X3DH key agreement, consumed after first use
// Maps to CockroachDB one_time_pre_keys table
type OneTimePreKey struct {
	KeyID     int       `json:"key_id" db:"key_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	PublicKey string    `json:"public_key" db:"public_key"` // Base64 encoded X25519 public key
	Used      bool      `json:"used" db:"used"` // Mark as true after retrieval
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// PreKeyBundle represents the complete set of public keys needed to initiate E2EE session
// Returned by GET /v1/keys/{user_id} endpoint
type PreKeyBundle struct {
	UserID          uuid.UUID     `json:"user_id"`
	IdentityKey     string        `json:"identity_key"` // Ed25519 public key
	SignedPreKey    *SignedPreKey `json:"signed_pre_key"`
	OneTimePreKey   *OneTimePreKey `json:"one_time_pre_key,omitempty"` // May be nil if exhausted
}

// KeysUploadRequest represents keys uploaded by client during registration or rotation
type KeysUploadRequest struct {
	IdentityKey    string              `json:"identity_key" binding:"required"`
	SignedPreKey   SignedPreKeyUpload  `json:"signed_pre_key" binding:"required"`
	OneTimePreKeys []OneTimeKeyUpload  `json:"one_time_pre_keys" binding:"required,min=20,max=100"`
}

// SignedPreKeyUpload is the signed pre-key data from client
type SignedPreKeyUpload struct {
	KeyID     int    `json:"key_id" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
	Signature string `json:"signature" binding:"required"`
}

// OneTimeKeyUpload is one-time pre-key data from client
type OneTimeKeyUpload struct {
	KeyID     int    `json:"key_id" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
}

// KeyRotationRequest represents signed pre-key rotation (every 7 days)
type KeyRotationRequest struct {
	NewSignedPreKey SignedPreKeyUpload `json:"new_signed_pre_key" binding:"required"`
	NewOneTimeKeys  []OneTimeKeyUpload `json:"new_one_time_keys,omitempty"`
}

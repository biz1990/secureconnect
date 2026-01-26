package crypto

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
)

// Service handles E2EE cryptography operations
type Service struct {
	keysRepo *cockroach.KeysRepository
}

// NewService creates a new crypto service
func NewService(keysRepo *cockroach.KeysRepository) *Service {
	return &Service{
		keysRepo: keysRepo,
	}
}

// UploadKeysInput contains public keys for upload
type UploadKeysInput struct {
	UserID         uuid.UUID
	IdentityKey    string
	SignedPreKey   domain.SignedPreKeyUpload
	OneTimePreKeys []domain.OneTimeKeyUpload
}

// UploadKeys stores user's public keys on server
func (s *Service) UploadKeys(ctx context.Context, input *UploadKeysInput) error {
	// 1. Save identity key
	identityKey := &domain.IdentityKey{
		UserID:           input.UserID,
		PublicKeyEd25519: input.IdentityKey,
	}

	if err := s.keysRepo.SaveIdentityKey(ctx, identityKey); err != nil {
		return fmt.Errorf("failed to save identity key: %w", err)
	}

	// 2. Save signed pre-key
	signedPreKey := &domain.SignedPreKey{
		KeyID:     input.SignedPreKey.KeyID,
		UserID:    input.UserID,
		PublicKey: input.SignedPreKey.PublicKey,
		Signature: input.SignedPreKey.Signature,
	}

	if err := s.keysRepo.SaveSignedPreKey(ctx, signedPreKey); err != nil {
		return fmt.Errorf("failed to save signed pre-key: %w", err)
	}

	// 3. Save one-time pre-keys
	oneTimeKeys := make([]domain.OneTimePreKey, len(input.OneTimePreKeys))
	for i, key := range input.OneTimePreKeys {
		oneTimeKeys[i] = domain.OneTimePreKey{
			KeyID:     key.KeyID,
			UserID:    input.UserID,
			PublicKey: key.PublicKey,
			Used:      false,
		}
	}

	if err := s.keysRepo.SaveOneTimePreKeys(ctx, input.UserID, oneTimeKeys); err != nil {
		return fmt.Errorf("failed to save one-time pre-keys: %w", err)
	}

	return nil
}

// GetPreKeyBundle retrieves public keys for initiating E2EE session
func (s *Service) GetPreKeyBundle(ctx context.Context, userID uuid.UUID) (*domain.PreKeyBundle, error) {
	return s.keysRepo.GetPreKeyBundle(ctx, userID)
}

// RotateSignedPreKey replaces old signed pre-key with new one
func (s *Service) RotateSignedPreKey(ctx context.Context, userID uuid.UUID, newKey domain.SignedPreKeyUpload, newOneTimeKeys []domain.OneTimeKeyUpload) error {
	// Save new signed pre-key
	signedPreKey := &domain.SignedPreKey{
		KeyID:     newKey.KeyID,
		UserID:    userID,
		PublicKey: newKey.PublicKey,
		Signature: newKey.Signature,
	}

	if err := s.keysRepo.SaveSignedPreKey(ctx, signedPreKey); err != nil {
		return fmt.Errorf("failed to rotate signed pre-key: %w", err)
	}

	// Optionally add new one-time keys
	if len(newOneTimeKeys) > 0 {
		oneTimeKeys := make([]domain.OneTimePreKey, len(newOneTimeKeys))
		for i, key := range newOneTimeKeys {
			oneTimeKeys[i] = domain.OneTimePreKey{
				KeyID:     key.KeyID,
				UserID:    userID,
				PublicKey: key.PublicKey,
				Used:      false,
			}
		}

		if err := s.keysRepo.SaveOneTimePreKeys(ctx, userID, oneTimeKeys); err != nil {
			return fmt.Errorf("failed to save new one-time keys: %w", err)
		}
	}

	return nil
}

// CountAvailableKeys returns count of unused one-time keys
func (s *Service) CountAvailableKeys(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.keysRepo.CountUnusedOneTimeKeys(ctx, userID)
}

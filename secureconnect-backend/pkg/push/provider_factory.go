package push

import (
	"fmt"

	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/logger"

	"go.uber.org/zap"
)

// ProviderType represents the type of push notification provider
type ProviderType string

const (
	ProviderTypeMock ProviderType = "mock"
	ProviderTypeFCM  ProviderType = "fcm"
	ProviderTypeAPNs ProviderType = "apns"
)

// NewProvider creates a push notification provider based on environment configuration
// Returns the appropriate provider based on PUSH_PROVIDER environment variable
func NewProvider() (Provider, error) {
	providerType := ProviderType(env.GetString("PUSH_PROVIDER", "mock"))

	logger.Info("Initializing push notification provider",
		zap.String("provider_type", string(providerType)))

	switch providerType {
	case ProviderTypeFCM:
		return newFCMProvider()
	case ProviderTypeAPNs:
		return newAPNsProvider()
	case ProviderTypeMock:
		return newMockProvider()
	default:
		logger.Warn("Unknown push provider type, falling back to mock",
			zap.String("provider_type", string(providerType)))
		return newMockProvider()
	}
}

// newFCMProvider creates a new FCM provider from environment configuration
func newFCMProvider() (Provider, error) {
	projectID := env.GetString("FCM_PROJECT_ID", "")
	credentialsPath := env.GetString("FCM_CREDENTIALS_PATH", "")

	if projectID == "" {
		return nil, fmt.Errorf("FCM_PROJECT_ID environment variable is required for FCM provider")
	}

	config := &FCMConfig{
		ProjectID:       projectID,
		CredentialsPath: credentialsPath,
	}

	return NewFCMProvider(config)
}

// newAPNsProvider creates a new APNs provider from environment configuration
func newAPNsProvider() (Provider, error) {
	bundleID := env.GetString("APNS_BUNDLE_ID", "")
	keyPath := env.GetString("APNS_KEY_PATH", "")
	keyID := env.GetString("APNS_KEY_ID", "")
	teamID := env.GetString("APNS_TEAM_ID", "")
	certificatePath := env.GetString("APNS_CERT_PATH", "")
	certificatePassword := env.GetString("APNS_CERT_PASSWORD", "")
	production := env.GetBool("APNS_PRODUCTION", false)

	if bundleID == "" {
		return nil, fmt.Errorf("APNS_BUNDLE_ID environment variable is required for APNs provider")
	}

	// Prefer token-based authentication
	if keyPath != "" && keyID != "" && teamID != "" {
		config := &APNsConfig{
			BundleID:   bundleID,
			KeyPath:    keyPath,
			KeyID:      keyID,
			TeamID:     teamID,
			Production: production,
		}
		return NewAPNsProvider(config)
	}

	// Fallback to certificate-based authentication
	if certificatePath != "" {
		config := &APNsConfig{
			BundleID:            bundleID,
			CertificatePath:     certificatePath,
			CertificatePassword: certificatePassword,
			Production:          production,
		}
		return NewAPNsProvider(config)
	}

	return nil, fmt.Errorf("either token-based (APNS_KEY_PATH, APNS_KEY_ID, APNS_TEAM_ID) or certificate-based (APNS_CERT_PATH) authentication must be provided for APNs provider")
}

// newMockProvider creates a new mock provider
func newMockProvider() (Provider, error) {
	logger.Info("Using mock push notification provider")
	return &MockProvider{}, nil
}

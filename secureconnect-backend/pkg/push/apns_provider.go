package push

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"

	"secureconnect-backend/pkg/logger"

	"go.uber.org/zap"
)

// APNsProvider implements Provider interface for Apple Push Notification Service
type APNsProvider struct {
	client     *apns2.Client
	production bool
	bundleID   string
	teamID     string
	keyID      string
}

// APNsConfig contains configuration for APNs provider
type APNsConfig struct {
	// Certificate-based authentication (legacy)
	CertificatePath     string // Path to .p12 or .pem certificate file
	CertificatePassword string // Password for .p12 certificate

	// Token-based authentication (recommended)
	KeyPath string // Path to .p8 private key file
	KeyID   string // 10-character Key ID from Apple Developer Portal
	TeamID  string // 10-character Team ID from Apple Developer Portal

	BundleID   string // Bundle ID of the app (e.g., com.example.app)
	Production bool   // Use production APNs endpoint (true) or sandbox (false)
}

// NewAPNsProvider creates a new APNs provider
func NewAPNsProvider(config *APNsConfig) (*APNsProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("APNs config is required")
	}

	if config.BundleID == "" {
		return nil, fmt.Errorf("BundleID is required")
	}

	var client *apns2.Client

	// Prefer token-based authentication
	if config.KeyPath != "" && config.KeyID != "" && config.TeamID != "" {
		// Token-based authentication
		authKey, keyErr := token.AuthKeyFromFile(config.KeyPath)
		if keyErr != nil {
			logger.Error("Failed to load APNs key file",
				zap.Error(keyErr),
				zap.String("key_path", config.KeyPath),
				zap.String("key_id", config.KeyID),
				zap.String("team_id", config.TeamID))
			return nil, fmt.Errorf("failed to load APNs key: %w", keyErr)
		}

		authToken := &token.Token{
			AuthKey: authKey,
			KeyID:   config.KeyID,
			TeamID:  config.TeamID,
		}

		client = apns2.NewTokenClient(authToken)

		logger.Info("APNs provider initialized with token authentication",
			zap.String("bundle_id", config.BundleID),
			zap.String("key_id", config.KeyID),
			zap.String("team_id", config.TeamID),
			zap.Bool("production", config.Production))
	} else if config.CertificatePath != "" {
		// Certificate-based authentication (legacy)
		certFile, openErr := os.Open(config.CertificatePath)
		if openErr != nil {
			logger.Error("Failed to open APNs certificate file",
				zap.Error(openErr),
				zap.String("cert_path", config.CertificatePath))
			return nil, fmt.Errorf("failed to open certificate: %w", openErr)
		}
		defer certFile.Close()

		cert, certErr := certificate.FromP12File(config.CertificatePath, config.CertificatePassword)
		if certErr != nil {
			logger.Error("Failed to load APNs certificate",
				zap.Error(certErr),
				zap.String("cert_path", config.CertificatePath))
			return nil, fmt.Errorf("failed to load certificate: %w", certErr)
		}

		if config.Production {
			client = apns2.NewClient(cert)
		} else {
			client = apns2.NewClient(cert)
		}

		logger.Info("APNs provider initialized with certificate authentication",
			zap.String("bundle_id", config.BundleID),
			zap.Bool("production", config.Production))
	} else {
		return nil, fmt.Errorf("either token-based (KeyPath, KeyID, TeamID) or certificate-based (CertificatePath) authentication must be provided")
	}

	return &APNsProvider{
		client:     client,
		production: config.Production,
		bundleID:   config.BundleID,
		teamID:     config.TeamID,
		keyID:      config.KeyID,
	}, nil
}

// Send implements Provider interface for APNs
func (a *APNsProvider) Send(ctx context.Context, notification *Notification, tokens []string) (*SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("APNs client is not initialized")
	}

	if len(tokens) == 0 {
		return &SendResult{}, nil
	}

	result := &SendResult{
		SuccessCount:  0,
		FailureCount:  0,
		InvalidTokens: []string{},
		Errors:        []error{},
	}

	for _, deviceToken := range tokens {
		// Build APNs payload
		p := payload.NewPayload().
			AlertTitle(notification.Title).
			AlertBody(notification.Body)

		// Add sound
		if notification.Sound != "" {
			p.Sound(notification.Sound)
		}

		// Add badge
		if notification.Badge != nil {
			p.Badge(*notification.Badge)
		}

		// Add category
		if notification.Category != "" {
			p.Category(notification.Category)
		}

		// Add custom data
		for key, value := range notification.Data {
			p.Custom(key, value)
		}

		// Create notification
		notificationMsg := &apns2.Notification{
			DeviceToken: deviceToken,
			Topic:       a.bundleID,
			Payload:     p,
		}

		// Set priority (10 = high, 5 = normal)
		if notification.Priority == "high" {
			notificationMsg.Priority = apns2.PriorityHigh
		} else {
			notificationMsg.Priority = apns2.PriorityLow
		}

		// Send notification
		resp, err := a.client.PushWithContext(ctx, notificationMsg)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, err)
			logger.Warn("Failed to send APNs notification",
				zap.Error(err),
				zap.String("device_token", deviceToken))
			continue
		}

		if resp.StatusCode == 200 {
			result.SuccessCount++
			logger.Debug("APNs notification sent successfully",
				zap.String("device_token", deviceToken),
				zap.String("apns_id", resp.ApnsID))
		} else {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("APNs error: %s", resp.Reason))

			// Check if token is invalid
			if resp.StatusCode == 410 || // Unregistered
				resp.Reason == "Unregistered" ||
				resp.Reason == "BadDeviceToken" ||
				resp.Reason == "DeviceTokenNotForTopic" {
				result.InvalidTokens = append(result.InvalidTokens, deviceToken)
			}

			logger.Warn("APNs notification failed",
				zap.Int("status_code", resp.StatusCode),
				zap.String("reason", resp.Reason),
				zap.String("device_token", deviceToken))
		}
	}

	logger.Info("APNs batch send completed",
		zap.Int("success_count", result.SuccessCount),
		zap.Int("failure_count", result.FailureCount),
		zap.Int("invalid_tokens", len(result.InvalidTokens)),
		zap.String("title", notification.Title))

	return result, nil
}

// SendToUser implements Provider interface for APNs
// Note: APNs doesn't have a direct "send to user" API, so this method
// requires caller to provide device tokens. This implementation
// returns an error as tokens must be managed externally.
func (a *APNsProvider) SendToUser(ctx context.Context, notification *Notification, userID uuid.UUID) (*SendResult, error) {
	return nil, fmt.Errorf("APNs SendToUser requires tokens to be provided externally. Use Send() method instead.")
}

// SendWithPriority sends a notification with explicit priority and expiration
func (a *APNsProvider) SendWithPriority(ctx context.Context, notification *Notification, deviceToken string, priority int, expiration time.Time) (*apns2.Response, error) {
	if a.client == nil {
		return nil, fmt.Errorf("APNs client is not initialized")
	}

	// Build APNs payload
	p := payload.NewPayload().
		AlertTitle(notification.Title).
		AlertBody(notification.Body)

	// Add sound
	if notification.Sound != "" {
		p.Sound(notification.Sound)
	}

	// Add badge
	if notification.Badge != nil {
		p.Badge(*notification.Badge)
	}

	// Add category
	if notification.Category != "" {
		p.Category(notification.Category)
	}

	// Add custom data
	for key, value := range notification.Data {
		p.Custom(key, value)
	}

	// Create notification
	notificationMsg := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       a.bundleID,
		Payload:     p,
		Priority:    priority,
		Expiration:  expiration,
	}

	// Send notification
	resp, err := a.client.PushWithContext(ctx, notificationMsg)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// SendSilentNotification sends a silent notification (no alert, just data)
func (a *APNsProvider) SendSilentNotification(ctx context.Context, data map[string]string, deviceToken string) (*apns2.Response, error) {
	if a.client == nil {
		return nil, fmt.Errorf("APNs client is not initialized")
	}

	// Build silent payload
	p := payload.NewPayload().
		ContentAvailable()

	// Add custom data
	for key, value := range data {
		p.Custom(key, value)
	}

	// Create notification
	notificationMsg := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       a.bundleID,
		Payload:     p,
		Priority:    apns2.PriorityHigh, // Silent notifications need high priority
	}

	// Send notification
	resp, err := a.client.PushWithContext(ctx, notificationMsg)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

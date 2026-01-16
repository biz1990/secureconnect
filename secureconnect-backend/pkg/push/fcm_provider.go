package push

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"google.golang.org/api/option"

	"secureconnect-backend/pkg/logger"

	"go.uber.org/zap"
)

// FCMProvider implements Provider interface for Firebase Cloud Messaging
type FCMProvider struct {
	app *firebase.App
}

// FCMConfig contains configuration for FCM provider
type FCMConfig struct {
	CredentialsPath string // Path to service account JSON file
	CredentialsJSON []byte // Service account JSON content (alternative to file path)
	ProjectID       string // Firebase Project ID
}

// NewFCMProvider creates a new FCM provider
func NewFCMProvider(config *FCMConfig) (*FCMProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("FCM config is required")
	}

	var opts []option.ClientOption

	// Use credentials from JSON content if provided
	if len(config.CredentialsJSON) > 0 {
		opts = append(opts, option.WithCredentialsJSON(config.CredentialsJSON))
	} else if config.CredentialsPath != "" {
		opts = append(opts, option.WithCredentialsFile(config.CredentialsPath))
	} else {
		return nil, fmt.Errorf("either CredentialsPath or CredentialsJSON must be provided")
	}

	ctx := context.Background()

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: config.ProjectID,
	}, opts...)
	if err != nil {
		logger.Error("Failed to initialize Firebase app",
			zap.Error(err),
			zap.String("project_id", config.ProjectID))
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	logger.Info("FCM provider initialized successfully",
		zap.String("project_id", config.ProjectID))

	return &FCMProvider{
		app: app,
	}, nil
}

// Send implements Provider interface for FCM
func (f *FCMProvider) Send(ctx context.Context, notification *Notification, tokens []string) (*SendResult, error) {
	if f.app == nil {
		return nil, fmt.Errorf("FCM app is not initialized")
	}

	if len(tokens) == 0 {
		return &SendResult{}, nil
	}

	// Get messaging client
	client, err := f.app.Messaging(ctx)
	if err != nil {
		logger.Error("Failed to get messaging client",
			zap.Error(err))
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	// Build FCM message
	fcmMessage := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Body,
		},
		Tokens: tokens,
		Data:   notification.Data,
	}

	// Add optional fields
	if notification.Sound != "" {
		fcmMessage.Android = &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Sound: notification.Sound,
			},
		}
	}

	if notification.Priority == "high" {
		if fcmMessage.Android == nil {
			fcmMessage.Android = &messaging.AndroidConfig{}
		}
		fcmMessage.Android.Priority = "high"
	}

	if notification.Badge != nil {
		if fcmMessage.Android == nil {
			fcmMessage.Android = &messaging.AndroidConfig{}
		}
		if fcmMessage.Android.Notification == nil {
			fcmMessage.Android.Notification = &messaging.AndroidNotification{}
		}
		fcmMessage.Android.Notification.NotificationCount = notification.Badge
	}

	if notification.Category != "" {
		if fcmMessage.Android == nil {
			fcmMessage.Android = &messaging.AndroidConfig{}
		}
		if fcmMessage.Android.Notification == nil {
			fcmMessage.Android.Notification = &messaging.AndroidNotification{}
		}
		fcmMessage.Android.Notification.ChannelID = notification.Category
	}

	// Send multicast message
	response, err := client.SendMulticast(ctx, fcmMessage)
	if err != nil {
		logger.Error("Failed to send FCM multicast message",
			zap.Error(err),
			zap.Int("token_count", len(tokens)))
		return nil, fmt.Errorf("failed to send FCM message: %w", err)
	}

	// Process results
	result := &SendResult{
		SuccessCount:  response.SuccessCount,
		FailureCount:  response.FailureCount,
		InvalidTokens: []string{},
		Errors:        []error{},
	}

	for i, resp := range response.Responses {
		if !resp.Success {
			if resp.Error != nil {
				result.Errors = append(result.Errors, resp.Error)
				logger.Warn("FCM send failed for token",
					zap.String("token_prefix", maskPushToken(tokens[i])),
					zap.Error(resp.Error))

				// Check if token is invalid
				if messaging.IsUnregistered(resp.Error) ||
					messaging.IsInvalidArgument(resp.Error) ||
					messaging.IsMessageRateExceeded(resp.Error) {
					result.InvalidTokens = append(result.InvalidTokens, tokens[i])
				}
			}
		}
	}

	logger.Info("FCM message sent",
		zap.Int("success_count", result.SuccessCount),
		zap.Int("failure_count", result.FailureCount),
		zap.Int("invalid_tokens", len(result.InvalidTokens)),
		zap.String("title", notification.Title))

	return result, nil
}

// SendToUser implements Provider interface for FCM
// Note: FCM doesn't have a direct "send to user" API, so this method
// requires caller to provide user tokens. This implementation
// returns an error as tokens must be managed externally.
func (f *FCMProvider) SendToUser(ctx context.Context, notification *Notification, userID uuid.UUID) (*SendResult, error) {
	return nil, fmt.Errorf("FCM SendToUser requires tokens to be provided externally. Use Send() method instead.")
}

// SendToTopic sends a message to a specific topic
func (f *FCMProvider) SendToTopic(ctx context.Context, notification *Notification, topic string) (*SendResult, error) {
	if f.app == nil {
		return nil, fmt.Errorf("FCM app is not initialized")
	}

	client, err := f.app.Messaging(ctx)
	if err != nil {
		logger.Error("Failed to get messaging client",
			zap.Error(err))
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	fcmMessage := &messaging.Message{
		Notification: &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Body,
		},
		Topic: topic,
		Data:  notification.Data,
	}

	// Add optional fields
	if notification.Sound != "" {
		fcmMessage.Android = &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Sound: notification.Sound,
			},
		}
	}

	if notification.Priority == "high" {
		if fcmMessage.Android == nil {
			fcmMessage.Android = &messaging.AndroidConfig{}
		}
		fcmMessage.Android.Priority = "high"
	}

	messageID, err := client.Send(ctx, fcmMessage)
	if err != nil {
		logger.Error("Failed to send FCM message to topic",
			zap.Error(err),
			zap.String("topic", topic))
		return nil, fmt.Errorf("failed to send FCM message to topic: %w", err)
	}

	logger.Info("FCM message sent to topic",
		zap.String("message_id", messageID),
		zap.String("topic", topic),
		zap.String("title", notification.Title))

	return &SendResult{
		SuccessCount:  1,
		FailureCount:  0,
		InvalidTokens: nil,
		Errors:        nil,
	}, nil
}

// SubscribeToTopic subscribes tokens to a topic
func (f *FCMProvider) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	if f.app == nil {
		return fmt.Errorf("FCM app is not initialized")
	}

	client, err := f.app.Messaging(ctx)
	if err != nil {
		logger.Error("Failed to get messaging client",
			zap.Error(err))
		return fmt.Errorf("failed to get messaging client: %w", err)
	}

	response, err := client.SubscribeToTopic(ctx, tokens, topic)
	if err != nil {
		logger.Error("Failed to subscribe to FCM topic",
			zap.Error(err),
			zap.String("topic", topic),
			zap.Int("token_count", len(tokens)))
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	logger.Info("Subscribed to FCM topic",
		zap.Int("success_count", response.SuccessCount),
		zap.Int("failure_count", response.FailureCount),
		zap.String("topic", topic))

	return nil
}

// maskPushToken returns a safe masked version of a push token for logging
// Shows only first 8 and last 8 characters, with middle masked
func maskPushToken(token string) string {
	if len(token) <= 16 {
		return "********"
	}
	return token[:8] + "..." + token[len(token)-8:]
}

// UnsubscribeFromTopic unsubscribes tokens from a topic
func (f *FCMProvider) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	if f.app == nil {
		return fmt.Errorf("FCM app is not initialized")
	}

	client, err := f.app.Messaging(ctx)
	if err != nil {
		logger.Error("Failed to get messaging client",
			zap.Error(err))
		return fmt.Errorf("failed to get messaging client: %w", err)
	}

	response, err := client.UnsubscribeFromTopic(ctx, tokens, topic)
	if err != nil {
		logger.Error("Failed to unsubscribe from FCM topic",
			zap.Error(err),
			zap.String("topic", topic),
			zap.Int("token_count", len(tokens)))
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	logger.Info("Unsubscribed from FCM topic",
		zap.Int("success_count", response.SuccessCount),
		zap.Int("failure_count", response.FailureCount),
		zap.String("topic", topic))

	return nil
}

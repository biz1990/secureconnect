package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/google/uuid"
)

// FirebaseProvider implements the Provider interface using Firebase Cloud Messaging
// It supports Android, iOS (via APNs bridge), and Web platforms
type FirebaseProvider struct {
	app         *firebase.App
	client      *messaging.Client
	projectID   string
	initialized bool
}

// NewFirebaseProvider creates a new Firebase push notification provider
// Initializes Firebase Admin SDK using credentials from environment
func NewFirebaseProvider(projectID string) *FirebaseProvider {
	// Check for credentials file path (supports Docker secrets)
	// Priority: FIREBASE_CREDENTIALS_PATH (Docker secret) -> GOOGLE_APPLICATION_CREDENTIALS (legacy)
	credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credentialsPath == "" {
		credentialsPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if credentialsPath == "" {
		log.Println("FIREBASE_CREDENTIALS_PATH not set, creating mock provider")
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// Read credentials file into memory (more secure than passing file path)
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		log.Printf("Failed to read Firebase credentials file: error=%v\n", err)
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// Extract project ID from credentials if not provided
	if projectID == "" {
		var creds struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(credentials, &creds); err != nil {
			log.Printf("Failed to parse Firebase credentials: error=%v\n", err)
			return &FirebaseProvider{
				projectID:   "",
				initialized: false,
			}
		}
		projectID = creds.ProjectID
	}

	// Initialize Firebase Admin SDK with credentials from memory (more secure)
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(credentials))
	if err != nil {
		log.Printf("Failed to initialize Firebase app: error=%v\n", err)
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// Get messaging client
	client, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("Failed to get Firebase messaging client: project_id=%s, error=%v\n", projectID, err)
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	log.Printf("Firebase Admin SDK initialized successfully: project_id=%s\n", projectID)

	return &FirebaseProvider{
		app:         app,
		client:      client,
		projectID:   projectID,
		initialized: true,
	}
}

// Send implements the Provider interface
// Sends a notification to multiple device tokens
func (f *FirebaseProvider) Send(ctx context.Context, notification *Notification, tokens []string) (*SendResult, error) {
	if len(tokens) == 0 {
		log.Println("No tokens provided for Firebase send")
		return &SendResult{}, nil
	}

	if !f.initialized {
		log.Println("FirebaseProvider not initialized, using mock behavior")
		return f.mockSend(ctx, notification, tokens)
	}

	// Build messages for each token
	messages := make([]*messaging.Message, len(tokens))
	for i, token := range tokens {
		messages[i] = f.buildMessage(notification, token)
	}

	// Send messages using SendEach
	response, err := f.client.SendEach(ctx, messages)
	if err != nil {
		log.Printf("Failed to send Firebase messages: project_id=%s, token_count=%d, error=%v\n", f.projectID, len(tokens), err)
		return &SendResult{
			SuccessCount:  0,
			FailureCount:  len(tokens),
			InvalidTokens: nil,
			Errors:        []error{err},
		}, err
	}

	// Process results
	successCount := 0
	failureCount := 0
	invalidTokens := []string{}

	for i, resp := range response.Responses {
		if resp.Success {
			successCount++
		} else {
			failureCount++
			if resp.Error != nil {
				log.Printf("Firebase send error for token: index=%d, error=%s\n", i, resp.Error.Error())

				// Check for invalid token errors by error message
				errMsg := resp.Error.Error()
				if errMsg == "UNREGISTERED" || errMsg == "INVALID_ARGUMENT" || errMsg == "registration-token-not-registered" {
					if i < len(tokens) {
						invalidTokens = append(invalidTokens, tokens[i])
					}
				}
			}
		}
	}

	log.Printf("Firebase messages sent: project_id=%s, success_count=%d, failure_count=%d, invalid_tokens=%d\n",
		f.projectID, successCount, failureCount, len(invalidTokens))

	return &SendResult{
		SuccessCount:  successCount,
		FailureCount:  failureCount,
		InvalidTokens: invalidTokens,
		Errors:        nil,
	}, nil
}

// SendToUser implements the Provider interface
// Sends a notification to a specific user (requires fetching user tokens first)
func (f *FirebaseProvider) SendToUser(ctx context.Context, notification *Notification, userID uuid.UUID) (*SendResult, error) {
	log.Printf("SendToUser called for FirebaseProvider: user_id=%s, title=%s\n", userID.String(), notification.Title)

	// Note: This method requires access to TokenRepository to fetch user tokens
	// In the current architecture, this is handled by the Service layer
	// This implementation is a placeholder for future enhancement

	return &SendResult{
		SuccessCount:  0,
		FailureCount:  0,
		InvalidTokens: nil,
		Errors:        nil,
	}, fmt.Errorf("SendToUser not implemented for FirebaseProvider - use Send with tokens")
}

// buildMessage constructs a Firebase message from a notification
func (f *FirebaseProvider) buildMessage(notification *Notification, token string) *messaging.Message {
	// Build data payload
	data := notification.Data
	if data == nil {
		data = make(map[string]string)
	}

	// Add common fields
	data["title"] = notification.Title
	data["body"] = notification.Body
	data["timestamp"] = fmt.Sprintf("%d", time.Now().Unix())

	// Add notification-specific data
	for k, v := range notification.Data {
		data[k] = v
	}

	// Build Android notification
	androidNotification := &messaging.AndroidNotification{
		Title: notification.Title,
		Body:  notification.Body,
	}

	if notification.Sound != "" {
		androidNotification.Sound = notification.Sound
	}

	if notification.Badge != nil {
		androidNotification.NotificationCount = notification.Badge
	}

	if notification.ClickAction != "" {
		androidNotification.ClickAction = notification.ClickAction
	}

	// Build Android config
	androidConfig := &messaging.AndroidConfig{
		Notification: androidNotification,
		Data:         data,
	}

	if notification.Priority != "" {
		androidConfig.Priority = notification.Priority
	}

	// Build APNs config (iOS)
	apsAlert := &messaging.ApsAlert{
		Title: notification.Title,
		Body:  notification.Body,
	}

	aps := &messaging.Aps{
		Alert: apsAlert,
	}

	if notification.Badge != nil {
		aps.Badge = notification.Badge
	}

	if notification.Sound != "" {
		aps.Sound = notification.Sound
	}

	apnsPayload := &messaging.APNSPayload{
		Aps: aps,
	}

	apnsConfig := &messaging.APNSConfig{
		Payload: apnsPayload,
	}

	// Build Web Push config
	webpushNotification := &messaging.WebpushNotification{
		Title: notification.Title,
		Body:  notification.Body,
		Icon:  "/icon-192x192.png",
	}

	webpushConfig := &messaging.WebpushConfig{
		Notification: webpushNotification,
		Data:         data,
	}

	// Build message
	return &messaging.Message{
		Data:    data,
		Android: androidConfig,
		APNS:    apnsConfig,
		Webpush: webpushConfig,
		Token:   token,
	}
}

// mockSend provides a mock implementation for development/testing
// when Firebase client is not initialized
func (f *FirebaseProvider) mockSend(_ context.Context, notification *Notification, tokens []string) (*SendResult, error) {
	log.Printf("FirebaseProvider: Mock sending notification: title=%s, body=%s, token_count=%d\n",
		notification.Title, notification.Body, len(tokens))

	// Return success for all tokens
	return &SendResult{
		SuccessCount:  len(tokens),
		FailureCount:  0,
		InvalidTokens: nil,
		Errors:        nil,
	}, nil
}

// IsInitialized returns whether the provider is properly initialized
func (f *FirebaseProvider) IsInitialized() bool {
	return f.initialized
}

// GetProjectID returns the Firebase project ID
func (f *FirebaseProvider) GetProjectID() string {
	return f.projectID
}

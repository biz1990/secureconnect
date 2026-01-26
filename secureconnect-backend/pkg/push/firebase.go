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
// Supports Docker secrets via FILE pattern: FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials
func NewFirebaseProvider(projectID string) *FirebaseProvider {
	// Check if running in production mode
	productionMode := os.Getenv("ENV") == "production"

	// Check for credentials file path (supports Docker secrets)
	// In production, ONLY Docker secrets are allowed via FIREBASE_CREDENTIALS_PATH
	credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credentialsPath == "" {
		if productionMode {
			log.Println("❌ FIREBASE_CREDENTIALS_PATH not set. Required in production mode.")
			log.Println("❌ Please create Docker secret: echo '<firebase-service-account-json>' | docker secret create firebase_credentials -")
			log.Println("❌ Then ensure docker-compose.production.yml has: FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials")
			log.Fatal("❌ Fatal: Firebase credentials required in production mode")
		}
		log.Println("FIREBASE_CREDENTIALS_PATH not set, creating mock provider (development mode)")
		log.Println("ℹ️  To use Firebase in development, set FIREBASE_CREDENTIALS_PATH=/path/to/service-account.json")
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// In production, verify credentials path points to Docker secrets
	if productionMode && credentialsPath != "/run/secrets/firebase_credentials" {
		log.Printf("❌ FIREBASE_CREDENTIALS_PATH must be /run/secrets/firebase_credentials in production. Got: %s\n", credentialsPath)
		log.Fatal("❌ Fatal: Firebase credentials must use Docker secrets in production mode")
	}

	// Read credentials file into memory (more secure than passing file path)
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		if productionMode {
			log.Printf("❌ Failed to read Firebase credentials file: path=%s, error=%v\n", credentialsPath, err)
			log.Println("❌ Ensure Docker secret exists: docker secret ls | grep firebase_credentials")
			log.Fatal("❌ Fatal: Firebase credentials file required in production mode")
		}
		log.Printf("Failed to read Firebase credentials file: path=%s, error=%v\n", credentialsPath, err)
		log.Println("Creating mock provider (development mode)")
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// Validate credentials content is not empty
	if len(credentials) == 0 {
		if productionMode {
			log.Println("❌ Firebase credentials file is empty")
			log.Fatal("❌ Fatal: Firebase credentials file must contain valid service account JSON")
		}
		log.Println("Firebase credentials file is empty, creating mock provider (development mode)")
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// Extract project ID from credentials if not provided
	// Supports FILE pattern: FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id
	projectIDFile := os.Getenv("FIREBASE_PROJECT_ID_FILE")
	if projectID == "" && projectIDFile != "" {
		projectIDBytes, err := os.ReadFile(projectIDFile)
		if err != nil {
			if productionMode {
				log.Printf("❌ Failed to read FIREBASE_PROJECT_ID_FILE: path=%s, error=%v\n", projectIDFile, err)
				log.Println("❌ Ensure Docker secret exists: docker secret ls | grep firebase_project_id")
				log.Fatal("❌ Fatal: Firebase project ID required in production mode")
			}
			log.Printf("Failed to read FIREBASE_PROJECT_ID_FILE: path=%s, error=%v\n", projectIDFile, err)
		} else {
			projectID = string(projectIDBytes)
			log.Printf("✅ Loaded project ID from file: project_id=%s\n", projectID)
		}
	}

	// In production, if using FILE pattern, verify it points to Docker secrets
	if productionMode && projectIDFile != "" && projectIDFile != "/run/secrets/firebase_project_id" {
		log.Printf("❌ FIREBASE_PROJECT_ID_FILE must be /run/secrets/firebase_project_id in production. Got: %s\n", projectIDFile)
		log.Fatal("❌ Fatal: Firebase project ID must use Docker secrets in production mode")
	}

	// If still no project ID, extract from credentials JSON
	if projectID == "" {
		var creds struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(credentials, &creds); err != nil {
			if productionMode {
				log.Printf("❌ Failed to parse Firebase credentials: error=%v\n", err)
				log.Fatal("❌ Fatal: Invalid Firebase credentials format")
			}
			log.Printf("Failed to parse Firebase credentials: error=%v\n", err)
			return &FirebaseProvider{
				projectID:   "",
				initialized: false,
			}
		}
		projectID = creds.ProjectID
	}

	// Validate project ID is not empty
	if projectID == "" {
		if productionMode {
			log.Println("❌ Firebase project ID is empty")
			log.Fatal("❌ Fatal: Firebase project ID required in production mode")
		}
		log.Println("⚠️  Warning: Firebase project ID is empty")
		return &FirebaseProvider{
			projectID:   "",
			initialized: false,
		}
	}

	// Initialize Firebase Admin SDK with credentials from memory (more secure)
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(credentials))
	if err != nil {
		if productionMode {
			log.Printf("❌ Failed to initialize Firebase app: project_id=%s, error=%v\n", projectID, err)
			log.Fatal("❌ Fatal: Firebase initialization failed in production mode")
		}
		log.Printf("Failed to initialize Firebase app: project_id=%s, error=%v\n", projectID, err)
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	// Get messaging client
	client, err := app.Messaging(ctx)
	if err != nil {
		if productionMode {
			log.Printf("❌ Failed to get Firebase messaging client: project_id=%s, error=%v\n", projectID, err)
			log.Fatal("❌ Fatal: Firebase messaging client initialization failed in production mode")
		}
		log.Printf("Failed to get Firebase messaging client: project_id=%s, error=%v\n", projectID, err)
		return &FirebaseProvider{
			projectID:   projectID,
			initialized: false,
		}
	}

	log.Printf("✅ Firebase Admin SDK initialized successfully: project_id=%s, credentials_path=%s\n", projectID, credentialsPath)
	if productionMode {
		log.Println("✅ Firebase running in PRODUCTION mode with real credentials")
	} else {
		log.Println("ℹ️  Firebase running in DEVELOPMENT mode")
	}

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

// Validate checks if the provider is properly initialized
// Returns an error if Firebase is required but misconfigured
func (f *FirebaseProvider) Validate() error {
	productionMode := os.Getenv("ENV") == "production"

	if !f.initialized {
		if productionMode {
			return fmt.Errorf("Firebase provider not initialized in production mode")
		}
		// In development, allow uninitialized provider
		return nil
	}

	if f.projectID == "" {
		return fmt.Errorf("Firebase project ID is empty")
	}

	return nil
}

// StartupCheck performs a comprehensive startup validation
// Returns an error if Firebase is required but misconfigured
// This should be called during service initialization
func StartupCheck(provider *FirebaseProvider) error {
	if provider == nil {
		return fmt.Errorf("Firebase provider is nil")
	}

	productionMode := os.Getenv("ENV") == "production"

	if err := provider.Validate(); err != nil {
		if productionMode {
			log.Printf("❌ Firebase startup check failed: %v\n", err)
			log.Println("❌ Fatal: Firebase validation failed in production mode")
			return fmt.Errorf("fatal: %w", err)
		}
		log.Printf("⚠️  Firebase startup check warning: %v\n", err)
		log.Println("ℹ️  Running in development mode with mock Firebase provider")
		return nil
	}

	log.Printf("✅ Firebase startup check passed: project_id=%s, initialized=%v\n",
		provider.GetProjectID(), provider.IsInitialized())
	return nil
}

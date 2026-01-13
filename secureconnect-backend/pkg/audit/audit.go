package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"secureconnect-backend/pkg/constants"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	// Authentication events
	EventLoginSuccess   AuditEventType = "login_success"
	EventLoginFailed    AuditEventType = "login_failed"
	EventLogout         AuditEventType = "logout"
	EventRegister       AuditEventType = "register"
	EventPasswordChange AuditEventType = "password_change"
	EventEmailChange    AuditEventType = "email_change"

	// User management events
	EventProfileUpdate AuditEventType = "profile_update"
	EventAccountDelete AuditEventType = "account_delete"
	EventUserBlock     AuditEventType = "user_block"
	EventUserUnblock   AuditEventType = "user_unblock"
	EventFriendRequest AuditEventType = "friend_request"
	EventFriendAccept  AuditEventType = "friend_accept"
	EventFriendReject  AuditEventType = "friend_reject"
	EventFriendRemove  AuditEventType = "friend_remove"

	// Conversation events
	EventConversationCreate AuditEventType = "conversation_create"
	EventMessageSend        AuditEventType = "message_send"
	EventMessageDelete      AuditEventType = "message_delete"

	// Call events
	EventCallInitiate AuditEventType = "call_initiate"
	EventCallEnd      AuditEventType = "call_end"

	// File events
	EventFileUpload AuditEventType = "file_upload"
	EventFileDelete AuditEventType = "file_delete"

	// Key management events
	EventKeyGenerate AuditEventType = "key_generate"
	EventKeyRotate   AuditEventType = "key_rotate"
	EventKeyRevoke   AuditEventType = "key_revoke"

	// Admin events
	EventAdminAction AuditEventType = "admin_action"
)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	EventID   uuid.UUID      `json:"event_id"`
	UserID    *uuid.UUID     `json:"user_id,omitempty"`
	EventType AuditEventType `json:"event_type"`
	Resource  string         `json:"resource,omitempty"`
	Action    string         `json:"action,omitempty"`
	IPAddress string         `json:"ip_address,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	Success   bool           `json:"success"`
	ErrorCode string         `json:"error_code,omitempty"`
	Details   string         `json:"details,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	redisClient *redis.Client
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(redisClient *redis.Client) *AuditLogger {
	return &AuditLogger{
		redisClient: redisClient,
	}
}

// Log logs an audit event
func (al *AuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	// Set timestamp
	event.Timestamp = time.Now().UTC()

	// Generate event ID if not set
	if event.EventID == uuid.Nil {
		event.EventID = uuid.New()
	}

	// Serialize event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Store in Redis list
	key := fmt.Sprintf("audit:events:%s", event.Timestamp.Format("2006-01-02"))
	member := fmt.Sprintf("%s:%s", event.EventID, eventJSON)

	err = al.redisClient.LPush(ctx, key, member).Err()
	if err != nil {
		return fmt.Errorf("failed to store audit event: %w", err)
	}

	// Set expiry for audit logs (keep for 90 days)
	err = al.redisClient.Expire(ctx, key, constants.AuditLogRetention).Err()
	if err != nil {
		return fmt.Errorf("failed to set audit log expiry: %w", err)
	}

	return nil
}

// LogLoginSuccess logs a successful login
func (al *AuditLogger) LogLoginSuccess(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventLoginSuccess,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogLoginFailed logs a failed login attempt
func (al *AuditLogger) LogLoginFailed(ctx context.Context, identifier, ipAddress, userAgent string, errorCode, details string) error {
	return al.Log(ctx, &AuditEvent{
		EventType: EventLoginFailed,
		Resource:  identifier,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   false,
		ErrorCode: errorCode,
		Details:   details,
	})
}

// LogLogout logs a logout event
func (al *AuditLogger) LogLogout(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventLogout,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogPasswordChange logs a password change
func (al *AuditLogger) LogPasswordChange(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string, success bool, errorCode string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventPasswordChange,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
		ErrorCode: errorCode,
	})
}

// LogProfileUpdate logs a profile update
func (al *AuditLogger) LogProfileUpdate(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string, fields string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventProfileUpdate,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   fields,
	})
}

// LogAccountDelete logs an account deletion
func (al *AuditLogger) LogAccountDelete(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventAccountDelete,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogUserBlock logs a user block event
func (al *AuditLogger) LogUserBlock(ctx context.Context, blockerID, blockedID uuid.UUID, ipAddress, userAgent, reason string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &blockerID,
		EventType: EventUserBlock,
		Resource:  blockedID.String(),
		Action:    "block",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   reason,
	})
}

// LogUserUnblock logs a user unblock event
func (al *AuditLogger) LogUserUnblock(ctx context.Context, blockerID, blockedID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &blockerID,
		EventType: EventUserUnblock,
		Resource:  blockedID.String(),
		Action:    "unblock",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogFriendRequest logs a friend request
func (al *AuditLogger) LogFriendRequest(ctx context.Context, requestingID, targetID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &requestingID,
		EventType: EventFriendRequest,
		Resource:  targetID.String(),
		Action:    "request",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogFriendAccept logs a friend acceptance
func (al *AuditLogger) LogFriendAccept(ctx context.Context, userID, friendID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventFriendAccept,
		Resource:  friendID.String(),
		Action:    "accept",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogFriendReject logs a friend rejection
func (al *AuditLogger) LogFriendReject(ctx context.Context, userID, friendID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventFriendReject,
		Resource:  friendID.String(),
		Action:    "reject",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogFriendRemove logs a friend removal
func (al *AuditLogger) LogFriendRemove(ctx context.Context, userID, friendID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventFriendRemove,
		Resource:  friendID.String(),
		Action:    "remove",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogCallInitiate logs a call initiation
func (al *AuditLogger) LogCallInitiate(ctx context.Context, userID uuid.UUID, callID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventCallInitiate,
		Resource:  callID.String(),
		Action:    "initiate",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogCallEnd logs a call ending
func (al *AuditLogger) LogCallEnd(ctx context.Context, userID uuid.UUID, callID uuid.UUID, duration int64, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventCallEnd,
		Resource:  callID.String(),
		Action:    "end",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   fmt.Sprintf("duration: %d seconds", duration),
	})
}

// LogFileUpload logs a file upload
func (al *AuditLogger) LogFileUpload(ctx context.Context, userID uuid.UUID, fileID uuid.UUID, fileName string, fileSize int64, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventFileUpload,
		Resource:  fileID.String(),
		Action:    "upload",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   fmt.Sprintf("filename: %s, size: %d bytes", fileName, fileSize),
	})
}

// LogFileDelete logs a file deletion
func (al *AuditLogger) LogFileDelete(ctx context.Context, userID uuid.UUID, fileID uuid.UUID, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventFileDelete,
		Resource:  fileID.String(),
		Action:    "delete",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// LogKeyGenerate logs a key generation
func (al *AuditLogger) LogKeyGenerate(ctx context.Context, userID uuid.UUID, keyType string, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventKeyGenerate,
		Action:    "generate",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   keyType,
	})
}

// LogKeyRotate logs a key rotation
func (al *AuditLogger) LogKeyRotate(ctx context.Context, userID uuid.UUID, keyType string, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventKeyRotate,
		Action:    "rotate",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   keyType,
	})
}

// LogKeyRevoke logs a key revocation
func (al *AuditLogger) LogKeyRevoke(ctx context.Context, userID uuid.UUID, keyType string, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &userID,
		EventType: EventKeyRevoke,
		Action:    "revoke",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Details:   keyType,
	})
}

// LogAdminAction logs an admin action
func (al *AuditLogger) LogAdminAction(ctx context.Context, adminID uuid.UUID, action, resource, ipAddress, userAgent string) error {
	return al.Log(ctx, &AuditEvent{
		UserID:    &adminID,
		EventType: EventAdminAction,
		Action:    action,
		Resource:  resource,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
	})
}

// GetEvents retrieves audit events for a user
func (al *AuditLogger) GetEvents(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*AuditEvent, error) {
	// Get keys for all days in the range
	now := time.Now().UTC()
	keys := make([]string, 0)
	for i := 0; i < 90; i++ {
		date := now.AddDate(0, 0, -i)
		key := fmt.Sprintf("audit:events:%s", date.Format("2006-01-02"))
		keys = append(keys, key)
	}

	// Get events from Redis
	var events []*AuditEvent
	for _, key := range keys {
		members, err := al.redisClient.LRange(ctx, key, int64(offset), int64(offset+limit)).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get audit events: %w", err)
		}

		for _, member := range members {
			var event AuditEvent
			err := json.Unmarshal([]byte(member), &event)
			if err != nil {
				continue
			}
			if event.UserID != nil && *event.UserID == userID {
				events = append(events, &event)
			}
		}
	}

	return events, nil
}

// GetEventsByType retrieves audit events by type
func (al *AuditLogger) GetEventsByType(ctx context.Context, eventType AuditEventType, limit int, offset int) ([]*AuditEvent, error) {
	// Get keys for all days in the range
	now := time.Now().UTC()
	keys := make([]string, 0)
	for i := 0; i < 90; i++ {
		date := now.AddDate(0, 0, -i)
		key := fmt.Sprintf("audit:events:%s", date.Format("2006-01-02"))
		keys = append(keys, key)
	}

	// Get events from Redis
	var events []*AuditEvent
	for _, key := range keys {
		members, err := al.redisClient.LRange(ctx, key, int64(offset), int64(offset+limit)).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get audit events: %w", err)
		}

		for _, member := range members {
			var event AuditEvent
			err := json.Unmarshal([]byte(member), &event)
			if err != nil {
				continue
			}
			if event.EventType == eventType {
				events = append(events, &event)
			}
		}
	}

	return events, nil
}

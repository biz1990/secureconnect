// Package constants defines application-wide constants for timeouts, limits, and durations.
package constants

import "time"

// Time-related constants
const (
	// DefaultTimeout is the default timeout for most operations
	DefaultTimeout = 30 * time.Second

	// LongTimeout is for complex operations or batch processing
	LongTimeout = 60 * time.Second

	// WebSocketPingInterval is the interval for WebSocket ping/pong
	WebSocketPingInterval = 60 * time.Second

	// GracefulShutdownTimeout is the timeout for graceful server shutdown
	GracefulShutdownTimeout = 30 * time.Second
)

// JWT-related constants
const (
	// AccessTokenExpiry is the default access token lifetime
	AccessTokenExpiry = 15 * time.Minute

	// RefreshTokenExpiry is the default refresh token lifetime
	RefreshTokenExpiry = 24 * time.Hour

	// SessionExpiry is the default session lifetime
	SessionExpiry = 30 * 24 * time.Hour // 30 days
)

// Database connection constants
const (
	// MaxConnLifetime is the maximum lifetime of a database connection
	MaxConnLifetime = 1 * time.Hour

	// MaxConnIdleTime is the maximum idle time for a database connection
	MaxConnIdleTime = 30 * time.Minute

	// HealthCheckPeriod is the interval between database health checks
	HealthCheckPeriod = 1 * time.Minute
)

// Security and rate limiting constants
const (
	// MaxFailedLoginAttempts is the maximum number of failed login attempts before lockout
	MaxFailedLoginAttempts = 5

	// AccountLockDuration is the duration an account remains locked after too many failed attempts
	AccountLockDuration = 15 * time.Minute

	// FailedLoginWindow is the time window in which failed login attempts are counted
	FailedLoginWindow = 15 * time.Minute
)

// Storage and file upload constants
const (
	// PresignedURLExpiry is the validity period for presigned upload URLs
	PresignedURLExpiry = 15 * time.Minute

	// EmailVerificationExpiry is the validity period for email verification tokens
	EmailVerificationExpiry = 24 * time.Hour
)

// Push notification constants
const (
	// PushTokenExpiry is the validity period for push notification tokens
	PushTokenExpiry = 30 * 24 * time.Hour // 30 days
)

// Audit log constants
const (
	// AuditLogRetention is the duration audit logs are retained
	AuditLogRetention = 90 * 24 * time.Hour // 90 days
)

// Pagination constants
const (
	// DefaultPageSize is the default number of items per page
	DefaultPageSize = 20

	// MaxPageSize is the maximum number of items per page
	MaxPageSize = 100

	// MinPageSize is the minimum number of items per page
	MinPageSize = 1
)

// Validation constants
const (
	// MinUsernameLength is the minimum allowed username length
	MinUsernameLength = 3

	// MinPasswordLength is the minimum allowed password length
	MinPasswordLength = 8

	// MaxUsernameLength is the maximum allowed username length
	MaxUsernameLength = 50

	// MaxDisplayNameLength is the maximum allowed display name length
	MaxDisplayNameLength = 100

	// MaxEmailLength is the maximum allowed email length
	MaxEmailLength = 255
)

// Call-related constants
const (
	// MaxCallDuration is the maximum allowed call duration (24 hours)
	MaxCallDuration = 24 * time.Hour

	// CallStatusRinging indicates a call is waiting to be answered
	CallStatusRinging = "ringing"

	// CallStatusActive indicates a call is in progress
	CallStatusActive = "active"

	// CallStatusEnded indicates a call has ended
	CallStatusEnded = "ended"

	// CallTypeAudio indicates an audio-only call
	CallTypeAudio = "audio"

	// CallTypeVideo indicates a video call
	CallTypeVideo = "video"
)

// User status constants
const (
	// UserStatusOnline indicates a user is currently online
	UserStatusOnline = "online"

	// UserStatusOffline indicates a user is currently offline
	UserStatusOffline = "offline"

	// UserStatusAway indicates a user is away
	UserStatusAway = "away"
)

// Message constants
const (
	// MaxMessageLength is the maximum allowed message length
	MaxMessageLength = 10000

	// MaxAttachmentSize is the maximum allowed attachment size in bytes (50MB)
	MaxAttachmentSize = 50 * 1024 * 1024
)

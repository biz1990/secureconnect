package domain

import (
	"time"

	"github.com/google/uuid"
)

// SystemStats represents overall system statistics
type SystemStats struct {
	TotalUsers    int64     `json:"total_users"`
	ActiveUsers   int64     `json:"active_users"` // Users active in last 24h
	TotalMessages int64     `json:"total_messages"`
	TotalCalls    int64     `json:"total_calls"`
	StorageUsed   int64     `json:"storage_used"`   // In bytes
	DatabaseSize  int64     `json:"database_size"`  // In bytes
	CacheHitRate  float64   `json:"cache_hit_rate"` // Percentage
	UptimeSeconds int64     `json:"uptime_seconds"`
	LastUpdated   time.Time `json:"last_updated"`
}

// UserListRequest represents query parameters for listing users
type UserListRequest struct {
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	Search    string `json:"search"`     // Search by email or username
	Status    string `json:"status"`     // Filter by status: online, offline, all
	SortBy    string `json:"sort_by"`    // Sort field: created_at, email, username
	SortOrder string `json:"sort_order"` // ASC or DESC
}

// UserListResponse represents paginated user list
type UserListResponse struct {
	Users      []UserInfo `json:"users"`
	TotalCount int        `json:"total_count"`
	HasMore    bool       `json:"has_more"`
}

// UserInfo represents user information for admin view
type UserInfo struct {
	UserID      uuid.UUID  `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	Status      string     `json:"status"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	IsBanned    bool       `json:"is_banned"`
	BannedAt    *time.Time `json:"banned_at,omitempty"`
	BanReason   *string    `json:"ban_reason,omitempty"`
}

// BanUserRequest represents request to ban a user
type BanUserRequest struct {
	UserID    uuid.UUID `json:"user_id" binding:"required"`
	Reason    string    `json:"reason" binding:"required,min=10,max=500"`
	Permanent bool      `json:"permanent"`
	Duration  int       `json:"duration"` // Duration in hours if not permanent
}

// UnbanUserRequest represents request to unban a user
type UnbanUserRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Reason string    `json:"reason" binding:"required,min=10,max=500"`
}

// ServiceHealth represents health status of a service
type ServiceHealth struct {
	ServiceName string                 `json:"service_name"`
	Status      string                 `json:"status"` // healthy, degraded, unhealthy
	LastCheck   time.Time              `json:"last_check"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
}

// SystemHealth represents overall system health
type SystemHealth struct {
	OverallStatus string                   `json:"overall_status"`
	Services      map[string]ServiceHealth `json:"services"`
	CheckedAt     time.Time                `json:"checked_at"`
}

// AuditLog represents an administrative action
type AuditLog struct {
	AuditID    uuid.UUID `json:"audit_id"`
	AdminID    uuid.UUID `json:"admin_id"`
	Action     string    `json:"action"`      // ban_user, unban_user, delete_user, etc.
	TargetType string    `json:"target_type"` // user, message, conversation, etc.
	TargetID   uuid.UUID `json:"target_id"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	Details    string    `json:"details"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuditLogRequest represents query parameters for audit logs
type AuditLogRequest struct {
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
	AdminID    *uuid.UUID `json:"admin_id,omitempty"`
	Action     string     `json:"action"`
	TargetType string     `json:"target_type"`
	StartDate  *time.Time `json:"start_date"`
	EndDate    *time.Time `json:"end_date"`
}

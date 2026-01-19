package cockroach

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"secureconnect-backend/internal/domain"
)

// AdminRepository handles administrative data operations
type AdminRepository struct {
	db *pgxpool.Pool
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

// GetSystemStats retrieves overall system statistics
func (r *AdminRepository) GetSystemStats(ctx context.Context) (*domain.SystemStats, error) {
	stats := &domain.SystemStats{
		LastUpdated: time.Now(),
	}

	// Get total users
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}

	// Get active users (last 24h)
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
		WHERE updated_at > NOW() - INTERVAL '24 hours'
	`).Scan(&stats.ActiveUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	// Get total messages (from Cassandra, approximate)
	// For CockroachDB, we might store message stats separately
	err = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(message_count), 0) FROM conversation_stats`).Scan(&stats.TotalMessages)
	if err != nil {
		// If table doesn't exist, default to 0
		stats.TotalMessages = 0
	}

	// Get total calls
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM calls`).Scan(&stats.TotalCalls)
	if err != nil {
		stats.TotalCalls = 0
	}

	// Get storage used
	err = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(storage_quota_used), 0) FROM users`).Scan(&stats.StorageUsed)
	if err != nil {
		stats.StorageUsed = 0
	}

	// Get database size (approximate)
	err = r.db.QueryRow(ctx, `SELECT pg_database_size('secureconnect')`).Scan(&stats.DatabaseSize)
	if err != nil {
		stats.DatabaseSize = 0
	}

	return stats, nil
}

// GetUsers retrieves a paginated list of users
func (r *AdminRepository) GetUsers(ctx context.Context, req *domain.UserListRequest) (*domain.UserListResponse, error) {
	// Build query
	query := `
		SELECT u.user_id, u.email, u.username, u.display_name, u.avatar_url,
		       u.status, u.role, u.created_at, u.last_login_at,
		       ub.banned_at, ub.ban_reason
		FROM users u
		LEFT JOIN user_bans ub ON u.user_id = ub.user_id AND ub.is_active = true
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	// Add search filter
	if req.Search != "" {
		query += fmt.Sprintf(" AND (u.email ILIKE $%d OR u.username ILIKE $%d)", argCount, argCount+1)
		args = append(args, "%"+req.Search+"%", "%"+req.Search+"%")
		argCount += 2
	}

	// Add status filter
	if req.Status != "" && req.Status != "all" {
		query += fmt.Sprintf(" AND u.status = $%d", argCount)
		args = append(args, req.Status)
		argCount++
	}

	// Add sorting with whitelist validation
	validSortColumns := map[string]bool{
		"created_at":    true,
		"email":         true,
		"username":      true,
		"status":        true,
		"last_login_at": true,
	}

	sortBy := "created_at"
	if req.SortBy != "" && validSortColumns[req.SortBy] {
		sortBy = req.SortBy
	} else if req.SortBy != "" {
		// Log invalid sort column attempt but fall back to default
		// This prevents SQL injection even if parameterization fails (defense in depth)
		return nil, fmt.Errorf("invalid sort column: %s", req.SortBy)
	}

	sortOrder := "DESC"
	if req.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY u.%s %s", sortBy, sortOrder)

	// Add pagination
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, req.Limit, req.Offset)

	// Execute query
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []domain.UserInfo
	for rows.Next() {
		var u domain.UserInfo
		var bannedAt *time.Time
		var banReason *string

		err := rows.Scan(
			&u.UserID,
			&u.Email,
			&u.Username,
			&u.DisplayName,
			&u.AvatarURL,
			&u.Status,
			&u.Role,
			&u.CreatedAt,
			&u.LastLoginAt,
			&bannedAt,
			&banReason,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		u.IsBanned = bannedAt != nil
		u.BannedAt = bannedAt
		u.BanReason = banReason

		users = append(users, u)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM users WHERE 1=1`
	countArgs := []interface{}{}
	countArgCount := 1

	if req.Search != "" {
		countQuery += fmt.Sprintf(" AND (email ILIKE $%d OR username ILIKE $%d)", countArgCount, countArgCount+1)
		countArgs = append(countArgs, "%"+req.Search+"%", "%"+req.Search+"%")
		countArgCount += 2
	}

	if req.Status != "" && req.Status != "all" {
		countQuery += fmt.Sprintf(" AND status = $%d", countArgCount)
		countArgs = append(countArgs, req.Status)
	}

	var totalCount int
	err = r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	hasMore := (req.Offset + len(users)) < totalCount

	return &domain.UserListResponse{
		Users:      users,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}, nil
}

// BanUser bans a user
func (r *AdminRepository) BanUser(ctx context.Context, req *domain.BanUserRequest, adminID uuid.UUID, ip string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create ban record
	var bannedUntil *time.Time
	if !req.Permanent {
		bannedAt := time.Now().Add(time.Duration(req.Duration) * time.Hour)
		bannedUntil = &bannedAt
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_bans (user_id, banned_by, ban_reason, banned_at, banned_until, is_active)
		VALUES ($1, $2, $3, NOW(), $4, true)
		ON CONFLICT (user_id, is_active) DO UPDATE SET
			banned_at = NOW(),
			banned_until = $4,
			ban_reason = EXCLUDED.ban_reason,
			banned_by = EXCLUDED.banned_by,
			is_active = true
	`, req.UserID, adminID, req.Reason, bannedUntil)
	if err != nil {
		return fmt.Errorf("failed to create ban record: %w", err)
	}

	// Update user status
	_, err = tx.Exec(ctx, `
		UPDATE users SET status = 'banned' WHERE user_id = $1
	`, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Create audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (admin_id, action, target_type, target_id, ip_address, details, created_at)
		VALUES ($1, 'ban_user', 'user', $2, $3, $4, NOW())
	`, adminID, req.UserID, ip, req.Reason)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return tx.Commit(ctx)
}

// UnbanUser unbans a user
func (r *AdminRepository) UnbanUser(ctx context.Context, req *domain.UnbanUserRequest, adminID uuid.UUID, ip string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Deactivate ban
	_, err = tx.Exec(ctx, `
		UPDATE user_bans SET is_active = false, unbanned_at = NOW()
		WHERE user_id = $1 AND is_active = true
	`, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to deactivate ban: %w", err)
	}

	// Update user status
	_, err = tx.Exec(ctx, `
		UPDATE users SET status = 'offline' WHERE user_id = $1
	`, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Create audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (admin_id, action, target_type, target_id, ip_address, details, created_at)
		VALUES ($1, 'unban_user', 'user', $2, $3, $4, NOW())
	`, adminID, req.UserID, ip, req.Reason)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return tx.Commit(ctx)
}

// GetAuditLogs retrieves audit logs
func (r *AdminRepository) GetAuditLogs(ctx context.Context, req *domain.AuditLogRequest) ([]domain.AuditLog, int, error) {
	// Build query
	query := `
		SELECT audit_id, admin_id, action, target_type, target_id,
		       ip_address, user_agent, details, created_at
		FROM audit_logs
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	// Add filters
	if req.AdminID != nil {
		query += fmt.Sprintf(" AND admin_id = $%d", argCount)
		args = append(args, req.AdminID)
		argCount++
	}

	if req.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argCount)
		args = append(args, req.Action)
		argCount++
	}

	if req.TargetType != "" {
		query += fmt.Sprintf(" AND target_type = $%d", argCount)
		args = append(args, req.TargetType)
		argCount++
	}

	if req.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, req.StartDate)
		argCount++
	}

	if req.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, req.EndDate)
		argCount++
	}

	// Add ordering and pagination
	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, req.Limit, req.Offset)

	// Execute query
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var log domain.AuditLog
		err := rows.Scan(
			&log.AuditID,
			&log.AdminID,
			&log.Action,
			&log.TargetType,
			&log.TargetID,
			&log.IPAddress,
			&log.UserAgent,
			&log.Details,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	countArgs := []interface{}{}
	countArgCount := 1

	if req.AdminID != nil {
		countQuery += fmt.Sprintf(" AND admin_id = $%d", countArgCount)
		countArgs = append(countArgs, req.AdminID)
		countArgCount++
	}

	if req.Action != "" {
		countQuery += fmt.Sprintf(" AND action = $%d", countArgCount)
		countArgs = append(countArgs, req.Action)
		countArgCount++
	}

	if req.TargetType != "" {
		countQuery += fmt.Sprintf(" AND target_type = $%d", countArgCount)
		countArgs = append(countArgs, req.TargetType)
		countArgCount++
	}

	if req.StartDate != nil {
		countQuery += fmt.Sprintf(" AND created_at >= $%d", countArgCount)
		countArgs = append(countArgs, req.StartDate)
		countArgCount++
	}

	if req.EndDate != nil {
		countQuery += fmt.Sprintf(" AND created_at <= $%d", countArgCount)
		countArgs = append(countArgs, req.EndDate)
		countArgCount++
	}

	var totalCount int
	err = r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	return logs, totalCount, nil
}

// CheckAdminRole checks if a user has admin role
func (r *AdminRepository) CheckAdminRole(ctx context.Context, userID uuid.UUID) (bool, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role FROM users WHERE user_id = $1`, userID).Scan(&role)
	if err != nil {
		return false, fmt.Errorf("failed to check admin role: %w", err)
	}
	return role == "admin", nil
}

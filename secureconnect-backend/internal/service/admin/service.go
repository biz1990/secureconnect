package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/repository/cockroach"
)

// Service handles administrative business logic
type Service struct {
	adminRepo *cockroach.AdminRepository
}

// NewService creates a new admin service
func NewService(adminRepo *cockroach.AdminRepository) *Service {
	return &Service{
		adminRepo: adminRepo,
	}
}

// GetSystemStats retrieves overall system statistics
func (s *Service) GetSystemStats(ctx context.Context) (*domain.SystemStats, error) {
	stats, err := s.adminRepo.GetSystemStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system stats: %w", err)
	}
	return stats, nil
}

// GetUsers retrieves a paginated list of users
func (s *Service) GetUsers(ctx context.Context, req *domain.UserListRequest) (*domain.UserListResponse, error) {
	// Set defaults
	if req.Limit == 0 {
		req.Limit = 50
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortOrder == "" {
		req.SortOrder = "DESC"
	}

	users, err := s.adminRepo.GetUsers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

// BanUser bans a user from the platform
func (s *Service) BanUser(ctx context.Context, adminID uuid.UUID, req *domain.BanUserRequest, ipAddress string) error {
	// Validate request
	if req.UserID == adminID {
		return fmt.Errorf("cannot ban yourself")
	}

	err := s.adminRepo.BanUser(ctx, req, adminID, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}
	return nil
}

// UnbanUser unbans a user
func (s *Service) UnbanUser(ctx context.Context, adminID uuid.UUID, req *domain.UnbanUserRequest, ipAddress string) error {
	err := s.adminRepo.UnbanUser(ctx, req, adminID, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to unban user: %w", err)
	}
	return nil
}

// GetAuditLogs retrieves audit logs
func (s *Service) GetAuditLogs(ctx context.Context, req *domain.AuditLogRequest) ([]domain.AuditLog, int, error) {
	// Set defaults
	if req.Limit == 0 {
		req.Limit = 50
	}
	if req.Limit > 200 {
		req.Limit = 200
	}

	logs, totalCount, err := s.adminRepo.GetAuditLogs(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get audit logs: %w", err)
	}
	return logs, totalCount, nil
}

// CheckAdminRole verifies if a user has admin privileges
func (s *Service) CheckAdminRole(ctx context.Context, userID uuid.UUID) (bool, error) {
	isAdmin, err := s.adminRepo.CheckAdminRole(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check admin role: %w", err)
	}
	return isAdmin, nil
}

// GetSystemHealth retrieves health status of all system components
func (s *Service) GetSystemHealth(ctx context.Context) (*domain.SystemHealth, error) {
	health := &domain.SystemHealth{
		Services:  make(map[string]domain.ServiceHealth),
		CheckedAt: s.getCurrentTime(),
	}

	// Check database health
	dbHealth := s.checkDatabaseHealth(ctx)
	health.Services["database"] = dbHealth

	// Check cache health
	cacheHealth := s.checkCacheHealth(ctx)
	health.Services["cache"] = cacheHealth

	// Check storage health
	storageHealth := s.checkStorageHealth(ctx)
	health.Services["storage"] = storageHealth

	// Determine overall status
	overallStatus := "healthy"
	for _, svc := range health.Services {
		if svc.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		} else if svc.Status == "degraded" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
		}
	}
	health.OverallStatus = overallStatus

	return health, nil
}

// checkDatabaseHealth checks database health
func (s *Service) checkDatabaseHealth(ctx context.Context) domain.ServiceHealth {
	// This would be implemented with actual health checks
	return domain.ServiceHealth{
		ServiceName: "database",
		Status:      "healthy",
		LastCheck:   s.getCurrentTime(),
		Metrics: map[string]interface{}{
			"connection_pool": "active",
		},
	}
}

// checkCacheHealth checks cache health
func (s *Service) checkCacheHealth(ctx context.Context) domain.ServiceHealth {
	// This would be implemented with actual health checks
	return domain.ServiceHealth{
		ServiceName: "cache",
		Status:      "healthy",
		LastCheck:   s.getCurrentTime(),
		Metrics: map[string]interface{}{
			"connection": "active",
		},
	}
}

// checkStorageHealth checks storage health
func (s *Service) checkStorageHealth(ctx context.Context) domain.ServiceHealth {
	// This would be implemented with actual health checks
	return domain.ServiceHealth{
		ServiceName: "storage",
		Status:      "healthy",
		LastCheck:   s.getCurrentTime(),
		Metrics: map[string]interface{}{
			"minio": "reachable",
		},
	}
}

// getCurrentTime returns current time (for easier mocking in tests)
func (s *Service) getCurrentTime() time.Time {
	return time.Now()
}

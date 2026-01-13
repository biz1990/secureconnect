package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/domain"
	"secureconnect-backend/internal/service/admin"
	"secureconnect-backend/pkg/response"
)

// Handler handles admin HTTP requests
type Handler struct {
	adminService *admin.Service
}

// NewHandler creates a new admin handler
func NewHandler(adminService *admin.Service) *Handler {
	return &Handler{
		adminService: adminService,
	}
}

// requireAdmin is middleware to check if user is admin
func (h *Handler) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "Not authenticated")
			c.Abort()
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			response.InternalError(c, "Invalid user ID")
			c.Abort()
			return
		}

		isAdmin, err := h.adminService.CheckAdminRole(c.Request.Context(), userID)
		if err != nil || !isAdmin {
			response.Forbidden(c, "Admin privileges required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetSystemStats retrieves system statistics
// GET /v1/admin/stats
func (h *Handler) GetSystemStats(c *gin.Context) {
	stats, err := h.adminService.GetSystemStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to get system stats")
		return
	}

	response.Success(c, http.StatusOK, stats)
}

// GetUsers retrieves list of users
// GET /v1/admin/users
func (h *Handler) GetUsers(c *gin.Context) {
	// Parse query parameters
	req := &domain.UserListRequest{
		Limit:     50,
		Offset:    0,
		SortBy:    "created_at",
		SortOrder: "DESC",
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = o
		}
	}

	if search := c.Query("search"); search != "" {
		req.Search = search
	}

	if status := c.Query("status"); status != "" {
		req.Status = status
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		req.SortBy = sortBy
	}

	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		req.SortOrder = sortOrder
	}

	users, err := h.adminService.GetUsers(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, "Failed to get users")
		return
	}

	response.Success(c, http.StatusOK, users)
}

// BanUser bans a user
// POST /v1/admin/users/ban
func (h *Handler) BanUser(c *gin.Context) {
	var req domain.BanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get admin ID from context
	adminIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Get IP address
	ipAddress := c.ClientIP()

	err := h.adminService.BanUser(c.Request.Context(), adminID, &req, ipAddress)
	if err != nil {
		if err.Error() == "cannot ban yourself" {
			response.ValidationError(c, err.Error())
			return
		}
		response.InternalError(c, "Failed to ban user")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "User banned successfully",
	})
}

// UnbanUser unbans a user
// POST /v1/admin/users/unban
func (h *Handler) UnbanUser(c *gin.Context) {
	var req domain.UnbanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get admin ID from context
	adminIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Get IP address
	ipAddress := c.ClientIP()

	err := h.adminService.UnbanUser(c.Request.Context(), adminID, &req, ipAddress)
	if err != nil {
		response.InternalError(c, "Failed to unban user")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "User unbanned successfully",
	})
}

// GetAuditLogs retrieves audit logs
// GET /v1/admin/audit-logs
func (h *Handler) GetAuditLogs(c *gin.Context) {
	// Parse query parameters
	req := &domain.AuditLogRequest{
		Limit:  50,
		Offset: 0,
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = o
		}
	}

	if action := c.Query("action"); action != "" {
		req.Action = action
	}

	if targetType := c.Query("target_type"); targetType != "" {
		req.TargetType = targetType
	}

	logs, totalCount, err := h.adminService.GetAuditLogs(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, "Failed to get audit logs")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"audit_logs":  logs,
		"total_count": totalCount,
		"limit":       req.Limit,
		"offset":      req.Offset,
	})
}

// GetSystemHealth retrieves system health status
// GET /v1/admin/health
func (h *Handler) GetSystemHealth(c *gin.Context) {
	health, err := h.adminService.GetSystemHealth(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to get system health")
		return
	}

	// Return appropriate HTTP status based on health
	statusCode := http.StatusOK
	if health.OverallStatus == "degraded" {
		statusCode = http.StatusMultiStatus // 207
	} else if health.OverallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable // 503
	}

	c.JSON(statusCode, gin.H{
		"success": true,
		"data":    health,
	})
}

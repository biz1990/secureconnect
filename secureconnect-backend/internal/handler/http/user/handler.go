package user

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"secureconnect-backend/internal/service/user"
	"secureconnect-backend/pkg/response"
)

// Handler handles user management HTTP requests
type Handler struct {
	userService *user.Service
}

// NewHandler creates a new user handler
func NewHandler(userService *user.Service) *Handler {
	return &Handler{
		userService: userService,
	}
}

// GetProfileRequest represents profile update request
type GetProfileRequest struct {
	Email       *string `json:"email,omitempty"`
	Username    *string `json:"username,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

// UpdateProfileRequest represents profile update request
type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name" binding:"omitempty,min=1,max=100"`
	AvatarURL   *string `json:"avatar_url" binding:"omitempty,url"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangeEmailRequest represents email change request
type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// BlockUserRequest represents block user request
type BlockUserRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

// GetProfile returns current user profile
// GET /v1/users/me
func (h *Handler) GetProfile(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Get user profile from database
	user, err := h.userService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to get profile")
		return
	}

	// Return user info
	response.Success(c, http.StatusOK, user)
}

// UpdateProfile updates current user profile
// PATCH /v1/users/me
func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Update profile
	err := h.userService.UpdateProfile(c.Request.Context(), userID, req.DisplayName, req.AvatarURL)
	if err != nil {
		response.InternalError(c, "Failed to update profile")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Profile updated successfully",
	})
}

// ChangePassword changes user password
// POST /v1/users/me/password
func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Change password
	err := h.userService.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		if err.Error() == "invalid old password" {
			response.Unauthorized(c, "Invalid old password")
			return
		}
		response.InternalError(c, "Failed to change password")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}

// ChangeEmail initiates email change (requires verification)
// POST /v1/users/me/email
func (h *Handler) ChangeEmail(c *gin.Context) {
	var req ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Initiate email change (sends verification email)
	err := h.userService.InitiateEmailChange(c.Request.Context(), userID, req.NewEmail, req.Password)
	if err != nil {
		response.InternalError(c, "Failed to initiate email change")
		return
	}

	response.Success(c, http.StatusAccepted, gin.H{
		"message": "Verification email sent",
	})
}

// VerifyEmail verifies email change with token
// POST /v1/users/me/email/verify
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Verify email change
	err := h.userService.VerifyEmailChange(c.Request.Context(), userID, req.Token)
	if err != nil {
		response.Unauthorized(c, "Invalid or expired token")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Email changed successfully",
	})
}

// DeleteAccount deletes user account
// DELETE /v1/users/me
func (h *Handler) DeleteAccount(c *gin.Context) {
	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Delete account
	err := h.userService.DeleteAccount(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to delete account")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Account deleted successfully",
	})
}

// GetBlockedUsers returns list of blocked users
// GET /v1/users/me/blocked
func (h *Handler) GetBlockedUsers(c *gin.Context) {
	// Get pagination parameters
	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Get blocked users
	blockedUsers, err := h.userService.GetBlockedUsers(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.InternalError(c, "Failed to get blocked users")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"blocked_users": blockedUsers,
		"limit":         limit,
		"offset":        offset,
	})
}

// BlockUser blocks another user
// POST /v1/users/:id/block
func (h *Handler) BlockUser(c *gin.Context) {
	targetUserIDStr := c.Param("id")

	targetUserID, err := uuid.Parse(targetUserIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid user ID")
		return
	}

	// Get requesting user ID from context
	requestingUserIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	requestingUserID, ok := requestingUserIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	var req BlockUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Block user
	err = h.userService.BlockUser(c.Request.Context(), requestingUserID, targetUserID, req.Reason)
	if err != nil {
		response.InternalError(c, "Failed to block user")
		return
	}

	response.Success(c, http.StatusCreated, gin.H{
		"message": "User blocked successfully",
	})
}

// UnblockUser unblocks a user
// DELETE /v1/users/:id/block
func (h *Handler) UnblockUser(c *gin.Context) {
	targetUserIDStr := c.Param("id")

	targetUserID, err := uuid.Parse(targetUserIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid user ID")
		return
	}

	// Get requesting user ID from context
	requestingUserIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	requestingUserID, ok := requestingUserIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Unblock user
	err = h.userService.UnblockUser(c.Request.Context(), requestingUserID, targetUserID)
	if err != nil {
		response.InternalError(c, "Failed to unblock user")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "User unblocked successfully",
	})
}

// GetFriends returns list of friends
// GET /v1/users/me/friends
func (h *Handler) GetFriends(c *gin.Context) {
	// Get pagination parameters
	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Get friends
	friends, err := h.userService.GetFriends(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.InternalError(c, "Failed to get friends")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"friends": friends,
		"limit":   limit,
		"offset":  offset,
	})
}

// SendFriendRequest sends a friend request
// POST /v1/users/:id/friend
func (h *Handler) SendFriendRequest(c *gin.Context) {
	targetUserIDStr := c.Param("id")

	targetUserID, err := uuid.Parse(targetUserIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid user ID")
		return
	}

	// Get requesting user ID from context
	requestingUserIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	requestingUserID, ok := requestingUserIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Send friend request
	err = h.userService.SendFriendRequest(c.Request.Context(), requestingUserID, targetUserID)
	if err != nil {
		response.InternalError(c, "Failed to send friend request")
		return
	}

	response.Success(c, http.StatusCreated, gin.H{
		"message": "Friend request sent",
	})
}

// AcceptFriendRequest accepts a friend request
// POST /v1/users/me/friends/:id/accept
func (h *Handler) AcceptFriendRequest(c *gin.Context) {
	requestIDStr := c.Param("id")

	requestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid request ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Accept friend request
	err = h.userService.AcceptFriendRequest(c.Request.Context(), userID, requestID)
	if err != nil {
		response.InternalError(c, "Failed to accept friend request")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Friend request accepted",
	})
}

// RejectFriendRequest rejects a friend request
// DELETE /v1/users/me/friends/:id/reject
func (h *Handler) RejectFriendRequest(c *gin.Context) {
	requestIDStr := c.Param("id")

	requestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid request ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Reject friend request
	err = h.userService.RejectFriendRequest(c.Request.Context(), userID, requestID)
	if err != nil {
		response.InternalError(c, "Failed to reject friend request")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Friend request rejected",
	})
}

// Unfriend removes a friend
// DELETE /v1/users/me/friends/:id
func (h *Handler) Unfriend(c *gin.Context) {
	friendIDStr := c.Param("id")

	friendID, err := uuid.Parse(friendIDStr)
	if err != nil {
		response.ValidationError(c, "Invalid friend ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID")
		return
	}

	// Unfriend
	err = h.userService.Unfriend(c.Request.Context(), userID, friendID)
	if err != nil {
		response.InternalError(c, "Failed to unfriend")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"message": "Friend removed",
	})
}

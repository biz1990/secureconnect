package domain

import (
	"time"
	
	"github.com/google/uuid"
)

// User represents a user entity in the system
// Maps to CockroachDB users table
type User struct {
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Email       string    `json:"email" db:"email"`
	Username    string    `json:"username" db:"username"`
	PasswordHash string   `json:"-" db:"password_hash"` // Never expose in JSON
	DisplayName string    `json:"display_name" db:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty" db:"avatar_url"`
	Status      string    `json:"status" db:"status"` // online, offline, busy
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// UserCreate represents data needed to create a new user
type UserCreate struct {
	Email       string `json:"email" binding:"required,email"`
	Username    string `json:"username" binding:"required,min=3,max=30"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
}

// UserLogin represents login credentials
type UserLogin struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse is the safe user representation returned to clients
type UserResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// ToResponse converts User to UserResponse (removes sensitive data)
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		UserID:      u.UserID,
		Email:       u.Email,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		Status:      u.Status,
		CreatedAt:   u.CreatedAt,
	}
}

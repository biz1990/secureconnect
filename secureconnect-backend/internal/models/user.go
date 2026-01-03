package models

import (
    "time"

    "github.com/google/uuid"
)

type User struct {
    UserID    uuid.UUID `json:"user_id" db:"user_id"`
    Email     string    `json:"email" db:"email"`
    Password  string    `json:"-" db:"password"` // Không trả về password trong JSON
    FullName   string    `json:"full_name" db:"full_name"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}
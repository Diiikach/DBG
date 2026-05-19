package models

import "time"

// User — пользователь системы (соответствует таблице public."user").
type User struct {
	UserID    int        `json:"user_id"`
	Username  string     `json:"username"`
	Email     *string    `json:"email,omitempty"`
	FirstName *string    `json:"first_name,omitempty"`
	LastName  *string    `json:"last_name,omitempty"`
	Role      string     `json:"role"`
	IsActive  bool       `json:"is_active"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// UserCreate — payload для регистрации пользователя.
type UserCreate struct {
	Username  string  `json:"username"`
	Password  string  `json:"password"`
	Email     *string `json:"email,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Role      *string `json:"role,omitempty"` // optional, default "viewer"
}

// Credentials — payload для логина.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse — ответ ручек /auth/login и /auth/register.
type AuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"` // unix seconds
	User      User   `json:"user"`
}

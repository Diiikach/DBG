package models

import "time"

// Role corresponds to SQL enum role in user table

type UserRole string

const (
	RoleAdmin      UserRole = "admin"
	RoleDoctor     UserRole = "doctor"
	RoleResearcher UserRole = "researcher"
	RoleViewer     UserRole = "viewer"
)

// User represents a system user

type User struct {
	UserID       int        `db:"user_id" json:"user_id"`
	Username     string     `db:"username" json:"username"`
	Email        string     `db:"email" json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"`
	FullName     string     `db:"full_name" json:"full_name"`
	Role         UserRole   `db:"role" json:"role"`
	Institution  *string    `db:"institution" json:"institution,omitempty"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	LastLogin    *time.Time `db:"last_login" json:"last_login,omitempty"`
}

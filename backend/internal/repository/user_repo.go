package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/term-paper-2026/backend/internal/auth"
	"github.com/term-paper-2026/backend/internal/models"
)

// ErrUserExists — username/email уже занят.
var ErrUserExists = errors.New("user already exists")

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(p *pgxpool.Pool) *UserRepo { return &UserRepo{pool: p} }

const userSelectCols = `user_id, username, email, first_name, last_name,
		role::text, is_active, created_at, updated_at`

// Create вставляет нового пользователя. Хэширует пароль bcrypt'ом.
func (r *UserRepo) Create(ctx context.Context, in models.UserCreate) (models.User, error) {
	role := "viewer"
	if in.Role != nil && *in.Role != "" {
		role = *in.Role
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	const q = `
		INSERT INTO public."user"
			(username, email, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5, $6::user_role)
		RETURNING ` + userSelectCols
	args := []any{in.Username, in.Email, hash, in.FirstName, in.LastName, role}
	start := time.Now()

	var u models.User
	err = r.pool.QueryRow(ctx, q, args...).Scan(
		&u.UserID, &u.Username, &u.Email, &u.FirstName, &u.LastName,
		&u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	logQuery(ctx, "user.Create", q, args, err, start)
	if err != nil {
		// 23505 — unique_violation.
		if isUniqueViolation(err) {
			return models.User{}, ErrUserExists
		}
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

// GetByUsername возвращает пользователя и его password_hash.
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (models.User, string, error) {
	const q = `
		SELECT ` + userSelectCols + `, COALESCE(password_hash, '')
		FROM public."user" WHERE username = $1
	`
	args := []any{username}
	start := time.Now()

	var u models.User
	var hash string
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&u.UserID, &u.Username, &u.Email, &u.FirstName, &u.LastName,
		&u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt, &hash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		logQuery(ctx, "user.GetByUsername", q, args, nil, start)
		return models.User{}, "", ErrNotFound
	}
	logQuery(ctx, "user.GetByUsername", q, args, err, start)
	if err != nil {
		return models.User{}, "", err
	}
	return u, hash, nil
}

// GetByID возвращает пользователя по id.
func (r *UserRepo) GetByID(ctx context.Context, id int) (models.User, error) {
	const q = `SELECT ` + userSelectCols + ` FROM public."user" WHERE user_id = $1`
	args := []any{id}
	start := time.Now()

	var u models.User
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&u.UserID, &u.Username, &u.Email, &u.FirstName, &u.LastName,
		&u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		logQuery(ctx, "user.GetByID", q, args, nil, start)
		return models.User{}, ErrNotFound
	}
	logQuery(ctx, "user.GetByID", q, args, err, start)
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}

// isUniqueViolation проверяет SQLSTATE 23505.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

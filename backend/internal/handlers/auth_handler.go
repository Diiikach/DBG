package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/term-paper-2026/backend/internal/auth"
	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/models"
	"github.com/term-paper-2026/backend/internal/repository"
)

// emailRegexp — простая регулярка для проверки формата email (ТЗ 4.1.1 п.1).
var emailRegexp = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// ClaimsProvider — функция, доставающая *auth.Claims из контекста запроса.
// Нужна, чтобы handlers не зависел от пакета router.
type ClaimsProvider func(r *http.Request) *auth.Claims

type AuthHandler struct {
	repo      *repository.UserRepo
	issuer    *auth.Issuer
	getClaims ClaimsProvider
}

func NewAuthHandler(repo *repository.UserRepo, issuer *auth.Issuer, getClaims ClaimsProvider) *AuthHandler {
	return &AuthHandler{repo: repo, issuer: issuer, getClaims: getClaims}
}

// Register — POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.FromContext(ctx).With(slog.String("handler", "auth.Register"))

	var in models.UserCreate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		writeError(ctx, w, http.StatusUnprocessableEntity, "username is required")
		return
	}
	if len(in.Password) < 6 {
		writeError(ctx, w, http.StatusUnprocessableEntity, "password must be at least 6 chars")
		return
	}
	// Email необязателен, но если передан — проверяем формат.
	if in.Email != nil && strings.TrimSpace(*in.Email) != "" {
		trimmedEmail := strings.TrimSpace(*in.Email)
		if !emailRegexp.MatchString(trimmedEmail) {
			writeError(ctx, w, http.StatusUnprocessableEntity, "invalid email format")
			return
		}
		in.Email = &trimmedEmail
	}

	u, err := h.repo.Create(ctx, in)
	if err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			log.LogAttrs(ctx, slog.LevelWarn, "register: user exists",
				slog.String("username", in.Username))
			writeError(ctx, w, http.StatusConflict, "user already exists")
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}

	tok, exp, err := h.issuer.Issue(u.UserID, u.Username, u.Role)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "issue token: "+err.Error())
		return
	}
	log.LogAttrs(ctx, slog.LevelInfo, "user registered",
		slog.Int("user_id", u.UserID), slog.String("username", u.Username))

	writeJSON(ctx, w, http.StatusCreated, models.AuthResponse{
		Token: tok, ExpiresAt: exp.Unix(), User: u,
	})
}

// Login — POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.FromContext(ctx).With(slog.String("handler", "auth.Login"))

	var in models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if in.Username == "" || in.Password == "" {
		writeError(ctx, w, http.StatusBadRequest, "username and password required")
		return
	}

	u, hash, err := h.repo.GetByUsername(ctx, in.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			log.LogAttrs(ctx, slog.LevelWarn, "login: user not found",
				slog.String("username", in.Username))
			writeError(ctx, w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	if !u.IsActive {
		log.LogAttrs(ctx, slog.LevelWarn, "login: inactive user",
			slog.Int("user_id", u.UserID))
		writeError(ctx, w, http.StatusForbidden, "user is inactive")
		return
	}
	if err := auth.CheckPassword(hash, in.Password); err != nil {
		log.LogAttrs(ctx, slog.LevelWarn, "login: bad password",
			slog.Int("user_id", u.UserID), slog.String("username", u.Username))
		writeError(ctx, w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	tok, exp, err := h.issuer.Issue(u.UserID, u.Username, u.Role)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "issue token: "+err.Error())
		return
	}
	log.LogAttrs(ctx, slog.LevelInfo, "login ok",
		slog.Int("user_id", u.UserID), slog.String("username", u.Username))

	writeJSON(ctx, w, http.StatusOK, models.AuthResponse{
		Token: tok, ExpiresAt: exp.Unix(), User: u,
	})
}

// Me — GET /api/auth/me. Требует RequireAuth.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := h.getClaims(r)
	if c == nil {
		writeError(ctx, w, http.StatusUnauthorized, "no claims in context")
		return
	}
	u, err := h.repo.GetByID(ctx, c.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, http.StatusNotFound, "user not found")
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, u)
}

package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

type authRegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type authLoginInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  userProfile `json:"user"`
}

type userProfile struct {
	UserID      int     `json:"user_id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	FullName    string  `json:"full_name"`
	Role        string  `json:"role"`
	Institution *string `json:"institution,omitempty"`
}

func (a *App) authRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var in authRegisterInput
	if err := readJSON(r.Body, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	in.Username = normalizeUsername(in.Username)
	in.Email = normalizeEmail(in.Email)
	in.FullName = strings.TrimSpace(in.FullName)
	in.Role = strings.TrimSpace(in.Role)
	fieldErrs := map[string]string{}
	if in.Username == "" {
		fieldErrs["username"] = "обязательно"
	}
	if in.Email == "" {
		fieldErrs["email"] = "обязательно"
	} else if !isEmailValid(in.Email) {
		fieldErrs["email"] = "некорректный email"
	}
	if in.Password == "" {
		fieldErrs["password"] = "обязательно"
	} else if len(in.Password) < 8 {
		fieldErrs["password"] = "минимум 8 символов"
	}
	if in.FullName == "" {
		fieldErrs["full_name"] = "обязательно"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
		return
	}
	role := in.Role
	if role == "" {
		role = "doctor"
	}

	var exists bool
	_ = a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM "user" WHERE lower(username)=lower($1))`, in.Username).Scan(&exists)
	if exists {
		fieldErrs["username"] = "уже используется"
	}
	exists = false
	_ = a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM "user" WHERE lower(email)=lower($1))`, in.Email).Scan(&exists)
	if exists {
		fieldErrs["email"] = "уже используется"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "password hash failed"})
		return
	}

	var user userProfile
	err = a.db.QueryRow(`
		INSERT INTO "user" (username, email, password_hash, full_name, role)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING user_id, username, email, full_name, role, institution`,
		in.Username, in.Email, string(hash), in.FullName, role).Scan(
		&user.UserID, &user.Username, &user.Email, &user.FullName, &user.Role, &user.Institution,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "user already exists"})
		return
	}

	token, err := a.createSession(user.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session create failed"})
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

func (a *App) authLoginHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var in authLoginInput
	if err := readJSON(r.Body, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	in.Username = normalizeUsername(in.Username)
	in.Email = normalizeEmail(in.Email)
	fieldErrs := map[string]string{}
	if in.Password == "" {
		fieldErrs["password"] = "обязательно"
	}
	if in.Username == "" && in.Email == "" {
		fieldErrs["login"] = "email или username обязательны"
	}
	if in.Email != "" && !isEmailValid(in.Email) {
		fieldErrs["email"] = "некорректный email"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
		return
	}

	var (
		user  userProfile
		hash  string
		query string
		arg   string
	)
	if in.Email != "" {
		query = `SELECT user_id, username, email, full_name, role, institution, password_hash FROM "user" WHERE email=$1`
		arg = in.Email
	} else {
		query = `SELECT user_id, username, email, full_name, role, institution, password_hash FROM "user" WHERE username=$1`
		arg = in.Username
	}
	err := a.db.QueryRow(query, arg).Scan(&user.UserID, &user.Username, &user.Email, &user.FullName, &user.Role, &user.Institution, &hash)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid credentials"})
		return
	}

	_, _ = a.db.Exec(`UPDATE "user" SET last_login = NOW() WHERE user_id = $1`, user.UserID)

	token, err := a.createSession(user.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session create failed"})
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func normalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	return strings.ToLower(email)
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}

func (a *App) authMeHandler(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	if userID == 0 {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	var user userProfile
	err := a.db.QueryRow(`
		SELECT user_id, username, email, full_name, role, institution
		FROM "user" WHERE user_id = $1`, userID).Scan(&user.UserID, &user.Username, &user.Email, &user.FullName, &user.Role, &user.Institution)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *App) authLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	token := bearerToken(r)
	if token == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	_, _ = a.db.Exec(`DELETE FROM user_session WHERE token = $1`, token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) createSession(userID int) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(30 * 24 * time.Hour)
	_, err = a.db.Exec(`INSERT INTO user_session (user_id, token, expires_at) VALUES ($1,$2,$3)`, userID, token, expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.db == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
			return
		}
		token := bearerToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}
		var userID int
		var expires time.Time
		err := a.db.QueryRow(`SELECT user_id, expires_at FROM user_session WHERE token = $1`, token).Scan(&userID, &expires)
		if errors.Is(err, sql.ErrNoRows) || err != nil || time.Now().After(expires) {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func userIDFromContext(ctx context.Context) int {
	v := ctx.Value(userIDKey)
	if v == nil {
		return 0
	}
	if id, ok := v.(int); ok {
		return id
	}
	return 0
}

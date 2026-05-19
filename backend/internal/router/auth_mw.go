package router

import (
	"context"
	"net/http"
	"strings"

	"github.com/term-paper-2026/backend/internal/auth"
)

// WithClaims — обёртка над auth.WithClaims для обратной совместимости.
func WithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return auth.WithClaims(ctx, c)
}

// ClaimsFromContext — обёртка над auth.ClaimsFromContext.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	return auth.ClaimsFromContext(ctx)
}

// UserIDFrom — обёртка над auth.UserIDFrom.
func UserIDFrom(ctx context.Context) int {
	return auth.UserIDFrom(ctx)
}

// RequireAuth — middleware, проверяющий Bearer JWT.
// Кладёт *auth.Claims в context для использования в хендлерах.
func RequireAuth(issuer *auth.Issuer) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				writeJSONErr(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}
			parts := strings.SplitN(h, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				writeJSONErr(w, http.StatusUnauthorized, "invalid Authorization header")
				return
			}
			c, err := issuer.Parse(parts[1])
			if err != nil {
				writeJSONErr(w, http.StatusUnauthorized, "invalid token: "+err.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithClaims(r.Context(), c)))
		})
	}
}

// writeJSONErr — маленький локальный помощник, чтобы не тащить handlers/common.
func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + jsonEscape(msg) + `"}`))
}

func jsonEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return r.Replace(s)
}

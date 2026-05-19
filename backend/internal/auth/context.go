package auth

import "context"

type claimsKey struct{}

// WithClaims кладёт claims в контекст.
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromContext достаёт claims из контекста (или nil).
func ClaimsFromContext(ctx context.Context) *Claims {
	if v, ok := ctx.Value(claimsKey{}).(*Claims); ok {
		return v
	}
	return nil
}

// UserIDFrom возвращает user_id из claims в контексте.
// Возвращает 0, если claims нет — хендлер должен трактовать это как 401.
func UserIDFrom(ctx context.Context) int {
	if c := ClaimsFromContext(ctx); c != nil {
		return c.UserID
	}
	return 0
}

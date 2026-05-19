package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/term-paper-2026/backend/internal/logging"
)

// slogRequestLogger — structured replacement for chi's middleware.Logger.
//
// Для каждого HTTP-запроса:
//   - на входе кладёт в context логгер, обогащённый request_id/method/path/remote_addr/ua;
//   - на выходе пишет одну строку уровня INFO с status/bytes/duration;
//   - повышает уровень до WARN для 4xx и до ERROR для 5xx.
//
// Полученный из контекста логгер можно использовать в любом handler/repo через
// logging.FromContext(ctx) — все записи автоматически будут содержать request_id.
func slogRequestLogger(base *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := middleware.GetReqID(r.Context())

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			reqLogger := base.With(
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)
			reqLogger.LogAttrs(r.Context(), slog.LevelDebug, "http request started",
				slog.String("query", r.URL.RawQuery),
			)

			// Прокидываем логгер дальше через context.
			ctx := logging.WithLogger(r.Context(), reqLogger)
			next.ServeHTTP(ww, r.WithContext(ctx))

			status := ww.Status()
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			reqLogger.LogAttrs(r.Context(), level, "http request completed",
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

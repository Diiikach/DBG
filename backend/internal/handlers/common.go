package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/term-paper-2026/backend/internal/logging"
)

// writeJSON отправляет JSON ответ. Ошибки сериализации (редкие) пишутся в
// логгер из контекста, чтобы не потеряться молча.
func writeJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logging.FromContext(ctx).LogAttrs(ctx, slog.LevelError,
			"encode response failed",
			slog.String("error", err.Error()),
			slog.Int("status", status),
		)
	}
}

// writeError отправляет JSON-ошибку и пишет её в лог. Уровень выбирается по
// HTTP-статусу: 4xx → WARN, 5xx → ERROR.
func writeError(ctx context.Context, w http.ResponseWriter, status int, msg string) {
	level := slog.LevelWarn
	if status >= 500 {
		level = slog.LevelError
	}
	logging.FromContext(ctx).LogAttrs(ctx, level, "request rejected",
		slog.Int("status", status),
		slog.String("error", msg),
	)
	writeJSON(ctx, w, status, map[string]string{"error": msg})
}

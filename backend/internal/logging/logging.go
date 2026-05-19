// Package logging предоставляет настроенный *slog.Logger для всего приложения.
//
// Уровень и формат настраиваются через переменные окружения:
//
//	LOG_LEVEL  = debug | info | warn | error   (default: info)
//	LOG_FORMAT = text | json                   (default: text)
//
// Поведение:
//   - text → человекочитаемый формат для локальной разработки;
//   - json → строки одной структурированной записью (для прод/сбора логов).
//
// Все логи пишутся в stdout; ошибки приложения дополнительно увеличивают
// поле level=ERROR, что упрощает фильтрацию.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Init создаёт *slog.Logger по конфигурации из переменных окружения и
// одновременно выставляет его глобальным дефолтом (slog.SetDefault), чтобы
// сторонние пакеты, использующие slog.Default(), писали туда же.
func Init() *slog.Logger {
	return InitTo(os.Stdout)
}

// InitTo — то же самое, но с возможностью указать произвольный io.Writer
// (используется в тестах).
func InitTo(w io.Writer) *slog.Logger {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level <= slog.LevelDebug, // источник дорого, только в debug
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}

	logger := slog.New(handler).With(
		slog.String("service", "variant-api"),
	)
	slog.SetDefault(logger)
	return logger
}

// parseLevel переводит строку уровня (любой регистр) в slog.Level.
// Неизвестные значения трактуются как info.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ---- Контекстный логгер ---------------------------------------------------

// ctxKey — приватный тип для ключей контекста.
type ctxKey struct{}

// WithLogger кладёт *slog.Logger в context, что позволяет передавать
// request-scoped поля (request_id, route, и т.п.) через стандартный context.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext достаёт *slog.Logger из контекста или возвращает slog.Default(),
// если в контексте логгера нет. Никогда не возвращает nil.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}

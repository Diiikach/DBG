package repository

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/term-paper-2026/backend/internal/logging"
)

// logQuery — единый helper, который пишет в slog результат SQL-операции.
// На DEBUG он печатает сам SQL и переданные параметры, на INFO — только имя
// операции и длительность, на ERROR — сообщение об ошибке.
//
// Использование:
//
//	defer func(start time.Time) { logQuery(ctx, "patient.Create", q, args, err, start) }(time.Now())
func logQuery(ctx context.Context, op, sql string, args []any, err error, start time.Time) {
	log := logging.FromContext(ctx).With(slog.String("repo_op", op))
	dur := time.Since(start)

	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "sql failed",
			slog.String("error", err.Error()),
			slog.Duration("duration", dur),
			slog.String("sql", compactSQL(sql)),
			slog.Int("args", len(args)),
		)
		return
	}

	log.LogAttrs(ctx, slog.LevelDebug, "sql executed",
		slog.Duration("duration", dur),
		slog.String("sql", compactSQL(sql)),
		slog.Int("args", len(args)),
	)
}

// compactSQL схлопывает повторяющиеся пробелы и переносы строк, чтобы SQL
// выводился в логе одной строкой и не ломал визуализацию JSON.
func compactSQL(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

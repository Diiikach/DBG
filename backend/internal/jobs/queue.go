// Package jobs реализует простую in-memory очередь фоновых задач с пулом воркеров.
//
// Очередь используется для асинхронной обработки sample-загрузок:
// HTTP-хендлер кладёт задачу в Submit() и сразу отвечает 202.
// Воркер забирает задачу и обновляет статус sample в БД.
package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/term-paper-2026/backend/internal/logging"
)

// Job — единица работы. Получает свой контекст; результат проверяется по err.
type Job func(ctx context.Context) error

// Queue — пул воркеров.
type Queue struct {
	ch     chan jobItem
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	closed bool
	mu     sync.Mutex
	log    *slog.Logger
}

type jobItem struct {
	name string
	job  Job
}

// ErrQueueClosed — очередь уже закрыта.
var ErrQueueClosed = errors.New("queue closed")

// New создаёт очередь и запускает workers воркеров.
func New(parent context.Context, log *slog.Logger, workers, buffer int) *Queue {
	if workers <= 0 {
		workers = 1
	}
	if buffer <= 0 {
		buffer = 16
	}
	ctx, cancel := context.WithCancel(parent)
	q := &Queue{
		ch:     make(chan jobItem, buffer),
		ctx:    ctx,
		cancel: cancel,
		log:    log,
	}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
	return q
}

// Submit ставит задачу в очередь. Блокируется, если буфер заполнен.
func (q *Queue) Submit(name string, job Job) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	q.mu.Unlock()

	select {
	case <-q.ctx.Done():
		return ErrQueueClosed
	case q.ch <- jobItem{name: name, job: job}:
		return nil
	}
}

// Shutdown сигналит воркерам остановиться и ждёт их завершения.
func (q *Queue) Shutdown() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.ch)
	q.mu.Unlock()
	q.wg.Wait()
	q.cancel()
}

func (q *Queue) worker(idx int) {
	defer q.wg.Done()
	wlog := q.log.With(slog.Int("worker", idx))
	for it := range q.ch {
		jobCtx := logging.WithLogger(q.ctx, wlog.With(slog.String("job", it.name)))
		wlog.LogAttrs(jobCtx, slog.LevelInfo, "job started", slog.String("job", it.name))
		err := safeRun(jobCtx, it.job)
		if err != nil {
			wlog.LogAttrs(jobCtx, slog.LevelError, "job failed",
				slog.String("job", it.name),
				slog.String("error", err.Error()))
		} else {
			wlog.LogAttrs(jobCtx, slog.LevelInfo, "job done",
				slog.String("job", it.name))
		}
	}
	wlog.LogAttrs(q.ctx, slog.LevelInfo, "worker stopped")
}

func safeRun(ctx context.Context, j Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("panic in job")
			logging.FromContext(ctx).LogAttrs(ctx, slog.LevelError, "job panic",
				slog.Any("recover", r))
		}
	}()
	return j(ctx)
}

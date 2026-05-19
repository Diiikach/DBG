// Точка входа REST API: auth + пациенты + sample-upload + варианты.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/term-paper-2026/backend/internal/auth"
	"github.com/term-paper-2026/backend/internal/config"
	"github.com/term-paper-2026/backend/internal/db"
	"github.com/term-paper-2026/backend/internal/enrichment"
	"github.com/term-paper-2026/backend/internal/handlers"
	"github.com/term-paper-2026/backend/internal/jobs"
	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/pipeline"
	"github.com/term-paper-2026/backend/internal/repository"
	"github.com/term-paper-2026/backend/internal/router"
)

// enrichScheduler — реализация handlers.EnrichScheduler:
// для каждого variant_id ставит фоновую задачу в очередь, в которой
// дергает Enricher.Enrich. Если очередь закрыта (shutdown) — глотает.
type enrichScheduler struct {
	queue    *jobs.Queue
	enricher *enrichment.Enricher
	log      *slog.Logger
}

func (s *enrichScheduler) ScheduleEnrich(variantIDs []int) {
	if s == nil || s.enricher == nil {
		return
	}
	for _, vid := range variantIDs {
		vidCopy := vid
		name := "enrich-" + strconv.Itoa(vidCopy)
		if err := s.queue.Submit(name, func(ctx context.Context) error {
			return s.enricher.Enrich(ctx, vidCopy)
		}); err != nil {
			s.log.Warn("submit enrich job failed",
				"variant_id", vidCopy, "error", err)
		}
	}
}

func main() {
	logger := logging.Init()
	cfg := config.Load()

	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		log.Fatalf("mkdir workdir: %v", err)
	}
	if err := os.MkdirAll(cfg.UploadsDir, 0o755); err != nil {
		log.Fatalf("mkdir uploads: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	// Repositories
	patientRepo := repository.NewPatientRepo(pool)
	variantRepo := repository.NewVariantRepo(pool)
	userRepo := repository.NewUserRepo(pool)

	// Auth
	issuer := auth.NewIssuer(cfg.JWTSecret, cfg.JWTTTL)

	// Pipeline + background jobs
	pipe := pipeline.New(cfg)
	queue := jobs.New(context.Background(), logger, cfg.JobsWorkers, cfg.JobsQueueSize)
	defer queue.Shutdown()

	// Enrichment (модуль 7).
	httpClient := &http.Client{Timeout: cfg.EnrichmentTimeout}
	myVarClient := enrichment.NewMyVariantClient(httpClient)
	enricher := enrichment.NewEnricher(variantRepo, myVarClient, cfg.EnrichmentEnabled, cfg.EnrichmentRPS)
	scheduler := &enrichScheduler{queue: queue, enricher: enricher, log: logger}

	// Handlers
	authH := handlers.NewAuthHandler(userRepo, issuer, func(r *http.Request) *auth.Claims {
		return router.ClaimsFromContext(r.Context())
	})
	patientH := handlers.NewPatientHandler(patientRepo)
	variantH := handlers.NewVariantHandler(patientRepo, variantRepo, pipe, scheduler, cfg)
	sampleH := handlers.NewSampleHandler(patientRepo, variantRepo, pipe, queue, scheduler, cfg)
	exportH := handlers.NewExportHandler(patientRepo, variantRepo)

	h := router.New(router.Deps{
		Logger:    logger,
		Issuer:    issuer,
		CORSOrigs: cfg.CORSOrigins,
		Auth:      authH,
		Patient:   patientH,
		Variant:   variantH,
		Sample:    sampleH,
		Export:    exportH,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("HTTP listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
}

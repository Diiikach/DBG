package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/term-paper-2026/backend/internal/auth"
	"github.com/term-paper-2026/backend/internal/config"
	"github.com/term-paper-2026/backend/internal/jobs"
	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/pipeline"
	"github.com/term-paper-2026/backend/internal/repository"
)

type SampleHandler struct {
	patientRepo *repository.PatientRepo
	variantRepo *repository.VariantRepo
	pipe        *pipeline.Pipeline
	queue       *jobs.Queue
	enricher    EnrichScheduler
	cfg         config.Config
}

func NewSampleHandler(
	pr *repository.PatientRepo,
	vr *repository.VariantRepo,
	p *pipeline.Pipeline,
	q *jobs.Queue,
	enricher EnrichScheduler,
	cfg config.Config,
) *SampleHandler {
	return &SampleHandler{patientRepo: pr, variantRepo: vr, pipe: p, queue: q, enricher: enricher, cfg: cfg}
}

// Upload — POST /api/patients/{id}/samples (multipart/form-data).
//
// Поля: sample_name (string, required), file (FASTA/FASTQ, опц. .gz, required),
// sample_type (default "short-reads"), sequencing_type (default "OTHER"),
// panel_name (optional).
//
// Возвращает 202 Accepted с {sample_id, status:"processing"}.
func (h *SampleHandler) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad patient id")
		return
	}
	log := logging.FromContext(ctx).With(
		slog.String("handler", "sample.Upload"),
		slog.Int("patient_id", id),
	)

	if _, err := h.patientRepo.Get(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, http.StatusNotFound, "patient not found")
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}

	// Parse multipart.
	maxBytes := int64(h.cfg.MaxUploadMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "parse multipart: "+err.Error())
		return
	}

	sampleName := strings.TrimSpace(r.FormValue("sample_name"))
	if sampleName == "" {
		sampleName = "sample-" + strconv.Itoa(id) + "-" + time.Now().Format("20060102T150405")
	}
	sampleType := strings.TrimSpace(r.FormValue("sample_type"))
	seqType := strings.TrimSpace(r.FormValue("sequencing_type"))
	panelName := strings.TrimSpace(r.FormValue("panel_name"))
	var panelPtr *string
	if panelName != "" {
		panelPtr = &panelName
	}
	_ = panelPtr // panel пока не пишем (в репозитории CreateSample не принимает)

	// Определяем режим: single-end ("file") vs paired-end ("file_r1"+"file_r2").
	// Spec: поля взаимоисключаемы. Только R1 без R2 → 422.
	var (
		reads1, reads2     []string
		r1Path, r2Path     string
		filenameForLog     string
	)
	if _, hasR1 := r.MultipartForm.File["file_r1"]; hasR1 {
		// paired-end путь
		_, hasR2 := r.MultipartForm.File["file_r2"]
		if !hasR2 {
			writeError(ctx, w, http.StatusUnprocessableEntity, "file_r2 is required when file_r1 is provided")
			return
		}
		f1, h1, err := r.FormFile("file_r1")
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "file_r1: "+err.Error())
			return
		}
		defer f1.Close()
		f2, h2, err := r.FormFile("file_r2")
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "file_r2: "+err.Error())
			return
		}
		defer f2.Close()

		// Распарсим оба и сохраним сырые файлы для воспроизводимости.
		var err1 error
		reads1, r1Path, err1 = parseAndSaveReads(f1, h1, h.cfg.UploadsDir, id, "R1",
			h.cfg.MaxReadsCount, h.cfg.MaxReadLen)
		if err1 != nil {
			writeError(ctx, w, http.StatusBadRequest, "parse R1: "+err1.Error())
			return
		}
		var err2 error
		reads2, r2Path, err2 = parseAndSaveReads(f2, h2, h.cfg.UploadsDir, id, "R2",
			h.cfg.MaxReadsCount, h.cfg.MaxReadLen)
		if err2 != nil {
			writeError(ctx, w, http.StatusBadRequest, "parse R2: "+err2.Error())
			return
		}
		if len(reads1) != len(reads2) {
			writeError(ctx, w, http.StatusUnprocessableEntity,
				fmt.Sprintf("paired-end: R1 (%d) and R2 (%d) read counts mismatch",
					len(reads1), len(reads2)))
			return
		}
		filenameForLog = h1.Filename + "+" + h2.Filename
	} else {
		// single-end путь (legacy)
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "file is required: "+err.Error())
			return
		}
		defer file.Close()
		var err1 error
		reads1, r1Path, err1 = parseAndSaveReads(file, header, h.cfg.UploadsDir, id, "SE",
			h.cfg.MaxReadsCount, h.cfg.MaxReadLen)
		if err1 != nil {
			writeError(ctx, w, http.StatusBadRequest, "parse reads: "+err1.Error())
			return
		}
		filenameForLog = header.Filename
	}

	if len(reads1) == 0 {
		writeError(ctx, w, http.StatusBadRequest, "no reads parsed from file")
		return
	}

	// Создаём sample со статусом processing.
	sampleID, err := h.variantRepo.CreateSample(ctx, id, sampleName, sampleType, seqType, "processing", nil)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "create sample: "+err.Error())
		return
	}
	// Сохраним пути r1/r2 (если есть) для воспроизводимости/повторного запуска.
	var r2Ptr *string
	if r2Path != "" {
		r2Ptr = &r2Path
	}
	if err := h.variantRepo.SetSamplePairedPaths(ctx, sampleID, r1Path, r2Ptr); err != nil {
		// не критично, просто логируем — пайплайн всё равно прогоним по in-memory reads
		logging.FromContext(ctx).LogAttrs(ctx, slog.LevelWarn,
			"set paired paths failed", slog.String("error", err.Error()))
	}

	log = log.With(
		slog.Int("sample_id", sampleID),
		slog.String("sample_name", sampleName),
		slog.Int("reads_count", len(reads1)),
		slog.Bool("paired", len(reads2) > 0),
		slog.String("filename", filenameForLog),
	)
	log.LogAttrs(ctx, slog.LevelInfo, "sample queued")

	patientID := id
	jobName := "sample-" + strconv.Itoa(sampleID)
	r1 := reads1
	r2 := reads2
	if qerr := h.queue.Submit(jobName, func(jobCtx context.Context) error {
		return h.runPipelineJob(jobCtx, patientID, sampleID, r1, r2)
	}); qerr != nil {
		// очередь закрыта — пометим sample failed.
		reason := "queue closed: " + qerr.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		writeError(ctx, w, http.StatusServiceUnavailable, "queue closed")
		return
	}

	writeJSON(ctx, w, http.StatusAccepted, map[string]any{
		"sample_id":  sampleID,
		"patient_id": patientID,
		"status":     "processing",
	})
}

// parseAndSaveReads сохраняет загруженный файл в uploadsDir (для воспроизводимости)
// и одновременно парсит его в []reads. Возвращает (reads, savedPath, err).
func parseAndSaveReads(
	src multipart.File, hdr *multipart.FileHeader, uploadsDir string,
	patientID int, suffix string, maxReads, maxLen int,
) ([]string, string, error) {
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("mkdir uploads: %w", err)
	}
	stamp := time.Now().UnixNano()
	dst := filepath.Join(uploadsDir,
		fmt.Sprintf("p%d-s%s-%d-%s", patientID, suffix, stamp, filepath.Base(hdr.Filename)))

	out, err := os.Create(dst)
	if err != nil {
		return nil, "", fmt.Errorf("create dst: %w", err)
	}
	defer out.Close()

	// Копируем в файл, параллельно делая копию в память (через TeeReader)
	// — чтобы один раз прочесть и сохранить, и распарсить.
	if _, err := io.Copy(out, src); err != nil {
		_ = os.Remove(dst)
		return nil, "", fmt.Errorf("copy: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		// не все multipart.File поддерживают Seek; в этом случае откроем заново
		fh, err2 := os.Open(dst)
		if err2 != nil {
			return nil, "", fmt.Errorf("reopen for parse: %w", err2)
		}
		defer fh.Close()
		gz := strings.HasSuffix(strings.ToLower(hdr.Filename), ".gz")
		reads, perr := pipeline.ParseReads(fh, gz, maxReads, maxLen)
		if perr != nil {
			return nil, "", perr
		}
		return reads, dst, nil
	}
	gzipped := strings.HasSuffix(strings.ToLower(hdr.Filename), ".gz")
	reads, err := pipeline.ParseReads(src, gzipped, maxReads, maxLen)
	if err != nil {
		return nil, "", err
	}
	return reads, dst, nil
}

// runPipelineJob — фоновая задача обработки sample.
// reads2 == nil → single-end; иначе paired-end (R1+R2).
func (h *SampleHandler) runPipelineJob(ctx context.Context, patientID, sampleID int, reads1, reads2 []string) error {
	log := logging.FromContext(ctx).With(
		slog.Int("patient_id", patientID),
		slog.Int("sample_id", sampleID),
		slog.Bool("paired", len(reads2) > 0),
	)
	res, err := h.pipe.RunPaired(ctx, reads1, reads2)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "pipeline failed", slog.String("error", err.Error()))
		reason := "pipeline: " + err.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		return err
	}
	_ = h.variantRepo.SetSampleVCFPath(ctx, sampleID, res.VCFPath)

	records, err := pipeline.ParseVCF(res.VCFPath)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "parse vcf failed", slog.String("error", err.Error()))
		reason := "parse vcf: " + err.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		return err
	}
	vids, err := h.variantRepo.SavePipelineResults(ctx, patientID, sampleID, records, h.cfg.GenomeBuild)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "save results failed", slog.String("error", err.Error()))
		reason := "save results: " + err.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		return err
	}
	_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "completed", nil)
	if h.enricher != nil {
		h.enricher.ScheduleEnrich(vids)
	}
	log.LogAttrs(ctx, slog.LevelInfo, "sample completed",
		slog.Int("variants", len(records)))
	return nil
}

// Get — GET /api/samples/{id}
func (h *SampleHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad sample id")
		return
	}
	s, err := h.variantRepo.GetSample(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(ctx, w, http.StatusNotFound, "sample not found")
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, s)
}

// ListAll — GET /api/samples?limit=&offset=
// Все sample'ы, доступные текущему пользователю (его пациенты + legacy).
// Для дашборда «все загрузки» (без выбора пациента).
func (h *SampleHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	q := r.URL.Query()
	limit := atoiDefault(q.Get("limit"), 100)
	offset := atoiDefault(q.Get("offset"), 0)
	items, total, err := h.variantRepo.ListAllSamples(ctx, userID, limit, offset)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// List — GET /api/patients/{id}/samples
func (h *SampleHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad patient id")
		return
	}
	if _, err := h.patientRepo.Get(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, http.StatusNotFound, "patient not found")
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := h.variantRepo.ListSamplesByPatient(ctx, id)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

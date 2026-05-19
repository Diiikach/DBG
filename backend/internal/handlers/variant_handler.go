package handlers

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/term-paper-2026/backend/internal/auth"
	"github.com/term-paper-2026/backend/internal/config"
	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/models"
	"github.com/term-paper-2026/backend/internal/pipeline"
	"github.com/term-paper-2026/backend/internal/repository"
)

// Регулярки валидации координат варианта (модуль 5).
var (
	chromRegexp = regexp.MustCompile(`^(chr)?(\d+|[XYM]|MT)$`)
	seqRegexp   = regexp.MustCompile(`^[ACGTN-]+$`)
)

// normalizeChrom приводит "chr1" → "1", "chrX" → "X", "MT"/"M" остаются как есть.
func normalizeChrom(c string) string {
	c = strings.TrimSpace(c)
	if len(c) > 3 && strings.EqualFold(c[:3], "chr") {
		c = c[3:]
	}
	return strings.ToUpper(c)
}

// EnrichScheduler — абстракция над постановкой enrichment-задачи в фоновую очередь.
// Реализуется в main.go через замыкание (jobs.Queue + enrichment.Enricher).
type EnrichScheduler interface {
	ScheduleEnrich(variantIDs []int)
}

type VariantHandler struct {
	patientRepo *repository.PatientRepo
	variantRepo *repository.VariantRepo
	pipe        *pipeline.Pipeline
	enricher    EnrichScheduler
	cfg         config.Config
}

func NewVariantHandler(
	pr *repository.PatientRepo,
	vr *repository.VariantRepo,
	p *pipeline.Pipeline,
	enricher EnrichScheduler,
	cfg config.Config,
) *VariantHandler {
	return &VariantHandler{patientRepo: pr, variantRepo: vr, pipe: p, enricher: enricher, cfg: cfg}
}

// Align — DEPRECATED. Синхронная альтернатива multipart-загрузке samples.
// Принимает JSON { sample_name, reads }, прогоняет пайплайн и сохраняет варианты.
// POST /api/patients/{id}/align
func (h *VariantHandler) Align(w http.ResponseWriter, r *http.Request) {
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
		slog.String("handler", "variant.Align"),
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

	var req models.AlignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.SampleName == "" {
		req.SampleName = "sample-" + strconv.Itoa(id)
	}
	if len(req.Reads) == 0 {
		writeError(ctx, w, http.StatusBadRequest, "reads must be non-empty")
		return
	}

	log = log.With(
		slog.String("sample_name", req.SampleName),
		slog.Int("reads_count", len(req.Reads)),
	)
	log.LogAttrs(ctx, slog.LevelInfo, "alignment requested (deprecated sync API)")

	startPipe := time.Now()
	res, err := h.pipe.Run(logging.WithLogger(ctx, log), req.Reads)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "pipeline failed",
			slog.String("error", err.Error()),
			slog.Duration("elapsed", time.Since(startPipe)))
		writeError(ctx, w, http.StatusUnprocessableEntity, "pipeline failed: "+err.Error())
		return
	}

	records, err := pipeline.ParseVCF(res.VCFPath)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "parse vcf: "+err.Error())
		return
	}

	sampleID, err := h.variantRepo.CreateSample(ctx, id, req.SampleName, "", "", "processing", &res.VCFPath)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "create sample: "+err.Error())
		return
	}

	vids, err := h.variantRepo.SavePipelineResults(ctx, id, sampleID, records, h.cfg.GenomeBuild)
	if err != nil {
		reason := "save results: " + err.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		writeError(ctx, w, http.StatusInternalServerError, "save results: "+err.Error())
		return
	}
	_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "completed", nil)
	if h.enricher != nil {
		h.enricher.ScheduleEnrich(vids)
	}

	writeJSON(ctx, w, http.StatusOK, models.AlignResponse{
		SampleID:  sampleID,
		PatientID: id,
		Variants:  records,
		VCFPath:   res.VCFPath,
		Status:    "completed",
	})
}

// ListVariants — GET /api/patients/{id}/variants
//
// Параметры: limit, offset, q (зарезервировано), chrom, variant_type, filter,
// min_qual, zygosity, sort.
func (h *VariantHandler) ListVariants(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	limit := atoiDefault(q.Get("limit"), 100)
	offset := atoiDefault(q.Get("offset"), 0)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	f := models.VariantFilter{
		Chrom:       strings.TrimSpace(q.Get("chrom")),
		VariantType: strings.ToUpper(strings.TrimSpace(q.Get("variant_type"))),
		FilterEq:    strings.TrimSpace(q.Get("filter")),
		Zygosity:    strings.ToUpper(strings.TrimSpace(q.Get("zygosity"))),
		Q:           strings.TrimSpace(q.Get("q")),
		Sort:        strings.TrimSpace(q.Get("sort")),
		Limit:       limit,
		Offset:      offset,
	}
	if s := q.Get("min_qual"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			f.MinQual = &v
		}
	}

	items, total, err := h.variantRepo.ListPatientVariantsFiltered(ctx, id, f)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, map[string]any{
		"patient_id": id,
		"items":      items,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetVariantDetails — GET /api/variants/{id}
func (h *VariantHandler) GetVariantDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad variant id")
		return
	}
	d, err := h.variantRepo.GetVariantDetails(ctx, id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(ctx, w, http.StatusNotFound, "variant not found")
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, d)
}

// CohortByVariant — GET /api/variants/{id}/patients
// Возвращает пациентов с этим же variant_id (когорта).
func (h *VariantHandler) CohortByVariant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad variant id")
		return
	}
	q := r.URL.Query()
	limit := atoiDefault(q.Get("limit"), 100)
	offset := atoiDefault(q.Get("offset"), 0)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items, total, err := h.patientRepo.ListByVariant(ctx, id, userID, limit, offset)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, models.PageResponse{
		Items: items, Total: total, Limit: limit, Offset: offset,
	})
}

// SearchVariants — GET /api/variants?rs_id=&gene=&chrom=&pos=&ref=&alt=&build=&q=
// Поиск варианта по rs_id/гену/координатам/глобальному q.
func (h *VariantHandler) SearchVariants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	q := r.URL.Query()
	f := models.VariantSearchFilter{
		RsID:   strings.TrimSpace(q.Get("rs_id")),
		Gene:   strings.TrimSpace(q.Get("gene")),
		Chrom:  strings.TrimSpace(q.Get("chrom")),
		Ref:    strings.TrimSpace(q.Get("ref")),
		Alt:    strings.TrimSpace(q.Get("alt")),
		Build:  strings.TrimSpace(q.Get("build")),
		Q:      strings.TrimSpace(q.Get("q")),
		Limit:  atoiDefault(q.Get("limit"), 50),
		Offset: atoiDefault(q.Get("offset"), 0),
	}
	if s := q.Get("pos"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			f.Pos = &v
		}
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	log := logging.FromContext(ctx).With(slog.String("handler", "variant.Search"))
	log.LogAttrs(ctx, slog.LevelDebug, "search variants",
		slog.String("rs_id", f.RsID), slog.String("gene", f.Gene),
		slog.String("chrom", f.Chrom), slog.String("q", f.Q),
	)

	items, total, err := h.variantRepo.SearchVariants(ctx, f, userID)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, models.PageResponse{
		Items: items, Total: total, Limit: f.Limit, Offset: f.Offset,
	})
}

// manualVariantReq — payload для ручного добавления варианта (модуль 5).
type manualVariantReq struct {
	Chromosome   string   `json:"chromosome"`
	Position     int64    `json:"position"`
	Reference    string   `json:"reference"`
	Alternate    string   `json:"alternate"`
	RsID         *string  `json:"rs_id,omitempty"`
	GenomeBuild  *string  `json:"genome_build,omitempty"`
	VariantType  *string  `json:"variant_type,omitempty"`
	Zygosity     *string  `json:"zygosity,omitempty"`
	Quality      *float64 `json:"quality,omitempty"`
	FilterStatus *string  `json:"filter_status,omitempty"`
}

// AddManualVariant — POST /api/patients/{id}/variants
// Ручное добавление одиночного варианта пациенту (модуль 5).
//
//	201 → { patient_variant_id, variant_id }
//	422 → невалидные координаты или ref/alt
//	409 → у пациента уже есть запись с этим variant_id
func (h *VariantHandler) AddManualVariant(w http.ResponseWriter, r *http.Request) {
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

	var req manualVariantReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	// Валидация координат и аллелей (модуль 5 ТЗ).
	chrom := strings.TrimSpace(req.Chromosome)
	if !chromRegexp.MatchString(chrom) {
		writeError(ctx, w, http.StatusUnprocessableEntity, "invalid chromosome")
		return
	}
	if req.Position <= 0 {
		writeError(ctx, w, http.StatusUnprocessableEntity, "position must be > 0")
		return
	}
	ref := strings.ToUpper(strings.TrimSpace(req.Reference))
	alt := strings.ToUpper(strings.TrimSpace(req.Alternate))
	if !seqRegexp.MatchString(ref) {
		writeError(ctx, w, http.StatusUnprocessableEntity, "invalid reference allele")
		return
	}
	if !seqRegexp.MatchString(alt) {
		writeError(ctx, w, http.StatusUnprocessableEntity, "invalid alternate allele")
		return
	}

	rec := models.VCFRecord{
		Chromosome:   normalizeChrom(chrom),
		Position:     req.Position,
		Reference:    ref,
		Alternate:    alt,
		RsID:         req.RsID,
		GenomeBuild:  req.GenomeBuild,
		VariantType:  req.VariantType,
		Quality:      req.Quality,
		Zygosity:     req.Zygosity,
	}
	if req.FilterStatus != nil {
		rec.Filter = *req.FilterStatus
	} else {
		rec.Filter = "PASS"
	}

	log := logging.FromContext(ctx).With(
		slog.String("handler", "variant.AddManualVariant"),
		slog.Int("patient_id", id),
		slog.String("chrom", rec.Chromosome),
		slog.Int64("pos", rec.Position),
	)

	vid, pvID, err := h.variantRepo.AddManualVariant(ctx, id, rec, h.cfg.GenomeBuild)
	if err != nil {
		if errors.Is(err, repository.ErrPatientVariantExists) {
			writeError(ctx, w, http.StatusConflict, "patient already has this variant")
			return
		}
		log.LogAttrs(ctx, slog.LevelError, "manual variant insert failed",
			slog.String("error", err.Error()))
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	log.LogAttrs(ctx, slog.LevelInfo, "manual variant added",
		slog.Int("variant_id", vid), slog.Int64("patient_variant_id", pvID))
	if h.enricher != nil {
		h.enricher.ScheduleEnrich([]int{vid})
	}

	writeJSON(ctx, w, http.StatusCreated, map[string]any{
		"patient_variant_id": pvID,
		"variant_id":         vid,
	})
}

// ImportVCF — POST /api/patients/{id}/variants/vcf (multipart/form-data).
// Прямой импорт VCF файлом без прогонки пайплайна (модуль 3).
//
// Поля multipart:
//   - file:         required, *.vcf или *.vcf.gz
//   - sample_name:  optional, default = "imported_<timestamp>"
//   - genome_build: optional, default = cfg.GenomeBuild (обычно "GRCh38")
//
// 200 → { sample_id, imported, skipped, warnings }
// 422 → файл пустой/битый/не VCF
// 404 → чужой пациент
func (h *VariantHandler) ImportVCF(w http.ResponseWriter, r *http.Request) {
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
		slog.String("handler", "variant.ImportVCF"),
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

	maxBytes := int64(h.cfg.MaxUploadMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "parse multipart: "+err.Error())
		return
	}

	sampleName := strings.TrimSpace(r.FormValue("sample_name"))
	if sampleName == "" {
		sampleName = "imported_" + time.Now().Format("20060102T150405")
	}
	build := strings.TrimSpace(r.FormValue("genome_build"))
	if build == "" {
		build = h.cfg.GenomeBuild
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "file is required: "+err.Error())
		return
	}
	defer file.Close()

	// Сохраняем upload во временный файл (если .gz — распаковываем).
	gzipped := strings.HasSuffix(strings.ToLower(header.Filename), ".gz") ||
		r.Header.Get("Content-Type") == "application/gzip"
	tmpPath, err := saveVCFUpload(file, gzipped, h.cfg.UploadsDir, id)
	if err != nil {
		writeError(ctx, w, http.StatusUnprocessableEntity, "save upload: "+err.Error())
		return
	}
	// Не удаляем tmpPath — он пригодится как vcf_file_path для sample.

	records, err := pipeline.ParseVCF(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		writeError(ctx, w, http.StatusUnprocessableEntity, "parse vcf: "+err.Error())
		return
	}
	if len(records) == 0 {
		_ = os.Remove(tmpPath)
		writeError(ctx, w, http.StatusUnprocessableEntity, "no variants parsed from vcf")
		return
	}

	// Создаём sample со статусом completed сразу (пайплайн не запускался).
	sampleID, err := h.variantRepo.CreateSample(ctx, id, sampleName, "vcf-import", "OTHER", "completed", &tmpPath)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "create sample: "+err.Error())
		return
	}

	vids, err := h.variantRepo.SavePipelineResults(ctx, id, sampleID, records, build)
	if err != nil {
		reason := "save results: " + err.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		writeError(ctx, w, http.StatusInternalServerError, "save results: "+err.Error())
		return
	}
	if h.enricher != nil {
		h.enricher.ScheduleEnrich(vids)
	}

	log.LogAttrs(ctx, slog.LevelInfo, "vcf imported",
		slog.Int("sample_id", sampleID),
		slog.String("sample_name", sampleName),
		slog.String("filename", header.Filename),
		slog.Int("imported", len(records)),
	)

	writeJSON(ctx, w, http.StatusOK, map[string]any{
		"sample_id": sampleID,
		"imported":  len(records),
		"skipped":   0,
		"warnings":  []string{},
	})
}

// ImportCSV — POST /api/patients/{id}/variants/csv (multipart/form-data).
// Импорт варианта таблицей (модуль 4).
//
// Поля multipart:
//   - file:      required, *.csv
//   - delimiter: optional, default ","
//   - sample_name / genome_build — как у ImportVCF.
//
// 200 → { sample_id, imported, skipped, warnings }
// 422 → отсутствуют обязательные колонки / нет валидных строк
// 404 → чужой пациент
func (h *VariantHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
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
		slog.String("handler", "variant.ImportCSV"),
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

	maxBytes := int64(h.cfg.MaxUploadMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "parse multipart: "+err.Error())
		return
	}

	sampleName := strings.TrimSpace(r.FormValue("sample_name"))
	if sampleName == "" {
		sampleName = "imported_csv_" + time.Now().Format("20060102T150405")
	}
	build := strings.TrimSpace(r.FormValue("genome_build"))
	if build == "" {
		build = h.cfg.GenomeBuild
	}
	delim := rune(',')
	if d := strings.TrimSpace(r.FormValue("delimiter")); d != "" {
		dr := []rune(d)
		if len(dr) > 0 {
			delim = dr[0]
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "file is required: "+err.Error())
		return
	}
	defer file.Close()

	records, warnings, err := pipeline.ParseCSV(file, delim)
	if err != nil {
		// Отсутствуют обязательные колонки → 422 со списком в сообщении.
		writeError(ctx, w, http.StatusUnprocessableEntity, "parse csv: "+err.Error())
		return
	}
	if len(records) == 0 {
		writeError(ctx, w, http.StatusUnprocessableEntity, "no valid variants in csv")
		return
	}

	sampleID, err := h.variantRepo.CreateSample(ctx, id, sampleName, "csv-import", "OTHER", "completed", nil)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "create sample: "+err.Error())
		return
	}

	vids, err := h.variantRepo.SavePipelineResults(ctx, id, sampleID, records, build)
	if err != nil {
		reason := "save results: " + err.Error()
		_ = h.variantRepo.UpdateSampleStatus(ctx, sampleID, "failed", &reason)
		writeError(ctx, w, http.StatusInternalServerError, "save results: "+err.Error())
		return
	}
	if h.enricher != nil {
		h.enricher.ScheduleEnrich(vids)
	}

	log.LogAttrs(ctx, slog.LevelInfo, "csv imported",
		slog.Int("sample_id", sampleID),
		slog.String("filename", header.Filename),
		slog.Int("imported", len(records)),
		slog.Int("warnings", len(warnings)),
	)

	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(ctx, w, http.StatusOK, map[string]any{
		"sample_id": sampleID,
		"imported":  len(records),
		"skipped":   len(warnings),
		"warnings":  warnings,
	})
}

// saveVCFUpload сохраняет multipart-файл в uploadsDir; если gzipped — на лету распаковывает.
// Возвращает путь к итоговому .vcf на диске.
func saveVCFUpload(src io.Reader, gzipped bool, uploadsDir string, patientID int) (string, error) {
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir uploads: %w", err)
	}
	name := fmt.Sprintf("vcf-p%d-%d.vcf", patientID, time.Now().UnixNano())
	path := filepath.Join(uploadsDir, name)

	out, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	var reader io.Reader = src
	if gzipped {
		gz, err := gzip.NewReader(src)
		if err != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	if _, err := io.Copy(out, reader); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("copy: %w", err)
	}
	return path, nil
}

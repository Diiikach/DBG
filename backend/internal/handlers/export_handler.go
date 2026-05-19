package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/models"
	"github.com/term-paper-2026/backend/internal/repository"
	"github.com/term-paper-2026/backend/internal/auth"
)

// ExportHandler — обработчики выгрузки данных в CSV/JSON.
//
// Регистрируется на маршрутах:
//   - GET /api/patients/export?format=csv|json&q=...
//   - GET /api/patients/{id}/variants/export?format=csv|json + те же фильтры,
//     что и в ListVariants.
type ExportHandler struct {
	patientRepo *repository.PatientRepo
	variantRepo *repository.VariantRepo
}

func NewExportHandler(pr *repository.PatientRepo, vr *repository.VariantRepo) *ExportHandler {
	return &ExportHandler{patientRepo: pr, variantRepo: vr}
}

// exportLimit — верхняя граница на выгрузку, чтобы не отдавать многомиллионные
// CSV. По ТЗ 4.1.4 ответ должен укладываться в 2 с — 50 000 строк это
// разумный предел.
const exportLimit = 50_000

// pickFormat возвращает "csv" или "json" с дефолтом "csv".
func pickFormat(r *http.Request) string {
	f := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if f == "json" {
		return "json"
	}
	return "csv"
}

// setDownloadHeaders выставляет Content-Type и Content-Disposition,
// чтобы браузер скачал файл, а не отрисовал его.
func setDownloadHeaders(w http.ResponseWriter, format, baseName string) {
	stamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.%s", baseName, stamp, format)
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
}

// writeCSV пишет в ответ CSV: заголовок + строки. После записи возвращает
// ошибку — её можно только залогировать, т.к. ответ уже стримится.
func writeCSV(w http.ResponseWriter, header []string, rows [][]string) error {
	// BOM — чтобы Excel корректно открыл UTF-8 CSV.
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// ExportPatients — GET /api/patients/export?format=csv|json&q=...
func (h *ExportHandler) ExportPatients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	log := logging.FromContext(ctx).With(slog.String("handler", "export.Patients"))

	format := pickFormat(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	items, total, err := h.patientRepo.List(ctx, userID, q, exportLimit, 0)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	log.LogAttrs(ctx, slog.LevelInfo, "patients export",
		slog.String("format", format),
		slog.Int("count", len(items)),
		slog.Int("total", total),
	)

	setDownloadHeaders(w, format, "patients")

	if format == "json" {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"items":     items,
			"total":     total,
			"exported":  len(items),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			log.LogAttrs(ctx, slog.LevelError, "encode json", slog.String("error", err.Error()))
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	header := []string{
		"patient_id", "external_id", "first_name", "last_name",
		"date_of_birth", "sex", "email", "phone_number",
		"address", "phenotype_description", "created_at", "updated_at",
	}
	rows := make([][]string, 0, len(items))
	for _, p := range items {
		rows = append(rows, []string{
			strconv.Itoa(p.PatientID),
			derefStr(p.ExternalID),
			p.FirstName,
			p.LastName,
			derefStr(p.DateOfBirth),
			derefStr(p.Sex),
			derefStr(p.Email),
			derefStr(p.PhoneNumber),
			derefStr(p.Address),
			derefStr(p.PhenotypeDescription),
			timePtrISO(p.CreatedAt),
			timePtrISO(p.UpdatedAt),
		})
	}
	if err := writeCSV(w, header, rows); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "write csv", slog.String("error", err.Error()))
	}
}

// ExportPatientVariants — GET /api/patients/{id}/variants/export?format=csv|json
// Поддерживает те же фильтры, что и [VariantHandler.ListVariants].
func (h *ExportHandler) ExportPatientVariants(w http.ResponseWriter, r *http.Request) {
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

	log := logging.FromContext(ctx).With(
		slog.String("handler", "export.PatientVariants"),
		slog.Int("patient_id", id),
	)

	format := pickFormat(r)
	q := r.URL.Query()
	f := models.VariantFilter{
		Chrom:       strings.TrimSpace(q.Get("chrom")),
		VariantType: strings.ToUpper(strings.TrimSpace(q.Get("variant_type"))),
		FilterEq:    strings.TrimSpace(q.Get("filter")),
		Zygosity:    strings.ToUpper(strings.TrimSpace(q.Get("zygosity"))),
		Sort:        strings.TrimSpace(q.Get("sort")),
		Limit:       exportLimit,
		Offset:      0,
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

	log.LogAttrs(ctx, slog.LevelInfo, "patient variants export",
		slog.String("format", format),
		slog.Int("count", len(items)),
		slog.Int("total", total),
	)

	baseName := fmt.Sprintf("patient-%d-variants", id)
	setDownloadHeaders(w, format, baseName)

	if format == "json" {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"patient_id": id,
			"items":      items,
			"total":      total,
			"exported":   len(items),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			log.LogAttrs(ctx, slog.LevelError, "encode json", slog.String("error", err.Error()))
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	header := []string{
		"patient_variant_id", "patient_id", "sample_id",
		"variant_id", "chromosome", "position", "reference", "alternate",
		"rs_id", "genome_build", "variant_type",
		"zygosity", "quality", "read_depth",
		"allele_depth_ref", "allele_depth_alt", "genotype_quality",
		"filter_status", "detected_at",
		"gene_symbols", "top_consequence", "top_impact",
		// flattened first annotation
		"hgvsc", "hgvsp", "gnomad_af", "gnomad_af_popmax",
		"sift_score", "sift_prediction", "polyphen_score", "polyphen_prediction",
		"cadd_score", "revel_score",
		"clinvar_significance", "clinvar_id",
	}
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		v := it.Variant
		var ann *models.VariantAnnotation
		if len(it.Annotations) > 0 {
			ann = &it.Annotations[0]
		}
		rows = append(rows, []string{
			strconv.FormatInt(it.PatientVariantID, 10),
			strconv.Itoa(it.PatientID),
			intPtrStr(it.SampleID),
			strconv.Itoa(v.VariantID),
			v.Chromosome,
			strconv.FormatInt(v.Position, 10),
			v.Reference,
			v.Alternate,
			derefStr(v.RsID),
			derefStr(v.GenomeBuild),
			derefStr(v.VariantType),
			derefStr(it.Zygosity),
			floatPtrStr(it.Quality),
			intPtrStr(it.ReadDepth),
			intPtrStr(it.AlleleDepthRef),
			intPtrStr(it.AlleleDepthAlt),
			intPtrStr(it.GenotypeQuality),
			derefStr(it.FilterStatus),
			timePtrISO(it.DetectedAt),
			strings.Join(it.GeneSymbols, "|"),
			derefStr(it.TopConsequence),
			derefStr(it.TopImpact),
			annStr(ann, func(a *models.VariantAnnotation) string { return derefStr(a.HGVSc) }),
			annStr(ann, func(a *models.VariantAnnotation) string { return derefStr(a.HGVSp) }),
			annFloat(ann, func(a *models.VariantAnnotation) *float64 { return a.GnomadAF }),
			annFloat(ann, func(a *models.VariantAnnotation) *float64 { return a.GnomadAFPopmax }),
			annFloat(ann, func(a *models.VariantAnnotation) *float64 { return a.SIFTScore }),
			annStr(ann, func(a *models.VariantAnnotation) string { return derefStr(a.SIFTPrediction) }),
			annFloat(ann, func(a *models.VariantAnnotation) *float64 { return a.PolyphenScore }),
			annStr(ann, func(a *models.VariantAnnotation) string { return derefStr(a.PolyphenPrediction) }),
			annFloat(ann, func(a *models.VariantAnnotation) *float64 { return a.CADDScore }),
			annFloat(ann, func(a *models.VariantAnnotation) *float64 { return a.RevelScore }),
			annStr(ann, func(a *models.VariantAnnotation) string { return derefStr(a.ClinvarSignificance) }),
			annStr(ann, func(a *models.VariantAnnotation) string { return derefStr(a.ClinvarID) }),
		})
	}
	if err := writeCSV(w, header, rows); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "write csv", slog.String("error", err.Error()))
	}
}

// ---- helpers ----

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func intPtrStr(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func floatPtrStr(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func timePtrISO(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.UTC().Format(time.RFC3339)
}

func annStr(a *models.VariantAnnotation, f func(*models.VariantAnnotation) string) string {
	if a == nil {
		return ""
	}
	return f(a)
}

func annFloat(a *models.VariantAnnotation, f func(*models.VariantAnnotation) *float64) string {
	if a == nil {
		return ""
	}
	return floatPtrStr(f(a))
}

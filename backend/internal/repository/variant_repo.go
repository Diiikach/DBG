package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/models"
)

type VariantRepo struct {
	pool *pgxpool.Pool
}

func NewVariantRepo(p *pgxpool.Pool) *VariantRepo { return &VariantRepo{pool: p} }

// CreateSample создаёт sample для пациента и возвращает его ID.
// status — начальный processing_status (например "uploaded" или "processing").
func (r *VariantRepo) CreateSample(ctx context.Context, patientID int, name, sampleType, seqType, status string, vcfPath *string) (int, error) {
	if sampleType == "" {
		sampleType = "short-reads"
	}
	if seqType == "" {
		seqType = "OTHER"
	}
	if status == "" {
		status = "processing"
	}
	const q = `
		INSERT INTO sample (patient_id, sample_name, sample_type, sequencing_type,
		                    vcf_file_path, processing_status)
		VALUES ($1, $2, $3, $4::seq_type, $5, $6::status)
		RETURNING sample_id
	`
	args := []any{patientID, name, sampleType, seqType, vcfPath, status}
	start := time.Now()
	var id int
	err := r.pool.QueryRow(ctx, q, args...).Scan(&id)
	logQuery(ctx, "variant.CreateSample", q, args, err, start)
	if err != nil {
		return 0, fmt.Errorf("insert sample: %w", err)
	}
	return id, nil
}

// UpdateSampleStatus обновляет статус обработки sample и причину провала.
// reason == nil очищает failure_reason (например, при переходе в completed).
func (r *VariantRepo) UpdateSampleStatus(ctx context.Context, sampleID int, status string, reason *string) error {
	const q = `UPDATE sample SET processing_status = $1::status, failure_reason = $2 WHERE sample_id = $3`
	args := []any{status, reason, sampleID}
	start := time.Now()
	_, err := r.pool.Exec(ctx, q, args...)
	logQuery(ctx, "variant.UpdateSampleStatus", q, args, err, start)
	return err
}

// SetSampleVCFPath сохраняет путь к VCF после успешного выполнения пайплайна.
func (r *VariantRepo) SetSampleVCFPath(ctx context.Context, sampleID int, path string) error {
	const q = `UPDATE sample SET vcf_file_path = $1 WHERE sample_id = $2`
	args := []any{path, sampleID}
	start := time.Now()
	_, err := r.pool.Exec(ctx, q, args...)
	logQuery(ctx, "variant.SetSampleVCFPath", q, args, err, start)
	return err
}

// SetSamplePairedPaths сохраняет r1_path/r2_path для paired-end загрузки (модуль 8).
// r2 может быть nil — тогда single-end (только R1).
func (r *VariantRepo) SetSamplePairedPaths(ctx context.Context, sampleID int, r1 string, r2 *string) error {
	const q = `UPDATE sample SET r1_path = $1, r2_path = $2 WHERE sample_id = $3`
	args := []any{r1, r2, sampleID}
	start := time.Now()
	_, err := r.pool.Exec(ctx, q, args...)
	logQuery(ctx, "variant.SetSamplePairedPaths", q, args, err, start)
	return err
}

const sampleCols = `sample_id, patient_id, sample_name, sample_type, sequencing_type::text,
		panel_name, sequencing_platform,
		to_char(sequencing_date,'YYYY-MM-DD'),
		mean_coverage, vcf_file_path,
		processing_status::text, failure_reason, created_at`

func scanSample(row pgx.Row, s *models.Sample) error {
	return row.Scan(
		&s.SampleID, &s.PatientID, &s.SampleName, &s.SampleType, &s.SequencingType,
		&s.PanelName, &s.SequencingPlatform, &s.SequencingDate,
		&s.MeanCoverage, &s.VCFFilePath, &s.ProcessingStatus, &s.FailureReason, &s.CreatedAt,
	)
}

// GetSample возвращает sample по id.
func (r *VariantRepo) GetSample(ctx context.Context, id int) (models.Sample, error) {
	const q = `SELECT ` + sampleCols + ` FROM sample WHERE sample_id = $1`
	args := []any{id}
	start := time.Now()
	var s models.Sample
	err := scanSample(r.pool.QueryRow(ctx, q, args...), &s)
	if errors.Is(err, pgx.ErrNoRows) {
		logQuery(ctx, "variant.GetSample", q, args, nil, start)
		return models.Sample{}, ErrNotFound
	}
	logQuery(ctx, "variant.GetSample", q, args, err, start)
	return s, err
}

// ListAllSamples — все sample, доступные пользователю (его пациенты + legacy
// записи с NULL в created_by_user_id). Используется для дашборда «все загрузки».
// Возвращает страницу и total. Дополнительно возвращает ФИО/external_id пациента,
// чтобы фронт мог показать, к кому относится sample, без N+1 запросов.
func (r *VariantRepo) ListAllSamples(ctx context.Context, userID, limit, offset int) ([]models.SampleWithPatient, int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	const countQ = `
		SELECT count(*) FROM sample s
		JOIN patient p ON p.patient_id = s.patient_id
		WHERE (p.created_by_user_id = $1 OR p.created_by_user_id IS NULL)`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count samples: %w", err)
	}
	// sampleCols используются без префикса s. в простых SELECT'ах;
	// здесь же JOIN с patient и колонка patient_id есть в обеих таблицах —
	// поэтому выбираем колонки sample явно с префиксом s. чтобы не было
	// "column reference patient_id is ambiguous".
	const q = `
		SELECT s.sample_id, s.patient_id, s.sample_name, s.sample_type,
		       s.sequencing_type::text, s.panel_name, s.sequencing_platform,
		       to_char(s.sequencing_date,'YYYY-MM-DD'), s.mean_coverage,
		       s.vcf_file_path, s.processing_status::text, s.failure_reason,
		       s.created_at,
		       p.external_id, p.first_name, p.last_name
		FROM sample s
		JOIN patient p ON p.patient_id = s.patient_id
		WHERE (p.created_by_user_id = $1 OR p.created_by_user_id IS NULL)
		ORDER BY s.sample_id DESC
		LIMIT $2 OFFSET $3`
	args := []any{userID, limit, offset}
	start := time.Now()
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		logQuery(ctx, "variant.ListAllSamples", q, args, err, start)
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.SampleWithPatient{}
	for rows.Next() {
		var s models.SampleWithPatient
		if err := rows.Scan(
			&s.SampleID, &s.PatientID, &s.SampleName, &s.SampleType, &s.SequencingType,
			&s.PanelName, &s.SequencingPlatform, &s.SequencingDate,
			&s.MeanCoverage, &s.VCFFilePath, &s.ProcessingStatus, &s.FailureReason, &s.CreatedAt,
			&s.PatientExternalID, &s.PatientFirstName, &s.PatientLastName,
		); err != nil {
			logQuery(ctx, "variant.ListAllSamples", q, args, err, start)
			return nil, 0, err
		}
		out = append(out, s)
	}
	logQuery(ctx, "variant.ListAllSamples", q, args, rows.Err(), start)
	return out, total, rows.Err()
}

// ListSamplesByPatient — все sample пациента.
func (r *VariantRepo) ListSamplesByPatient(ctx context.Context, patientID int) ([]models.Sample, error) {
	const q = `SELECT ` + sampleCols + `
		FROM sample WHERE patient_id = $1
		ORDER BY sample_id DESC`
	args := []any{patientID}
	start := time.Now()
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		logQuery(ctx, "variant.ListSamplesByPatient", q, args, err, start)
		return nil, err
	}
	defer rows.Close()
	out := []models.Sample{}
	for rows.Next() {
		var s models.Sample
		if err := scanSample(rows, &s); err != nil {
			logQuery(ctx, "variant.ListSamplesByPatient", q, args, err, start)
			return nil, err
		}
		out = append(out, s)
	}
	logQuery(ctx, "variant.ListSamplesByPatient", q, args, rows.Err(), start)
	return out, rows.Err()
}

// UpsertVariant вставляет вариант (если уже есть — возвращает существующий id).
func (r *VariantRepo) UpsertVariant(ctx context.Context, tx pgx.Tx, v models.VCFRecord, build string) (int, string, error) {
	variantType := classifyVariant(v.Reference, v.Alternate)
	if v.VariantType != nil && *v.VariantType != "" {
		variantType = *v.VariantType
	}
	// Если в VCFRecord есть свой build — используем его, иначе берем дефолт.
	useBuild := build
	if v.GenomeBuild != nil && *v.GenomeBuild != "" {
		useBuild = *v.GenomeBuild
	}
	// COALESCE для rs_id: новое значение перезапишет NULL, но не наоборот.
	const q = `
		INSERT INTO variant (chromosome, position, reference, alternate, genome_build, variant_type, rs_id)
		VALUES ($1, $2, $3, $4, $5::gen_build, $6::variant_type, $7)
		ON CONFLICT (chromosome, position, reference, alternate, genome_build)
		DO UPDATE SET rs_id = COALESCE(variant.rs_id, EXCLUDED.rs_id)
		RETURNING variant_id
	`
	args := []any{v.Chromosome, v.Position, v.Reference, v.Alternate, useBuild, variantType, v.RsID}
	start := time.Now()
	var id int
	err := tx.QueryRow(ctx, q, args...).Scan(&id)
	logQuery(ctx, "variant.UpsertVariant", q, args, err, start)
	if err != nil {
		return 0, "", fmt.Errorf("upsert variant: %w", err)
	}
	return id, variantType, nil
}

// InsertPatientVariant связывает пациента с вариантом.
func (r *VariantRepo) InsertPatientVariant(ctx context.Context, tx pgx.Tx,
	patientID int, sampleID int, variantID int, v models.VCFRecord) error {

	const q = `
		INSERT INTO patient_variant
			(patient_id, variant_id, sample_id, zygosity, quality, read_depth,
			 allele_depth_ref, allele_depth_alt, genotype_quality, filter_status)
		VALUES ($1, $2, $3, $4::zygosity_type, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (patient_id, variant_id) DO UPDATE SET
			sample_id        = EXCLUDED.sample_id,
			zygosity         = EXCLUDED.zygosity,
			quality          = EXCLUDED.quality,
			read_depth       = EXCLUDED.read_depth,
			allele_depth_ref = EXCLUDED.allele_depth_ref,
			allele_depth_alt = EXCLUDED.allele_depth_alt,
			genotype_quality = EXCLUDED.genotype_quality,
			filter_status    = EXCLUDED.filter_status,
			detected_at      = CURRENT_TIMESTAMP
	`
	args := []any{
		patientID, variantID, sampleID, v.Zygosity, v.Quality, v.ReadDepth,
		v.AlleleDepthRef, v.AlleleDepthAlt, v.GenotypeQuality, v.Filter,
	}
	start := time.Now()
	_, err := tx.Exec(ctx, q, args...)
	logQuery(ctx, "variant.InsertPatientVariant", q, args, err, start)
	return err
}

// SavePipelineResults — атомарно сохраняет все варианты пациента.
// Возвращает список variant_id, прошедших через UpsertVariant (для постановки
// фоновой enrichment-задачи на каждый из них).
func (r *VariantRepo) SavePipelineResults(ctx context.Context, patientID, sampleID int,
	records []models.VCFRecord, build string) ([]int, error) {

	log := logging.FromContext(ctx).With(
		slog.String("repo_op", "variant.SavePipelineResults"),
		slog.Int("patient_id", patientID),
		slog.Int("sample_id", sampleID),
		slog.Int("records", len(records)),
	)
	start := time.Now()
	log.LogAttrs(ctx, slog.LevelInfo, "tx begin")

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "tx begin failed", slog.String("error", err.Error()))
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	variantIDs := make([]int, 0, len(records))
	seen := make(map[int]struct{}, len(records))
	for i, rec := range records {
		vid, _, err := r.UpsertVariant(ctx, tx, rec, build)
		if err != nil {
			log.LogAttrs(ctx, slog.LevelError, "upsert variant failed",
				slog.Int("record_idx", i), slog.String("error", err.Error()))
			return nil, err
		}
		if _, dup := seen[vid]; !dup {
			seen[vid] = struct{}{}
			variantIDs = append(variantIDs, vid)
		}
		if err := r.InsertPatientVariant(ctx, tx, patientID, sampleID, vid, rec); err != nil {
			log.LogAttrs(ctx, slog.LevelError, "insert patient_variant failed",
				slog.Int("record_idx", i), slog.String("error", err.Error()))
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "tx commit failed",
			slog.String("error", err.Error()), slog.Duration("duration", time.Since(start)))
		return nil, err
	}
	log.LogAttrs(ctx, slog.LevelInfo, "tx committed", slog.Duration("duration", time.Since(start)))
	return variantIDs, nil
}

// allowedSortColumns — белый список колонок для сортировки.
var allowedSortColumns = map[string]string{
	"chrom":         "v.chromosome",
	"position":      "v.position",
	"quality":       "pv.quality",
	"depth":         "pv.read_depth",
	"read_depth":    "pv.read_depth",
	"variant_type":  "v.variant_type",
	"filter_status": "pv.filter_status",
}

// buildOrderBy парсит ?sort=col1,-col2 и возвращает безопасный ORDER BY.
func buildOrderBy(sort string) string {
	if sort == "" {
		return " ORDER BY v.chromosome, v.position"
	}
	parts := strings.Split(sort, ",")
	pieces := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		dir := "ASC"
		if strings.HasPrefix(p, "-") {
			dir = "DESC"
			p = p[1:]
		} else if strings.HasPrefix(p, "+") {
			p = p[1:]
		}
		col, ok := allowedSortColumns[p]
		if !ok {
			continue
		}
		pieces = append(pieces, col+" "+dir)
	}
	if len(pieces) == 0 {
		return " ORDER BY v.chromosome, v.position"
	}
	return " ORDER BY " + strings.Join(pieces, ", ")
}

// ListPatientVariantsFiltered — варианты пациента c фильтрами/сортировкой и total.
func (r *VariantRepo) ListPatientVariantsFiltered(ctx context.Context,
	patientID int, f models.VariantFilter) ([]models.PatientVariantRich, int, error) {

	conds := []string{"pv.patient_id = $1"}
	args := []any{patientID}
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if f.Chrom != "" {
		add("v.chromosome = ?", f.Chrom)
	}
	if f.VariantType != "" {
		add("v.variant_type = ?::variant_type", f.VariantType)
	}
	if f.FilterEq != "" {
		add("pv.filter_status = ?", f.FilterEq)
	}
	if f.MinQual != nil {
		add("pv.quality >= ?", *f.MinQual)
	}
	if f.Zygosity != "" {
		add("pv.zygosity = ?::zygosity_type", f.Zygosity)
	}
	// Полнотекстовый поиск по странице вариантов пациента.
	// Поддерживаем:
	//   • rsID         — точное совпадение по v.rs_id (case-insensitive)
	//   • chrom:pos    — равенство хромосомы и позиции
	//   • остальное    — подстрочный ILIKE по rs_id, координатам, ref/alt,
	//                    gene_symbol, HGVSc/HGVSp, ClinVar.
	if q := strings.TrimSpace(f.Q); q != "" {
		ql := strings.ToLower(q)
		switch {
		case strings.HasPrefix(ql, "rs") && len(q) > 2:
			args = append(args, q)
			ph := "$" + strconv.Itoa(len(args))
			conds = append(conds, "LOWER(v.rs_id) = LOWER("+ph+")")
		case strings.Contains(q, ":"):
			parts := strings.SplitN(q, ":", 2)
			chrom := strings.TrimSpace(parts[0])
			if strings.HasPrefix(strings.ToLower(chrom), "chr") {
				chrom = chrom[3:]
			}
			chrom = strings.ToUpper(chrom)
			args = append(args, chrom)
			cph := "$" + strconv.Itoa(len(args))
			sub := "v.chromosome = " + cph
			if posStr := strings.TrimSpace(parts[1]); posStr != "" {
				if pos, err := strconv.ParseInt(posStr, 10, 64); err == nil && pos > 0 {
					args = append(args, pos)
					pph := "$" + strconv.Itoa(len(args))
					sub += " AND v.position = " + pph
				}
			}
			conds = append(conds, "("+sub+")")
		default:
			args = append(args, "%"+q+"%")
			ph := "$" + strconv.Itoa(len(args))
			conds = append(conds, `(
				v.rs_id ILIKE `+ph+` OR
				v.chromosome ILIKE `+ph+` OR
				v.reference ILIKE `+ph+` OR
				v.alternate ILIKE `+ph+` OR
				CAST(v.position AS text) ILIKE `+ph+` OR
				EXISTS (
				  SELECT 1 FROM variant_annotation va2
				  LEFT JOIN gene g2 ON g2.gene_id = va2.gene_id
				  WHERE va2.variant_id = v.variant_id AND (
				    g2.gene_symbol ILIKE `+ph+` OR
				    va2.hgvsc ILIKE `+ph+` OR
				    va2.hgvsp ILIKE `+ph+` OR
				    va2.consequence ILIKE `+ph+` OR
				    va2.clinvar_significance ILIKE `+ph+` OR
				    va2.clinvar_id ILIKE `+ph+`
				  )
				)
			)`)
		}
	}
	where := strings.Join(conds, " AND ")

	limit := f.Limit
	offset := f.Offset
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit, offset)
	limitPH := "$" + strconv.Itoa(len(args)-1)
	offsetPH := "$" + strconv.Itoa(len(args))

	// top_ann — «топовая» аннотация варианта (HIGH→MODERATE→LOW→other);
	// её поля кладём в PatientVariantRich.Annotations как массив из одного
	// элемента, чтобы фронт мог рендерить HGVSc/HGVSp/gnomAD/SIFT/PolyPhen/
	// CADD/REVEL/ClinVar прямо в таблице, без отдельного запроса деталей.
	sql := `
		SELECT pv.patient_variant_id, pv.patient_id, pv.sample_id,
		       v.variant_id, v.chromosome, v.position, v.reference, v.alternate,
		       v.rs_id, v.genome_build::text, v.variant_type::text,
		       pv.zygosity::text, pv.quality, pv.read_depth,
		       pv.allele_depth_ref, pv.allele_depth_alt,
		       pv.genotype_quality, pv.filter_status, pv.detected_at,
		       COALESCE(
		          (SELECT array_agg(DISTINCT g.gene_symbol)
		             FROM variant_annotation va
		             LEFT JOIN gene g ON g.gene_id = va.gene_id
		            WHERE va.variant_id = v.variant_id AND g.gene_symbol IS NOT NULL),
		          ARRAY[]::varchar[]
		       ) AS gene_symbols,
		       top_ann.annotation_id, top_ann.gene_id,
		       top_ann.consequence, top_ann.impact,
		       top_ann.transcript_id, top_ann.hgvsc, top_ann.hgvsp,
		       top_ann.protein_position, top_ann.amino_acids, top_ann.codons,
		       top_ann.gnomad_af, top_ann.gnomad_af_popmax,
		       top_ann.sift_score, top_ann.sift_prediction,
		       top_ann.polyphen_score, top_ann.polyphen_prediction,
		       top_ann.cadd_score, top_ann.revel_score,
		       top_ann.clinvar_significance, top_ann.clinvar_id
		FROM patient_variant pv
		JOIN variant v ON v.variant_id = pv.variant_id
		LEFT JOIN LATERAL (
		    SELECT va.annotation_id, va.gene_id,
		           TRIM(BOTH '[]' FROM va.consequence) AS consequence,
		           va.impact::text AS impact,
		           va.transcript_id, va.hgvsc, va.hgvsp,
		           va.protein_position, va.amino_acids, va.codons,
		           va.gnomad_af, va.gnomad_af_popmax,
		           va.sift_score, va.sift_prediction,
		           va.polyphen_score, va.polyphen_prediction,
		           va.cadd_score, va.revel_score,
		           va.clinvar_significance, va.clinvar_id
		    FROM variant_annotation va
		    WHERE va.variant_id = v.variant_id
		    ORDER BY CASE va.impact::text
		      WHEN 'HIGH'     THEN 1
		      WHEN 'MODERATE' THEN 2
		      WHEN 'LOW'      THEN 3
		      ELSE 4 END,
		      va.annotation_id
		    LIMIT 1
		) top_ann ON TRUE
		WHERE ` + where + buildOrderBy(f.Sort) + `
		LIMIT ` + limitPH + ` OFFSET ` + offsetPH

	start := time.Now()
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		logQuery(ctx, "variant.ListPatientVariantsFiltered", sql, args, err, start)
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]models.PatientVariantRich, 0, limit)
	for rows.Next() {
		var pv models.PatientVariantRich
		var genes []string
		var (
			annID *int64
			ann   models.VariantAnnotation
		)
		if err := rows.Scan(
			&pv.PatientVariantID, &pv.PatientID, &pv.SampleID,
			&pv.Variant.VariantID, &pv.Variant.Chromosome, &pv.Variant.Position,
			&pv.Variant.Reference, &pv.Variant.Alternate,
			&pv.Variant.RsID, &pv.Variant.GenomeBuild, &pv.Variant.VariantType,
			&pv.Zygosity, &pv.Quality, &pv.ReadDepth,
			&pv.AlleleDepthRef, &pv.AlleleDepthAlt,
			&pv.GenotypeQuality, &pv.FilterStatus, &pv.DetectedAt,
			&genes,
			&annID, &ann.GeneID,
			&ann.Consequence, &ann.Impact,
			&ann.TranscriptID, &ann.HGVSc, &ann.HGVSp,
			&ann.ProteinPosition, &ann.AminoAcids, &ann.Codons,
			&ann.GnomadAF, &ann.GnomadAFPopmax,
			&ann.SIFTScore, &ann.SIFTPrediction,
			&ann.PolyphenScore, &ann.PolyphenPrediction,
			&ann.CADDScore, &ann.RevelScore,
			&ann.ClinvarSignificance, &ann.ClinvarID,
		); err != nil {
			logQuery(ctx, "variant.ListPatientVariantsFiltered", sql, args, err, start)
			return nil, 0, err
		}
		pv.GeneSymbols = genes
		// top_ann через LEFT JOIN LATERAL может вернуть NULL, если аннотаций нет.
		if annID != nil {
			ann.AnnotationID = *annID
			ann.VariantID = pv.Variant.VariantID
			pv.Annotations = []models.VariantAnnotation{ann}
			pv.TopConsequence = ann.Consequence
			pv.TopImpact = ann.Impact
		}
		out = append(out, pv)
	}
	if err := rows.Err(); err != nil {
		logQuery(ctx, "variant.ListPatientVariantsFiltered", sql, args, err, start)
		return nil, 0, err
	}
	logQuery(ctx, "variant.ListPatientVariantsFiltered", sql, args, nil, start)

	// Total
	countSQL := `SELECT count(*) FROM patient_variant pv
		JOIN variant v ON v.variant_id = pv.variant_id WHERE ` + where
	// args без LIMIT/OFFSET
	countArgs := args[:len(args)-2]
	var total int
	cstart := time.Now()
	cerr := r.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	logQuery(ctx, "variant.ListPatientVariantsFilteredCount", countSQL, countArgs, cerr, cstart)
	if cerr != nil {
		return nil, 0, cerr
	}
	return out, total, nil
}

// ListPatientVariants — обратно-совместимый враппер.
func (r *VariantRepo) ListPatientVariants(ctx context.Context, patientID, limit, offset int) ([]models.PatientVariant, error) {
	rich, _, err := r.ListPatientVariantsFiltered(ctx, patientID,
		models.VariantFilter{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]models.PatientVariant, 0, len(rich))
	for _, r := range rich {
		out = append(out, r.PatientVariant)
	}
	return out, nil
}

// GetVariant возвращает базовую запись варианта.
func (r *VariantRepo) GetVariant(ctx context.Context, id int) (models.Variant, error) {
	const q = `SELECT variant_id, chromosome, position, reference, alternate,
		rs_id, genome_build::text, variant_type::text
		FROM variant WHERE variant_id = $1`
	args := []any{id}
	start := time.Now()
	var v models.Variant
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&v.VariantID, &v.Chromosome, &v.Position, &v.Reference, &v.Alternate,
		&v.RsID, &v.GenomeBuild, &v.VariantType,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		logQuery(ctx, "variant.GetVariant", q, args, nil, start)
		return models.Variant{}, ErrNotFound
	}
	logQuery(ctx, "variant.GetVariant", q, args, err, start)
	return v, err
}

// GetVariantDetails — собирает агрегат для деталки варианта.
// userID нужен для фильтрации Interpretations и PatientCount по принадлежности
// пациента текущему пользователю (общий вариант — приватная когорта).
func (r *VariantRepo) GetVariantDetails(ctx context.Context, id, userID int) (models.VariantDetails, error) {
	v, err := r.GetVariant(ctx, id)
	if err != nil {
		return models.VariantDetails{}, err
	}
	out := models.VariantDetails{
		Variant:          v,
		Annotations:      []models.VariantAnnotation{},
		Genes:            []models.Gene{},
		Phenotypes:       []models.Phenotype{},
		Interpretations:  []models.VariantInterpretation{},
		EnrichmentStatus: "pending",
	}

	// enrichment_status — отдельным селектом, чтобы не ломать сигнатуру GetVariant.
	{
		const q = `SELECT enrichment_status FROM variant WHERE variant_id = $1`
		if err := r.pool.QueryRow(ctx, q, id).Scan(&out.EnrichmentStatus); err != nil {
			// не критично — оставим "pending"
			out.EnrichmentStatus = "pending"
		}
	}

	// Annotations
	{
		const q = `SELECT annotation_id, variant_id, gene_id,
			TRIM(BOTH '[]' FROM consequence) AS consequence,
			impact::text,
			transcript_id, hgvsc, hgvsp, protein_position, amino_acids, codons,
			gnomad_af, gnomad_af_popmax, sift_score, sift_prediction,
			polyphen_score, polyphen_prediction, cadd_score, revel_score,
			clinvar_significance, clinvar_id
			FROM variant_annotation WHERE variant_id = $1
			ORDER BY annotation_id`
		rows, err := r.pool.Query(ctx, q, id)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var a models.VariantAnnotation
			if err := rows.Scan(&a.AnnotationID, &a.VariantID, &a.GeneID, &a.Consequence,
				&a.Impact, &a.TranscriptID, &a.HGVSc, &a.HGVSp, &a.ProteinPosition,
				&a.AminoAcids, &a.Codons, &a.GnomadAF, &a.GnomadAFPopmax,
				&a.SIFTScore, &a.SIFTPrediction, &a.PolyphenScore, &a.PolyphenPrediction,
				&a.CADDScore, &a.RevelScore, &a.ClinvarSignificance, &a.ClinvarID); err != nil {
				rows.Close()
				return out, err
			}
			out.Annotations = append(out.Annotations, a)
		}
		rows.Close()
	}

	// Genes (через annotation.gene_id)
	{
		const q = `SELECT DISTINCT g.gene_id, g.gene_symbol, g.gene_name, g.ensembl_gene_id,
			g.ncbi_gene_id, g.omim_gene_id, g.chromosome, g.start_position, g.end_position,
			g.strand::text, g.gene_description
			FROM gene g
			JOIN variant_annotation va ON va.gene_id = g.gene_id
			WHERE va.variant_id = $1`
		rows, err := r.pool.Query(ctx, q, id)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var g models.Gene
			if err := rows.Scan(&g.GeneID, &g.GeneSymbol, &g.GeneName, &g.EnsemblGeneID,
				&g.NCBIGeneID, &g.OMIMGeneID, &g.Chromosome, &g.StartPosition, &g.EndPosition,
				&g.Strand, &g.GeneDescription); err != nil {
				rows.Close()
				return out, err
			}
			out.Genes = append(out.Genes, g)
		}
		rows.Close()
	}

	// Phenotypes (через gene_phenotype)
	{
		const q = `SELECT DISTINCT p.phenotype_id, p.phenotype_name, p.omim_phenotype_id,
			p.orpha_code, p.inheritance_pattern::text, p.description
			FROM phenotype p
			JOIN gene_phenotype gp ON gp.phenotype_id = p.phenotype_id
			JOIN variant_annotation va ON va.gene_id = gp.gene_id
			WHERE va.variant_id = $1`
		rows, err := r.pool.Query(ctx, q, id)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var p models.Phenotype
			if err := rows.Scan(&p.PhenotypeID, &p.PhenotypeName, &p.OMIMPhenotypeID,
				&p.OrphaCode, &p.InheritancePattern, &p.Description); err != nil {
				rows.Close()
				return out, err
			}
			out.Phenotypes = append(out.Phenotypes, p)
		}
		rows.Close()
	}

	// Interpretations (через patient_variant.variant_id = $1)
	// Фильтруем по принадлежности пациента текущему пользователю.
	{
		const q = `SELECT vi.interpretation_id, vi.patient_variant_id, vi.user_id,
			vi.acmg_classification::text, vi.interpretation_text, vi.disease_id,
			vi.created_at, vi.updated_at
			FROM variant_interpretation vi
			JOIN patient_variant pv ON pv.patient_variant_id = vi.patient_variant_id
			JOIN patient p ON p.patient_id = pv.patient_id
			WHERE pv.variant_id = $1
			  AND (p.created_by_user_id = $2 OR p.created_by_user_id IS NULL)
			ORDER BY vi.updated_at DESC`
		rows, err := r.pool.Query(ctx, q, id, userID)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var vi models.VariantInterpretation
			if err := rows.Scan(&vi.InterpretationID, &vi.PatientVariantID, &vi.UserID,
				&vi.ACMGClassification, &vi.InterpretationText, &vi.DiseaseID,
				&vi.CreatedAt, &vi.UpdatedAt); err != nil {
				rows.Close()
				return out, err
			}
			out.Interpretations = append(out.Interpretations, vi)
		}
		rows.Close()
	}

	// Patient count — только когорта текущего пользователя.
	{
		const q = `SELECT count(DISTINCT pv.patient_id)
			FROM patient_variant pv
			JOIN patient p ON p.patient_id = pv.patient_id
			WHERE pv.variant_id = $1
			  AND (p.created_by_user_id = $2 OR p.created_by_user_id IS NULL)`
		if err := r.pool.QueryRow(ctx, q, id, userID).Scan(&out.PatientCount); err != nil {
			return out, err
		}
	}

	return out, nil
}

// classifyVariant — простая классификация по длинам ref/alt.
func classifyVariant(ref, alt string) string {
	switch {
	case len(ref) == 1 && len(alt) == 1:
		return "SNV"
	case len(ref) > len(alt):
		return "DEL"
	case len(ref) < len(alt):
		return "INS"
	case len(ref) == len(alt) && len(ref) > 1:
		return "MNV"
	default:
		return "OTHER"
	}
}

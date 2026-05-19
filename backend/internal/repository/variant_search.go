package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/term-paper-2026/backend/internal/models"
)

// SearchVariants — поиск по бд вариантов (Модуль 6 ТЗ).
// Возвращает страницу результатов и total. PatientCount считается только
// по пациентам, принадлежащим userID (или legacy NULL).
func (r *VariantRepo) SearchVariants(
	ctx context.Context, f models.VariantSearchFilter, userID int,
) ([]models.VariantSearchResult, int, error) {
	// Условия WHERE и аргументы строим динамически.
	where := []string{}
	args := []any{}
	addArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	// Глобальный поиск q — приоритет: rs_id → gene_symbol → координаты.
	if q := strings.TrimSpace(f.Q); q != "" {
		// Если выглядит как rsXXX, ищем по rs_id (case-insensitive).
		// Если содержит ':' → координаты chrom:pos.
		// Иначе считаем именем гена.
		ql := strings.ToLower(q)
		switch {
		case strings.HasPrefix(ql, "rs"):
			ph := addArg(q)
			where = append(where, fmt.Sprintf("LOWER(v.rs_id) = LOWER(%s)", ph))
		case strings.Contains(q, ":"):
			parts := strings.SplitN(q, ":", 2)
			if len(parts) == 2 {
				ph1 := addArg(normalizeChrom(parts[0]))
				where = append(where, fmt.Sprintf(
					"regexp_replace(UPPER(v.chromosome), '^CHR', '') = %s", ph1))
				if parts[1] != "" {
					if pos, err := parsePositiveInt(parts[1]); err == nil {
						ph2 := addArg(pos)
						where = append(where, fmt.Sprintf("v.position = %s", ph2))
					}
				}
			}
		default:
			ph := addArg(q)
			where = append(where,
				fmt.Sprintf(`v.variant_id IN (
					SELECT va.variant_id FROM variant_annotation va
					JOIN gene g ON g.gene_id = va.gene_id
					WHERE LOWER(g.gene_symbol) = LOWER(%s)
				)`, ph))
		}
	}

	if rs := strings.TrimSpace(f.RsID); rs != "" {
		ph := addArg(rs)
		where = append(where, fmt.Sprintf("LOWER(v.rs_id) = LOWER(%s)", ph))
	}
	if gene := strings.TrimSpace(f.Gene); gene != "" {
		ph := addArg(gene)
		where = append(where,
			fmt.Sprintf(`v.variant_id IN (
				SELECT va.variant_id FROM variant_annotation va
				JOIN gene g ON g.gene_id = va.gene_id
				WHERE LOWER(g.gene_symbol) = LOWER(%s)
			)`, ph))
	}
	if c := strings.TrimSpace(f.Chrom); c != "" {
		// БД может хранить как "chr1", так и "1" — сравниваем без префикса.
		ph := addArg(normalizeChrom(c))
		where = append(where, fmt.Sprintf(
			"regexp_replace(UPPER(v.chromosome), '^CHR', '') = %s", ph))
	}
	if f.Pos != nil {
		ph := addArg(*f.Pos)
		where = append(where, fmt.Sprintf("v.position = %s", ph))
	}
	if ref := strings.TrimSpace(f.Ref); ref != "" {
		ph := addArg(strings.ToUpper(ref))
		where = append(where, fmt.Sprintf("UPPER(v.reference) = %s", ph))
	}
	if alt := strings.TrimSpace(f.Alt); alt != "" {
		ph := addArg(strings.ToUpper(alt))
		where = append(where, fmt.Sprintf("UPPER(v.alternate) = %s", ph))
	}
	if b := strings.TrimSpace(f.Build); b != "" {
		ph := addArg(b)
		where = append(where, fmt.Sprintf("v.genome_build::text = %s", ph))
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	// Считаем total.
	// Без фильтров COUNT(*) по 500k+ строк занимает 10+ сек — используем
	// быстрый estimate из pg_class (погрешность ±несколько %, но мгновенно).
	// С фильтрами делаем точный COUNT по отфильтрованному подмножеству.
	var total int
	if whereSQL == "" {
		const estimateQ = `SELECT reltuples::bigint FROM pg_class WHERE relname = 'variant'`
		if err := r.pool.QueryRow(ctx, estimateQ).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("estimate variants: %w", err)
		}
	} else {
		countQ := `SELECT COUNT(*) FROM variant v ` + whereSQL
		if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count variants: %w", err)
		}
	}
	if total == 0 {
		return []models.VariantSearchResult{}, 0, nil
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	// userID для коррелированного подсчёта.
	uidPh := addArg(userID)
	limPh := addArg(limit)
	offPh := addArg(offset)

	q := `
		SELECT
			v.variant_id, v.chromosome, v.position, v.reference, v.alternate,
			v.rs_id, v.genome_build::text, v.variant_type::text,
			COALESCE(
				(SELECT array_agg(DISTINCT g.gene_symbol)
				   FROM variant_annotation va
				   JOIN gene g ON g.gene_id = va.gene_id
				  WHERE va.variant_id = v.variant_id AND g.gene_symbol IS NOT NULL),
				ARRAY[]::text[]
			) AS gene_symbols,
			(SELECT COUNT(DISTINCT pv.patient_id)
			   FROM patient_variant pv
			   JOIN patient p ON p.patient_id = pv.patient_id
			  WHERE pv.variant_id = v.variant_id
			    AND (p.created_by_user_id = ` + uidPh + ` OR p.created_by_user_id IS NULL)
			) AS patient_count,
			(SELECT va.impact::text
			   FROM variant_annotation va
			  WHERE va.variant_id = v.variant_id
			  ORDER BY CASE va.impact::text
			    WHEN 'HIGH' THEN 1 WHEN 'MODERATE' THEN 2
			    WHEN 'LOW' THEN 3 WHEN 'MODIFIER' THEN 4 ELSE 5
			  END
			  LIMIT 1) AS top_impact,
			(SELECT va.clinvar_significance
			   FROM variant_annotation va
			  WHERE va.variant_id = v.variant_id AND va.clinvar_significance IS NOT NULL
			  LIMIT 1) AS clinvar_significance
		FROM variant v
		` + whereSQL + `
		ORDER BY v.variant_id
		LIMIT ` + limPh + ` OFFSET ` + offPh

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search variants: %w", err)
	}
	defer rows.Close()

	out := make([]models.VariantSearchResult, 0, limit)
	for rows.Next() {
		var item models.VariantSearchResult
		var genes []string
		if err := rows.Scan(
			&item.Variant.VariantID, &item.Variant.Chromosome, &item.Variant.Position,
			&item.Variant.Reference, &item.Variant.Alternate,
			&item.Variant.RsID, &item.Variant.GenomeBuild, &item.Variant.VariantType,
			&genes, &item.PatientCount, &item.TopImpact, &item.ClinVarSignificance,
		); err != nil {
			return nil, 0, fmt.Errorf("scan search row: %w", err)
		}
		item.GeneSymbols = genes
		if item.GeneSymbols == nil {
			item.GeneSymbols = []string{}
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// normalizeChrom приводит "chr1"/"1" к каноничному формату как в таблице
// variant.chromosome (которое хранит "1", "X", "MT" — без префикса chr).
func normalizeChrom(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "chr")
	return strings.ToUpper(s)
}

// parsePositiveInt — strconv.ParseInt с проверкой >0.
func parsePositiveInt(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int64(c-'0')
	}
	if n <= 0 {
		return 0, fmt.Errorf("non-positive")
	}
	return n, nil
}

// Package pipeline — CSV-парсер для модуля 4 backend-spec-tz.
package pipeline

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/term-paper-2026/backend/internal/models"
)

// ErrMissingCSVColumns возвращается, когда в шапке CSV отсутствует
// хотя бы одна обязательная колонка. Errors.Is + сообщение со списком.
var ErrMissingCSVColumns = errors.New("missing required CSV columns")

// requiredCSVColumns — обязательные заголовки (case-insensitive).
var requiredCSVColumns = []string{
	"chromosome",
	"position",
	"reference_allele",
	"alternate_allele",
}

// ParseCSV парсит CSV-таблицу вариантов в []VCFRecord + warnings.
// Заголовки case-insensitive, порядок произвольный, лишние колонки игнорируются.
// На каждую отброшенную строку возвращается одна строка warning с причиной.
// Если отсутствует обязательная колонка — возвращается ErrMissingCSVColumns
// (обёрнутая через fmt.Errorf с перечислением недостающих).
func ParseCSV(r io.Reader, delim rune) ([]models.VCFRecord, []string, error) {
	if delim == 0 {
		delim = ','
	}
	cr := csv.NewReader(r)
	cr.Comma = delim
	cr.FieldsPerRecord = -1   // допускаем разное число колонок
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}

	// Нормализуем заголовки и строим map: имя → индекс колонки.
	idx := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key != "" {
			idx[key] = i
		}
	}

	// Проверяем обязательные колонки.
	var missing []string
	for _, c := range requiredCSVColumns {
		if _, ok := idx[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrMissingCSVColumns, strings.Join(missing, ", "))
	}

	var out []models.VCFRecord
	var warnings []string
	lineNo := 1 // header — строка 1
	for {
		row, err := cr.Read()
		lineNo++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: read error: %s", lineNo, err.Error()))
			continue
		}

		get := func(col string) string {
			i, ok := idx[col]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		chrom := get("chromosome")
		posStr := get("position")
		ref := strings.ToUpper(get("reference_allele"))
		alt := strings.ToUpper(get("alternate_allele"))

		if chrom == "" || posStr == "" || ref == "" || alt == "" {
			warnings = append(warnings, fmt.Sprintf("line %d: empty required field", lineNo))
			continue
		}
		pos, err := strconv.ParseInt(posStr, 10, 64)
		if err != nil || pos <= 0 {
			warnings = append(warnings, fmt.Sprintf("line %d: bad position %q", lineNo, posStr))
			continue
		}

		rec := models.VCFRecord{
			Chromosome: chrom,
			Position:   pos,
			Reference:  ref,
			Alternate:  alt,
			Filter:     "PASS",
		}

		if v := get("rs_id"); v != "" {
			rs := v
			rec.RsID = &rs
		}
		if v := get("genome_build"); v != "" {
			b := v
			rec.GenomeBuild = &b
		}
		if v := get("variant_type"); v != "" {
			t := strings.ToUpper(v)
			rec.VariantType = &t
		}
		if v := get("zygosity"); v != "" {
			z := strings.ToLower(v)
			// Spec допускает HETEROZYGOUS/HOMOZYGOUS/HEMIZYGOUS,
			// в БД zygosity_type — нижний регистр.
			rec.Zygosity = &z
		}
		if v := get("quality"); v != "" {
			if q, err := strconv.ParseFloat(v, 64); err == nil {
				rec.Quality = &q
			}
		}
		if v := get("read_depth"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				rec.ReadDepth = &n
			}
		}
		if v := get("filter_status"); v != "" {
			rec.Filter = v
		}

		out = append(out, rec)
	}

	return out, warnings, nil
}

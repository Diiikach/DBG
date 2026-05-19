package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/term-paper-2026/backend/internal/models"
)

// ErrPatientVariantExists возвращается, когда у пациента уже есть запись
// с тем же variant_id (нарушение уникального ключа patient_variant_uniq).
var ErrPatientVariantExists = errors.New("patient_variant already exists")

// AddManualVariant — атомарно upsert-ит variant и вставляет patient_variant
// без sample_id (ручной ввод). Возвращает variantID и patient_variant_id.
// Если у пациента уже есть запись с этим variant_id — ErrPatientVariantExists.
func (r *VariantRepo) AddManualVariant(
	ctx context.Context, patientID int, v models.VCFRecord, defaultBuild string,
) (variantID int, patientVariantID int64, err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("tx begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	vid, _, err := r.UpsertVariant(ctx, tx, v, defaultBuild)
	if err != nil {
		return 0, 0, err
	}

	// Вставляем patient_variant БЕЗ ON CONFLICT, чтобы поймать 23505 и вернуть 409.
	const insertQ = `
		INSERT INTO patient_variant
			(patient_id, variant_id, sample_id, zygosity, quality, read_depth,
			 allele_depth_ref, allele_depth_alt, genotype_quality, filter_status,
			 detected_at)
		VALUES ($1, $2, NULL, $3::zygosity_type, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP)
		RETURNING patient_variant_id
	`
	args := []any{
		patientID, vid, v.Zygosity, v.Quality, v.ReadDepth,
		v.AlleleDepthRef, v.AlleleDepthAlt, v.GenotypeQuality, v.Filter,
	}
	var pvID int64
	if err := tx.QueryRow(ctx, insertQ, args...).Scan(&pvID); err != nil {
		if isUniqueViolation(err) {
			return 0, 0, ErrPatientVariantExists
		}
		return 0, 0, fmt.Errorf("insert patient_variant: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("tx commit: %w", err)
	}
	return vid, pvID, nil
}

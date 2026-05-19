package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/term-paper-2026/backend/internal/models"
)

// SetEnrichmentStatus обновляет variant.enrichment_status.
// status ∈ {"pending","ok","failed","skipped"}.
func (r *VariantRepo) SetEnrichmentStatus(ctx context.Context, variantID int, status string) error {
	const q = `UPDATE variant SET enrichment_status = $1 WHERE variant_id = $2`
	args := []any{status, variantID}
	start := time.Now()
	_, err := r.pool.Exec(ctx, q, args...)
	logQuery(ctx, "variant.SetEnrichmentStatus", q, args, err, start)
	return err
}

// UpsertGeneBySymbol находит или создаёт ген по gene_symbol, возвращает gene_id.
// Используется в enrichment-задаче, когда внешний API вернул symbol, которого
// нет в локальной таблице gene.
func (r *VariantRepo) UpsertGeneBySymbol(ctx context.Context, symbol string) (int, error) {
	if symbol == "" {
		return 0, errors.New("empty gene symbol")
	}
	const q = `
		INSERT INTO gene (gene_symbol)
		VALUES ($1)
		ON CONFLICT (gene_symbol) WHERE gene_symbol IS NOT NULL
		DO UPDATE SET gene_symbol = EXCLUDED.gene_symbol
		RETURNING gene_id
	`
	args := []any{symbol}
	start := time.Now()
	var id int
	err := r.pool.QueryRow(ctx, q, args...).Scan(&id)
	logQuery(ctx, "variant.UpsertGeneBySymbol", q, args, err, start)
	if err != nil {
		return 0, fmt.Errorf("upsert gene: %w", err)
	}
	return id, nil
}

// EnrichmentAnnotation — минимальный набор полей, который пишет enrichment-задача.
// Все поля опциональны (nil значит «нет данных»).
type EnrichmentAnnotation struct {
	GeneID              *int
	Consequence         *string
	Impact              *string
	GnomadAF            *float64
	GnomadAFPopmax      *float64
	SIFTScore           *float64
	SIFTPrediction      *string
	PolyphenScore       *float64
	PolyphenPrediction  *string
	CADDScore           *float64
	RevelScore          *float64
	ClinvarSignificance *string
	ClinvarID           *string
}

// ReplaceVariantAnnotation удаляет старые enrichment-аннотации варианта
// (transcript_id IS NULL — наш маркер «from external API»)
// и вставляет новую запись. Делает идемпотентно: повторный enrich не накапливает строки.
func (r *VariantRepo) ReplaceVariantAnnotation(
	ctx context.Context, variantID int, a EnrichmentAnnotation,
) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("tx begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Удаляем старые external-аннотации (без transcript_id) — оставляем те, что
	// пришли из VEP/локального аннотатора (с transcript_id), если такие есть.
	const delQ = `DELETE FROM variant_annotation
		WHERE variant_id = $1 AND transcript_id IS NULL`
	if _, err := tx.Exec(ctx, delQ, variantID); err != nil {
		return fmt.Errorf("delete old ann: %w", err)
	}

	// impact — нативный ENUM impact_type, пишем через текст (nil → NULL).
	const insQ = `
		INSERT INTO variant_annotation
			(variant_id, gene_id, consequence, impact,
			 gnomad_af, gnomad_af_popmax,
			 sift_score, sift_prediction,
			 polyphen_score, polyphen_prediction,
			 cadd_score, revel_score,
			 clinvar_significance, clinvar_id)
		VALUES ($1, $2, $3, $4::impact_type,
		        $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	args := []any{
		variantID, a.GeneID, a.Consequence, a.Impact,
		a.GnomadAF, a.GnomadAFPopmax,
		a.SIFTScore, a.SIFTPrediction,
		a.PolyphenScore, a.PolyphenPrediction,
		a.CADDScore, a.RevelScore,
		a.ClinvarSignificance, a.ClinvarID,
	}
	start := time.Now()
	_, err = tx.Exec(ctx, insQ, args...)
	logQuery(ctx, "variant.ReplaceVariantAnnotation", insQ, args, err, start)
	if err != nil {
		return fmt.Errorf("insert ann: %w", err)
	}
	return tx.Commit(ctx)
}

// GetVariantForEnrichment возвращает координаты варианта, нужные для построения HGVS.
func (r *VariantRepo) GetVariantForEnrichment(ctx context.Context, variantID int) (models.Variant, error) {
	return r.GetVariant(ctx, variantID)
}

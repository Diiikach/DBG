package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/repository"
)

// EnrichmentRepo — подмножество методов VariantRepo, нужное для enrichment.
// Объявлено как интерфейс, чтобы упростить unit-тесты.
type EnrichmentRepo interface {
	GetVariantForEnrichment(ctx context.Context, variantID int) (variantCoords, error)
	UpsertGeneBySymbol(ctx context.Context, symbol string) (int, error)
	ReplaceVariantAnnotation(ctx context.Context, variantID int, a repository.EnrichmentAnnotation) error
	SetEnrichmentStatus(ctx context.Context, variantID int, status string) error
}

// variantCoords — то, что нам нужно знать о варианте для построения HGVS-ID.
type variantCoords struct {
	Chromosome string
	Position   int64
	Reference  string
	Alternate  string
}

// repoAdapter оборачивает *repository.VariantRepo и приводит models.Variant → variantCoords.
type repoAdapter struct{ inner *repository.VariantRepo }

func (a repoAdapter) GetVariantForEnrichment(ctx context.Context, vid int) (variantCoords, error) {
	v, err := a.inner.GetVariantForEnrichment(ctx, vid)
	if err != nil {
		return variantCoords{}, err
	}
	return variantCoords{
		Chromosome: v.Chromosome,
		Position:   v.Position,
		Reference:  v.Reference,
		Alternate:  v.Alternate,
	}, nil
}
func (a repoAdapter) UpsertGeneBySymbol(ctx context.Context, s string) (int, error) {
	return a.inner.UpsertGeneBySymbol(ctx, s)
}
func (a repoAdapter) ReplaceVariantAnnotation(ctx context.Context, vid int, ann repository.EnrichmentAnnotation) error {
	return a.inner.ReplaceVariantAnnotation(ctx, vid, ann)
}
func (a repoAdapter) SetEnrichmentStatus(ctx context.Context, vid int, status string) error {
	return a.inner.SetEnrichmentStatus(ctx, vid, status)
}

// Enricher — оркестратор обогащения. Потокобезопасен.
type Enricher struct {
	repo    EnrichmentRepo
	client  *MyVariantClient
	enabled bool

	// rate-limiter: токен раз в interval. Без внешних зависимостей.
	mu       sync.Mutex
	lastCall time.Time
	interval time.Duration
}

// NewEnricher создаёт Enricher с rate-limit = 1/rps.
func NewEnricher(vr *repository.VariantRepo, client *MyVariantClient, enabled bool, rps int) *Enricher {
	if rps <= 0 {
		rps = 1
	}
	return &Enricher{
		repo:     repoAdapter{inner: vr},
		client:   client,
		enabled:  enabled,
		interval: time.Second / time.Duration(rps),
	}
}

// Enabled — true, если обогащение включено в конфиге.
func (e *Enricher) Enabled() bool { return e.enabled }

// waitForToken блокирует горутину до следующего «слота» rate-limiter'а.
func (e *Enricher) waitForToken(ctx context.Context) error {
	e.mu.Lock()
	wait := time.Until(e.lastCall.Add(e.interval))
	if wait <= 0 {
		e.lastCall = time.Now()
		e.mu.Unlock()
		return nil
	}
	// бронируем «слот» заранее, чтобы конкурирующие горутины ждали корректно
	e.lastCall = e.lastCall.Add(e.interval)
	e.mu.Unlock()

	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Enrich выполняет обогащение для одного variant_id. Всегда обновляет
// enrichment_status (ok/failed/skipped) — это часть контракта.
func (e *Enricher) Enrich(ctx context.Context, variantID int) error {
	log := logging.FromContext(ctx).With(
		slog.String("op", "enrichment.Enrich"),
		slog.Int("variant_id", variantID),
	)

	if !e.enabled {
		_ = e.repo.SetEnrichmentStatus(ctx, variantID, "skipped")
		log.LogAttrs(ctx, slog.LevelDebug, "enrichment disabled, skipped")
		return nil
	}

	v, err := e.repo.GetVariantForEnrichment(ctx, variantID)
	if err != nil {
		_ = e.repo.SetEnrichmentStatus(ctx, variantID, "failed")
		log.LogAttrs(ctx, slog.LevelError, "load variant failed", slog.String("error", err.Error()))
		return err
	}

	hgvs := buildHGVS(v.Chromosome, v.Position, v.Reference, v.Alternate)
	if hgvs == "" {
		_ = e.repo.SetEnrichmentStatus(ctx, variantID, "skipped")
		log.LogAttrs(ctx, slog.LevelDebug, "cannot build HGVS, skipped")
		return nil
	}

	if err := e.waitForToken(ctx); err != nil {
		// контекст отменён — это не «провал API», статус оставим как был.
		return err
	}

	data, err := e.client.Fetch(ctx, hgvs)
	if err != nil {
		_ = e.repo.SetEnrichmentStatus(ctx, variantID, "failed")
		log.LogAttrs(ctx, slog.LevelWarn, "myvariant fetch failed",
			slog.String("hgvs", hgvs), slog.String("error", err.Error()))
		return err
	}
	if data == nil {
		_ = e.repo.SetEnrichmentStatus(ctx, variantID, "skipped")
		return nil
	}

	ann := repository.EnrichmentAnnotation{
		Consequence:         data.Consequence,
		Impact:              data.Impact,
		GnomadAF:            data.GnomadAF,
		GnomadAFPopmax:      data.GnomadAFPopmax,
		SIFTScore:           data.SIFTScore,
		SIFTPrediction:      data.SIFTPrediction,
		PolyphenScore:       data.PolyphenScore,
		PolyphenPrediction:  data.PolyphenPrediction,
		CADDScore:           data.CADDScore,
		RevelScore:          data.RevelScore,
		ClinvarSignificance: data.ClinvarSignificance,
		ClinvarID:           data.ClinvarID,
	}
	// gene_symbol → upsert в gene, ставим gene_id в аннотации.
	if data.GeneSymbol != nil && *data.GeneSymbol != "" {
		gid, gerr := e.repo.UpsertGeneBySymbol(ctx, strings.TrimSpace(*data.GeneSymbol))
		if gerr == nil {
			ann.GeneID = &gid
		} else {
			log.LogAttrs(ctx, slog.LevelWarn, "upsert gene failed",
				slog.String("symbol", *data.GeneSymbol), slog.String("error", gerr.Error()))
		}
	}

	if err := e.repo.ReplaceVariantAnnotation(ctx, variantID, ann); err != nil {
		_ = e.repo.SetEnrichmentStatus(ctx, variantID, "failed")
		log.LogAttrs(ctx, slog.LevelError, "write annotation failed",
			slog.String("error", err.Error()))
		return err
	}
	_ = e.repo.SetEnrichmentStatus(ctx, variantID, "ok")
	log.LogAttrs(ctx, slog.LevelInfo, "enrichment ok", slog.String("hgvs", hgvs))
	return nil
}

// buildHGVS строит строку HGVS-id вида "chr1:g.123456A>G", понятную MyVariant.info.
// Для INDEL'ов поддержка простая (insertion/deletion), exotic cases пропускаем (return "").
func buildHGVS(chrom string, pos int64, ref, alt string) string {
	chrom = strings.TrimSpace(chrom)
	ref = strings.ToUpper(strings.TrimSpace(ref))
	alt = strings.ToUpper(strings.TrimSpace(alt))
	if chrom == "" || pos <= 0 || ref == "" || alt == "" {
		return ""
	}
	// нормализация: всегда с префиксом chr
	if !strings.HasPrefix(strings.ToLower(chrom), "chr") {
		chrom = "chr" + chrom
	}
	switch {
	case len(ref) == 1 && len(alt) == 1:
		// SNV: chr1:g.123A>G
		return fmt.Sprintf("%s:g.%d%s>%s", chrom, pos, ref, alt)
	case len(ref) > 1 && len(alt) == 1 && strings.HasPrefix(ref, alt):
		// Deletion: chr1:g.124_125del (удалены ref[1:])
		delStart := pos + 1
		delEnd := pos + int64(len(ref)-1)
		if delStart == delEnd {
			return fmt.Sprintf("%s:g.%ddel", chrom, delStart)
		}
		return fmt.Sprintf("%s:g.%d_%ddel", chrom, delStart, delEnd)
	case len(alt) > 1 && len(ref) == 1 && strings.HasPrefix(alt, ref):
		// Insertion: chr1:g.123_124insXXX
		ins := alt[1:]
		return fmt.Sprintf("%s:g.%d_%dins%s", chrom, pos, pos+1, ins)
	default:
		// сложные случаи (MNV/complex) — пропускаем
		return ""
	}
}

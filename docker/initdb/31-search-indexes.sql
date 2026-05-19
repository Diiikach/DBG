-- Индексы для быстрого поиска вариантов (GET /api/variants).
-- Коррелированные подзапросы в SearchVariants используют variant_id
-- для поиска аннотаций, генов и patient_count — без индексов это seq-scan
-- по 500k+ строк на каждую из 50 строк результата (~10 сек → ~0.3 сек).

CREATE INDEX IF NOT EXISTS idx_variant_annotation_variant_id
    ON variant_annotation(variant_id);

-- Составной индекс для подзапроса top_impact (ORDER BY CASE impact).
CREATE INDEX IF NOT EXISTS idx_variant_annotation_variant_impact
    ON variant_annotation(variant_id, impact);

-- Частичный индекс для подзапроса clinvar_significance (WHERE IS NOT NULL).
CREATE INDEX IF NOT EXISTS idx_variant_annotation_clinvar
    ON variant_annotation(variant_id, clinvar_significance)
    WHERE clinvar_significance IS NOT NULL;

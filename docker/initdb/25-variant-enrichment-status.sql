-- Модуль 7: статус обогащения варианта через MyVariant.info / ClinVar.
-- pending — задача поставлена, ещё не выполнена;
-- ok      — данные получены и записаны в variant_annotation;
-- failed  — внешний API ответил ошибкой / таймаут;
-- skipped — обогащение отключено в конфиге (ENRICHMENT_ENABLED=false).

ALTER TABLE public.variant
    ADD COLUMN IF NOT EXISTS enrichment_status varchar(20) NOT NULL DEFAULT 'pending';

CREATE INDEX IF NOT EXISTS variant_enrichment_status_idx
    ON public.variant (enrichment_status);

-- Уникальный индекс на gene_symbol — нужен для UPSERT-а гена в enrichment.
-- Партиальный (WHERE NOT NULL), чтобы не мешать историческим NULL-строкам.
CREATE UNIQUE INDEX IF NOT EXISTS gene_symbol_uniq
    ON public.gene (gene_symbol) WHERE gene_symbol IS NOT NULL;

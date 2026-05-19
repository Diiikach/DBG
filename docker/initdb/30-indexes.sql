-- Выполняется после восстановления дампа.
-- Дополнительные индексы для частых паттернов доступа из API.
--
-- patient_variant.variant_id используется в:
--   • GetVariantDetails  → count(DISTINCT patient_id) и JOIN для интерпретаций
--   • CohortByVariant    → выборка пациентов с заданным вариантом
--
-- Существующий уникальный индекс (patient_id, variant_id) не подходит для
-- запросов с фильтром только по variant_id — Postgres делает full scan
-- по 70+ млн строк, что приводит к зависанию запроса.

CREATE INDEX IF NOT EXISTS idx_patient_variant_variant_id
    ON public.patient_variant (variant_id);

-- В дампе у таблицы variant не пришли ни первичный ключ, ни уникальный индекс
-- по локусу. Они необходимы пайплайну: SavePipelineResults выполняет
-- INSERT ... ON CONFLICT (chromosome, position, reference, alternate, genome_build).
-- Без них любая загрузка sample падает с SQLSTATE 42P10.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE t.relname = 'variant' AND c.contype = 'p'
    ) THEN
        ALTER TABLE public.variant ADD PRIMARY KEY (variant_id);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'variant_uniq_locus'
    ) THEN
        ALTER TABLE public.variant
            ADD CONSTRAINT variant_uniq_locus
            UNIQUE (chromosome, position, reference, alternate, genome_build);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'patient_variant_uniq'
    ) THEN
        ALTER TABLE public.patient_variant
            ADD CONSTRAINT patient_variant_uniq
            UNIQUE (patient_id, variant_id);
    END IF;
END $$;

-- Поле variant.chromosome в дампе пришло как varchar(5) — слишком мало для
-- RefSeq-аксешнов вида "NC_000022.11". Расширяем до varchar(32) идемпотентно.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'variant'
          AND column_name = 'chromosome'
          AND character_maximum_length IS NOT NULL
          AND character_maximum_length < 32
    ) THEN
        ALTER TABLE public.variant
            ALTER COLUMN chromosome TYPE varchar(32);
    END IF;
END $$;

-- Колонка sample.failure_reason для хранения причины провала пайплайна.
ALTER TABLE public.sample
    ADD COLUMN IF NOT EXISTS failure_reason TEXT;

-- Дамп оставил sequence'ы вариантных таблиц на минимуме (1), потому что вставка
-- шла напрямую с указанными id. Без ресинка любой INSERT через nextval()
-- падает с дубликатом PK (SQLSTATE 23505) на первой же удачной попытке.
DO $$
DECLARE
    t record;
BEGIN
    FOR t IN
        SELECT 'variant'::regclass AS tbl, 'variant_id' AS col
        UNION ALL SELECT 'patient_variant'::regclass, 'patient_variant_id'
        UNION ALL SELECT 'sample'::regclass, 'sample_id'
    LOOP
        EXECUTE format(
            'SELECT setval(pg_get_serial_sequence(%L, %L), (SELECT COALESCE(max(%I), 1) FROM %s))',
            t.tbl::text, t.col, t.col, t.tbl::text
        );
    END LOOP;
END $$;

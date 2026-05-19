-- Модуль 6: индексы под GET /api/variants?rs_id=&gene=&chrom=&pos=
-- и базовый join variant ← variant_annotation ← gene.

CREATE INDEX IF NOT EXISTS variant_rs_id_idx
    ON public.variant (rs_id);

CREATE INDEX IF NOT EXISTS variant_locus_idx
    ON public.variant (chromosome, position, genome_build);

CREATE INDEX IF NOT EXISTS gene_symbol_idx
    ON public.gene (gene_symbol);

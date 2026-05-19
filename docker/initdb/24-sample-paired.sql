-- Модуль 8: paired-end FASTQ — храним пути обоих файлов на sample,
-- чтобы можно было перезапустить пайплайн и отдать в bwa mem R1+R2.

ALTER TABLE public.sample
    ADD COLUMN IF NOT EXISTS r1_path TEXT,
    ADD COLUMN IF NOT EXISTS r2_path TEXT;

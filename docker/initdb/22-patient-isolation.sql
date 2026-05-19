-- Модуль 2: авторизационная изоляция пациентов (ТЗ 4.5.4).
-- Каждый пациент принадлежит создавшему его пользователю; legacy-записи
-- (до миграции) остаются с NULL и видны только в обходных режимах.

ALTER TABLE public.patient
    ADD COLUMN IF NOT EXISTS created_by_user_id int;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'patient_created_by_user_fk'
    ) THEN
        ALTER TABLE public.patient
            ADD CONSTRAINT patient_created_by_user_fk
            FOREIGN KEY (created_by_user_id)
            REFERENCES public."user"(user_id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS patient_created_by_idx
    ON public.patient (created_by_user_id);

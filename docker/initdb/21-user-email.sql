-- Модуль 1: расширение public."user" под требования ТЗ 4.1.1 п.1
-- email, first_name, last_name присутствуют ещё с 05-user-table.sql,
-- здесь оставляем идемпотентные ALTER для надёжности и добавляем
-- регистронезависимый UNIQUE-индекс по email.

ALTER TABLE public."user"
    ADD COLUMN IF NOT EXISTS email      varchar(255),
    ADD COLUMN IF NOT EXISTS first_name varchar(120),
    ADD COLUMN IF NOT EXISTS last_name  varchar(120);

-- Снимаем старый «обычный» UNIQUE (email), если он был создан 05-скриптом,
-- т.к. он чувствителен к регистру.
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'public' AND t.relname = 'user'
       AND c.contype = 'u'
       AND pg_get_constraintdef(c.oid) ILIKE '%(email)%';
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE public."user" DROP CONSTRAINT %I', cname);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS user_email_uniq
    ON public."user" (lower(email))
    WHERE email IS NOT NULL;

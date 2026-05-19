-- Таблица public."user" отсутствует в dump.sql, но на неё ссылается FK
-- в конце дампа (variant_interpretation_user_id_fkey).
-- Создаём таблицу ДО применения дампа, чтобы FK не упал.
--
-- На этом этапе тип public.user_role ещё не существует (он создаётся
-- внутри дампа), поэтому колонка role объявляется как text.
-- После применения дампа скрипт 20-user-table.sql приведёт тип к enum.

CREATE TABLE IF NOT EXISTS public."user" (
    user_id       serial PRIMARY KEY,
    username      varchar(100) NOT NULL UNIQUE,
    email         varchar(255) UNIQUE,
    password_hash varchar(255),
    first_name    varchar(100),
    last_name     varchar(100),
    role          text NOT NULL DEFAULT 'viewer',
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at    timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

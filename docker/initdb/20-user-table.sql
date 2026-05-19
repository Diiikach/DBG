-- Выполняется ПОСЛЕ 10-dump.sql.
-- К этому моменту:
--   • таблица public."user" уже существует (создана в 05-user-table.sql)
--   • тип public.user_role уже существует (создан внутри дампа)
--   • FK variant_interpretation_user_id_fkey уже добавлен дампом
--
-- Приводим колонку role с text к правильному enum-типу.
-- Используем USING для явного приведения значений.

ALTER TABLE public."user"
    ALTER COLUMN role TYPE public.user_role
        USING role::public.user_role;

-- Восстанавливаем DEFAULT с правильным типом (после смены типа DEFAULT сбрасывается).
ALTER TABLE public."user"
    ALTER COLUMN role SET DEFAULT 'viewer'::public.user_role;

-- Передаём владение таблицей нужному пользователю.
ALTER TABLE public."user" OWNER TO genadmin;

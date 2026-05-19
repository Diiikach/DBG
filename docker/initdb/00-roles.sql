-- Дамп dump.sql ссылается на роль "postgres" как владельца БД и ряда объектов,
-- но docker-образ создаёт только POSTGRES_USER (genadmin).
-- Создаём роль postgres до применения дампа.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'postgres') THEN
        CREATE ROLE postgres WITH LOGIN SUPERUSER;
    END IF;
END$$;

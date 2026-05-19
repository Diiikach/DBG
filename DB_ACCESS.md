# Доступ к БД `variant_db`

В проекте используется **единственная** БД — PostgreSQL 18 в Docker-контейнере
`variant_db`, описанном в [`docker-compose.yml`](docker-compose.yml:1).
Локальный PostgreSQL на хосте (Homebrew и т. п.) **не используется**; если он
запущен — его следует остановить, чтобы исключить путаницу с двумя базами.

## Параметры подключения

| Параметр | С хоста | Внутри docker-сети compose |
|---|---|---|
| Host | `localhost` | `db` |
| Port | `5433` (проброс из контейнера) | `5432` |
| Database | `variant_db` | `variant_db` |
| User | `genadmin` (SUPERUSER) | `genadmin` |
| Password | `genadmin` | `genadmin` |

DSN с хоста:

```
postgresql://genadmin:genadmin@localhost:5433/variant_db?sslmode=disable
```

DSN из API-контейнера (формируется в [`docker-compose.yml`](docker-compose.yml:59)):

```
postgresql://genadmin:genadmin@db:5432/variant_db?sslmode=disable
```

Переменные окружения см. в [`db.env`](db.env:1).

## Быстрый старт

```bash
# 1. Поднять стек (БД, API, фронт)
docker compose up -d

# 2. Подключение из консоли — два эквивалентных способа

# 2а. С хоста через проброшенный порт 5433
set -a && source ./db.env && set +a
psql

# 2б. Внутри контейнера (пароль не нужен)
docker compose exec db psql -U genadmin variant_db
```

## Содержимое

10 таблиц в схеме `public`:
`gene`, `phenotype`, `gene_phenotype`, `patient`, `patient_variant`, `sample`,
`variant`, `variant_annotation`, `variant_interpretation`, `user`.

Основной объём — таблица `patient_variant` (~70.9 млн строк, ~7.8 GB всего).

## Восстановление с нуля

Контейнер при первом запуске на пустом томе автоматически выполняет скрипты из
[`docker/initdb/`](docker/initdb/) и [`dump.sql`](dump.sql:1) (см.
volumes-секцию [`docker-compose.yml`](docker-compose.yml:24)). Чтобы полностью
пересоздать данные:

```bash
docker compose down -v        # удалит том db_data
docker compose up -d db       # БД зальётся заново из dump.sql (~30 мин)
docker compose logs -f db     # следить за прогрессом
```

# База геномных вариантов (Go + PostgreSQL)

Бэкенд и схема базы данных для управления геномными вариантами, пациентами, образцами, генами, заболеваниями, пользователями и клиническими интерпретациями.

## Стек
- Go 1.21+ (stdlib `net/http`, `encoding/json`)
- PostgreSQL 14+
- `database/sql` + драйвер PostgreSQL (рекомендуется: `pgx` через `github.com/jackc/pgx/v5/stdlib`)

## Структура проекта
Корень модуля — `src/`.
- `src/cmd/api/` точка входа HTTP
- `src/internal/models/` модели БД (DTO)
- `src/internal/` слои приложения (будут расширяться)
- `src/migrations/` DDL PostgreSQL (зеркало `db_init/`)
- `db_init/` SQL для инициализации Docker
- `web/` фронтенд Svelte
- `docs/` документация
- `tests/` тесты (заготовка)

Детали: `docs/PROJECT_STRUCTURE.md`.

## База данных
Схема PostgreSQL находится в `db_init/` и продублирована в `src/migrations/`.

Быстрый старт (Docker):
1. `make up`
2. API доступен на `localhost:8080`, БД на `localhost:5432`.

Детали схемы: `docs/DATABASE.md`.

## API
Сейчас реализовано:
- `GET /health` -> `{"status":"ok","db":"up|down"}`
- CRUD-минимум для пациентов и вариантов
- Поиск похожих мутаций по `rs_id`, `gene` или локусу
- Внешнее обогащение по публичным источникам (MyVariant.info)
- Экспорт в CSV/JSON
- Импорт VCF/CSV
- Импорт FASTQ и автогенерация SNP (bwa/samtools/bcftools)
- Авторизация врачей и личные кабинеты

Детали: `docs/API.md`.

Настройки сервера через env:
- `DGV_HTTP_ADDR` (по умолчанию `:8080`)
- `DGV_HTTP_READ_TIMEOUT` (по умолчанию `5s`)
- `DGV_HTTP_WRITE_TIMEOUT` (по умолчанию `10s`)
- `DGV_HTTP_IDLE_TIMEOUT` (по умолчанию `60s`)
- `DGV_DB_DSN` (PostgreSQL DSN, опционально)
- `DGV_STORAGE_DIR` (каталог хранения файлов, по умолчанию `/data`)
- `DGV_REFERENCE_FASTA` (референс‑геном FASTA, обязателен для FASTQ)
- `DGV_PIPELINE_ENABLED` (`true|false`, включает обработчик FASTQ)
- `DGV_TOOL_BWA` (путь к `bwa`, по умолчанию `bwa`)
- `DGV_TOOL_SAMTOOLS` (путь к `samtools`, по умолчанию `samtools`)
- `DGV_TOOL_BCFTOOLS` (путь к `bcftools`, по умолчанию `bcftools`)

Запуск сервера:
```
cd src
go run ./cmd/api
```

## Docker
Запуск всего стека:
```
make up
```

Перед запуском FASTQ-пайплайна положите референс‑геном в `data/reference.fa`.

Остановка:
```
make down
```

Логи:
```
make logs
```

## Frontend (Svelte)
Фронтенд находится в `web/`.

Dev‑сервер:
```
cd web
npm run dev
```

Сборка:
```
cd web
npm run build
```

Dev UI в Compose:
- `http://localhost:5173/`

Прокси для API в dev:
- Запросы к `/api/*` проксируются на `VITE_API_PROXY_TARGET`
- По умолчанию `http://localhost:8080`, в Compose — `http://api:8080`

# Структура проекта

Корень модуля: `src/` (Go module).

```
src/
  cmd/
    api/                # HTTP точка входа (net/http)
  internal/
    models/             # Модели БД (DTO)
    db/                 # Подключение БД
    httpapi/            # HTTP маршруты и обработчики
    config/             # Конфигурация (планируется)
    repository/         # Репозитории SQL (планируется)
    service/            # Бизнес-логика (планируется)
    transport/
      http/             # Роутинг и обработчики (планируется)
  migrations/           # DDL PostgreSQL (зеркало db_init/)
tests/                  # Тесты (заготовка)
db_init/                # Инициализация БД в Docker (Postgres)
docs/                   # Документация
web/                    # Svelte фронтенд
```

## Принципы слоёв
- `cmd/api`: старт HTTP сервера.
- `internal/transport/http`: маршрутизация, валидация, JSON encode/decode.
- `internal/service`: бизнес-логика (без SQL).
- `internal/repository`: SQL-запросы через `database/sql`.
- `internal/models`: модели, согласованные со схемой БД.

Сейчас пакеты минимальны; расширяйте их по мере роста API.

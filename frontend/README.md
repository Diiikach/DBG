# Frontend — геномная клиническая платформа

SPA на React 18 + TypeScript + Vite поверх REST API из [`../backend/README.md`](../backend/README.md).

## Стек

- **React 18 + TypeScript + Vite**
- **Mantine v7** — UI, формы, нотификации, модалки
- **TanStack Query v5** — серверное состояние, кэш, polling
- **React Router v6** — маршрутизация (browser history)
- **axios** — HTTP-клиент с Bearer-interceptor
- **zod** + **mantine-form-zod-resolver** — валидация форм
- **dayjs** — даты (локаль ru)

## Запуск

### Вариант 1. Локально (dev-сервер)

Требуется Node.js ≥ 18.

```bash
cd frontend
npm install
npm run dev          # http://localhost:5173
```

Бэкенд (см. [`../backend/README.md`](../backend/README.md)) должен быть запущен на `http://localhost:8080`. Адрес настраивается переменной окружения:

```bash
# .env.development (уже есть в репозитории)
VITE_API_BASE_URL=http://localhost:8080
```

### Вариант 2. Через docker-compose (db + api + web)

Из корня репозитория:

```bash
docker-compose up -d --build
```

После запуска:

- Web (nginx + статика):   <http://localhost:8082>
- API:                      <http://localhost:8081>
- PostgreSQL:               localhost:5433

`VITE_API_BASE_URL` встраивается на этапе сборки образа web. По умолчанию указывает на `http://localhost:8081` (см. `args.VITE_API_BASE_URL` сервиса web в [`../docker-compose.yml`](../docker-compose.yml:1)). Чтобы переопределить — экспортируйте переменную перед `docker-compose build web`:

```bash
VITE_API_BASE_URL=http://api.example.com docker-compose build web
WEB_PORT=9000 docker-compose up -d
```

## Скрипты

| Команда | Описание |
|---|---|
| `npm run dev` | dev-сервер с HMR |
| `npm run build` | production-сборка (`tsc -b && vite build`) |
| `npm run preview` | предпросмотр production-сборки |
| `npm run lint` | ESLint |

## Структура

```
src/
├── api/                # axios-клиент и тонкие обёртки над ручками
│   ├── client.ts
│   ├── auth.ts
│   ├── patients.ts
│   ├── samples.ts
│   └── variants.ts
├── auth/               # AuthContext, ProtectedRoute, useAuth
├── components/         # переиспользуемые UI-компоненты
│   ├── AppShell.tsx
│   ├── PatientForm.tsx
│   ├── SamplesTab.tsx
│   ├── SampleStatusBadge.tsx
│   ├── VariantsTab.tsx
│   ├── VariantDetailDrawer.tsx
│   └── CohortDrawer.tsx
├── pages/              # страницы (роуты)
│   ├── LoginPage.tsx
│   ├── RegisterPage.tsx
│   ├── PatientsListPage.tsx
│   ├── PatientNewPage.tsx
│   └── PatientDetailPage.tsx
├── types/api.ts        # TS-типы, зеркальные backend/internal/models/*.go
├── utils/format.ts     # форматирование дат/чисел (ru-RU)
├── router.tsx          # createBrowserRouter
└── main.tsx            # точка входа + провайдеры
```

## Аутентификация

- Токен и `expires_at` хранятся в `localStorage` (`auth_token`, `auth_expires_at`).
- При старте приложения, если токен ещё валиден, вызывается `GET /api/auth/me` для восстановления `user`.
- На `401` от любого запроса токен сбрасывается, происходит редирект на `/login`.
- Logout-эндпоинта на бэке нет — просто чистим `localStorage`.

## Маршруты

| Путь | Доступ | Содержимое |
|---|---|---|
| `/login` | публичный | вход |
| `/register` | публичный | регистрация |
| `/patients` | защищ. | список + поиск + пагинация |
| `/patients/new` | защищ. | форма создания |
| `/patients/:id` | защищ. | карточка с табами Информация / Образцы / Варианты |

## Известные ограничения бэкенда

См. раздел «Известные ограничения бэкенда» в [`../plans/frontend-plan.md`](../plans/frontend-plan.md). Кратко:

- Нет WebSocket/SSE для статуса sample → polling `GET /api/samples/{id}` раз в 3 секунды.
- `panel_name` отправляется, но в БД не сохраняется.
- Параметр `q` для `/api/patients/{id}/variants` зарезервирован, на фронте используем структурированные фильтры (`chrom`, `variant_type`, `filter`, `min_qual`, `zygosity`, `sort`).
- Лимит размера файла на upload контролируется бэкендовым `MAX_UPLOAD_MB` (по умолчанию 256 МБ); алфавит ридов `A/C/G/T/N`, ≤ 2000 нт/рид, ≤ 10000 ридов.

## CORS

Бэкенд по умолчанию отдаёт `Access-Control-Allow-Origin: *` ([`router.New`](../backend/internal/router/router.go:36)). В проде имеет смысл задать `CORS_ORIGINS=https://app.example.com` на бэке.

# План реализации фронтенда

> Цель: SPA-приложение для работы с пациентами, загрузки ридов и просмотра геномных вариантов поверх существующего REST API из [`backend/README.md`](../backend/README.md:1).
>
> Все ручки `/api/*` (кроме `register`/`login`) требуют `Authorization: Bearer <JWT>`.
> Базовый URL по умолчанию — `http://localhost:8080`.

---

## 1. Стек и базовая инфраструктура

### Рекомендуемый стек

- **React 18 + TypeScript** + **Vite** — быстрый старт, лучший DX.
- **React Router v6** — клиентский роутинг.
- **TanStack Query (React Query) v5** — серверное состояние, кэш, инвалидации, polling.
- **Axios** (или `fetch` + обёртка) — HTTP-клиент с интерцептором для JWT.
- **UI**: один из вариантов
  - **Mantine v7** — готовые `Table`, `Modal`, `Drawer`, `FileInput`, `Notifications` (рекомендуется).
  - либо **MUI v5** + `@mui/x-data-grid` (если важны мощные таблицы из коробки).
- **react-hook-form + zod** — формы и валидация.
- **dayjs** — работа с датами.
- ESLint + Prettier + tsconfig strict.

### Структура проекта

```
frontend/
├── src/
│   ├── api/
│   │   ├── client.ts           # axios instance + interceptors (Bearer, 401 → logout)
│   │   ├── auth.ts             # register, login, me
│   │   ├── patients.ts         # CRUD пациентов
│   │   ├── samples.ts          # upload, list, get (status polling)
│   │   └── variants.ts         # listByPatient, getDetails, cohort
│   ├── types/
│   │   └── api.ts              # TS-типы, зеркалят backend/internal/models/*.go
│   ├── auth/
│   │   ├── AuthContext.tsx     # token + user в контексте, persist в localStorage
│   │   ├── ProtectedRoute.tsx  # редирект на /login, если нет токена
│   │   └── useAuth.ts
│   ├── pages/
│   │   ├── LoginPage.tsx
│   │   ├── RegisterPage.tsx
│   │   ├── PatientsListPage.tsx
│   │   ├── PatientNewPage.tsx       # форма добавления
│   │   ├── PatientDetailPage.tsx    # карточка пациента + samples + variants
│   │   ├── SampleUploadPage.tsx     # загрузка ридов
│   │   ├── VariantsTablePage.tsx    # таблица вариантов пациента (вкладка PatientDetail)
│   │   └── VariantDetailDrawer.tsx  # деталка варианта + когорта
│   ├── components/
│   │   ├── AppShell.tsx        # layout: header + nav + main
│   │   ├── PatientForm.tsx
│   │   ├── SampleStatusBadge.tsx
│   │   ├── VariantsDataTable.tsx
│   │   └── CohortTable.tsx
│   ├── hooks/
│   │   ├── usePatients.ts
│   │   ├── useSamplePolling.ts
│   │   └── useVariants.ts
│   ├── router.tsx
│   ├── App.tsx
│   └── main.tsx
├── .env.development            # VITE_API_BASE_URL=http://localhost:8080
├── package.json
└── vite.config.ts
```

### HTTP-клиент

[`src/api/client.ts`](frontend/src/api/client.ts:1) — axios-инстанс:

- `baseURL = import.meta.env.VITE_API_BASE_URL`.
- Request interceptor: добавляет `Authorization: Bearer ${token}` из `localStorage` (ключ `auth_token`).
- Response interceptor: при `401` — чистит токен и делает редирект на `/login`.
- Унифицированная обработка ошибок: достаём `error.response.data.error` (формат backend).

---

## 2. Аутентификация и авторизация

### Маршруты

| Путь | Компонент | Доступ |
|---|---|---|
| `/login` | `LoginPage` | публичный |
| `/register` | `RegisterPage` | публичный |
| всё остальное | через `ProtectedRoute` | требует токена |

### Используемые эндпоинты

- `POST /api/auth/register` → `{ token, expires_at, user }` ([`auth_handler.Register`](../backend/internal/handlers/auth_handler.go:31)).
- `POST /api/auth/login` → `{ token, expires_at, user }` ([`auth_handler.Login`](../backend/internal/handlers/auth_handler.go:72)).
- `GET /api/auth/me` — восстановление сессии при перезагрузке страницы ([`auth_handler.Me`](../backend/internal/handlers/auth_handler.go:124)).

### Поведение

1. **LoginPage**: поля `username`, `password`. Submit → `POST /api/auth/login`.
   - На успехе: сохраняем `token` и `expires_at` в `localStorage`, кладём `user` в контекст, redirect на `/patients`.
   - На `401`: показываем «Неверный логин или пароль».
   - На `403` (`user is inactive`): «Учётная запись отключена».
2. **RegisterPage**: `username`, `password` (мин. 6 символов — валидация совпадает с бэкендом).
   - На `409`: «Пользователь уже существует».
3. **AuthContext** на старте приложения:
   - Если в `localStorage` есть токен, дергаем `GET /api/auth/me` → восстанавливаем `user`.
   - Если `expires_at` < now — сразу logout.
4. **Logout**: чистим `localStorage`, редирект на `/login` (бэкенд stateless, отдельной ручки нет).

### Валидация форм (zod)

```ts
const loginSchema = z.object({
  username: z.string().min(1, "Введите имя пользователя"),
  password: z.string().min(1, "Введите пароль"),
});

const registerSchema = z.object({
  username: z.string().min(1),
  password: z.string().min(6, "Минимум 6 символов"),
});
```

---

## 3. Пациенты

### Маршруты

| Путь | Компонент | Назначение |
|---|---|---|
| `/patients` | `PatientsListPage` | список + поиск |
| `/patients/new` | `PatientNewPage` | форма создания |
| `/patients/:id` | `PatientDetailPage` | карточка + табы Samples / Variants |

### Используемые эндпоинты

- `GET /api/patients?limit=&offset=&q=` → `{ items, total, limit, offset }` ([`patient_handler.List`](../backend/internal/handlers/patient_handler.go:108)).
- `POST /api/patients` → объект [`Patient`](../backend/internal/models/models.go:6) ([`patient_handler.Create`](../backend/internal/handlers/patient_handler.go:26)).
- `GET /api/patients/{id}` → [`Patient`](../backend/internal/models/models.go:6).
- `PATCH /api/patients/{id}` — частичное обновление через [`PatientUpdate`](../backend/internal/models/extras.go:6).
- `DELETE /api/patients/{id}` — `204 No Content`.

### Форма создания пациента

Поля (соответствуют [`PatientCreate`](../backend/internal/models/models.go:22)):

| Поле | Тип | Required | Валидация |
|---|---|---|---|
| `first_name` | text | ✅ | непустая строка |
| `last_name` | text | ✅ | непустая строка |
| `external_id` | text | ❌ | — |
| `date_of_birth` | date (YYYY-MM-DD) | ❌ | формат ISO |
| `sex` | select | ❌ | `male` / `female` / `other` |
| `phone_number` | tel | ❌ | — |
| `email` | email | ❌ | формат email |
| `address` | textarea | ❌ | — |
| `phenotype_description` | textarea | ❌ | — |

После успеха → redirect на `/patients/:id`.

### Список пациентов

- Поисковая строка → debounce 300ms → `GET /api/patients?q=...&limit=50&offset=0`.
- Пагинация (limit/offset, кнопки «‹/›» или Mantine `Pagination`).
- Колонки: `external_id`, ФИО, `date_of_birth`, `sex`, `email`, кнопка «Открыть».

---

## 4. Загрузка ридов (samples)

### Где разместить

Отдельный таб «Образцы» на `/patients/:id` или модалка «Загрузить риды» из карточки пациента.

### Используемые эндпоинты

- `POST /api/patients/{id}/samples` (`multipart/form-data`) → `202 { sample_id, patient_id, status:"processing" }` ([`sample_handler.Upload`](../backend/internal/handlers/sample_handler.go:46)).
- `GET /api/patients/{id}/samples` → `{ items, total }`.
- `GET /api/samples/{id}` → [`Sample`](../backend/internal/models/models.go:63) с `processing_status`.

### Поля формы (multipart)

| Поле | Тип | Default | Заметки |
|---|---|---|---|
| `sample_name` | text | автогенерация на бэке, если пусто | желательно заполнить |
| `sample_type` | select | `short-reads` | пока единственное реальное значение |
| `sequencing_type` | select | `OTHER` | значения enum БД |
| `panel_name` | text | — | принимается, но **в БД пока не сохраняется** (см. [`sample_handler`](../backend/internal/handlers/sample_handler.go:86)) |
| `file` | file | — | FASTA/FASTQ (`.fa`, `.fasta`, `.fq`, `.fastq`), опц. `.gz` |

Лимит размера = `MAX_UPLOAD_MB` (по умолчанию 256 МБ) — в форме показываем подсказку.

### Polling статуса

После успешного upload:

1. Записываем `sample_id` в локальное состояние / queryClient.
2. Запускаем polling `GET /api/samples/{id}` каждые 3 секунды (React Query `refetchInterval`):
   - пока `processing_status === "processing"` — крутим спиннер;
   - на `"completed"` — показываем тост «Анализ завершён», инвалидируем `variants` пациента;
   - на `"failed"` — красный тост с предложением загрузить заново.
3. Останавливаем polling по выходу с страницы (React Query сделает это сам через `enabled`).

### Ограничения, которые показываем юзеру

- Алфавит ридов: `A`, `C`, `G`, `T`, `N`.
- Длина одного рида ≤ 2000 нт, всего ≤ 10000 ридов (см. ограничения в [`backend/README.md`](../backend/README.md:261)).
- Бэкенд может вернуть `400` с `parse reads: ...` — выводим текст ошибки прямо в форму.

---

## 5. Таблица вариантов пациента

### Где разместить

Таб «Варианты» на `/patients/:id`, либо отдельный путь `/patients/:id/variants`.

### Используемые эндпоинты

- `GET /api/patients/{id}/variants?...` → `{ patient_id, items: PatientVariantRich[], total, limit, offset }` ([`variant_handler.ListVariants`](../backend/internal/handlers/variant_handler.go:123)).
- `GET /api/variants/{id}` → [`VariantDetails`](../backend/internal/models/extras.go:81) ([`variant_handler.GetVariantDetails`](../backend/internal/handlers/variant_handler.go:175)).
- `GET /api/variants/{id}/patients?limit=&offset=` → `{ items: Patient[], total, ... }` ([`variant_handler.CohortByVariant`](../backend/internal/handlers/variant_handler.go:196)).

### Колонки таблицы (все доступные)

Источник: [`PatientVariantRich`](../backend/internal/models/extras.go:92) = [`PatientVariant`](../backend/internal/models/models.go:47) + `gene_symbols[]` + `annotations[]` ([`VariantAnnotation`](../backend/internal/models/extras.go:19)).

#### Базовые (показываем по умолчанию)

| Колонка | Источник |
|---|---|
| Хромосома | `variant.chromosome` |
| Позиция | `variant.position` |
| Ref / Alt | `variant.reference` / `variant.alternate` |
| Тип | `variant.variant_type` (`SNV/INS/DEL/INDEL/MNV/CNV/SV/OTHER`) |
| rs ID | `variant.rs_id` |
| Genome Build | `variant.genome_build` |
| Гены | `gene_symbols.join(", ")` |
| Зиготность | `zygosity` |
| Quality | `quality` |
| DP | `read_depth` |
| AD ref/alt | `allele_depth_ref` / `allele_depth_alt` |
| GQ | `genotype_quality` |
| Filter | `filter_status` |
| Detected at | `detected_at` |

#### Аннотационные (опционально, по чекбоксам «Показать колонки»)

Из `annotations[0]` (если есть несколько — выбираем по `impact` или показываем worst):

| Колонка | Поле |
|---|---|
| Consequence | `consequence` |
| Impact | `impact` |
| HGVSc / HGVSp | `hgvsc` / `hgvsp` |
| Protein pos | `protein_position` |
| Amino acids | `amino_acids` |
| gnomAD AF / popmax | `gnomad_af` / `gnomad_af_popmax` |
| SIFT score / pred | `sift_score` / `sift_prediction` |
| PolyPhen score / pred | `polyphen_score` / `polyphen_prediction` |
| CADD | `cadd_score` |
| REVEL | `revel_score` |
| ClinVar significance | `clinvar_significance` |
| ClinVar ID | `clinvar_id` |

### Фильтры и сортировка (server-side)

Все параметры пробрасываются в URL и в запрос:

- `chrom` — text input.
- `variant_type` — select (`SNV/INS/DEL/INDEL/MNV/CNV/SV/OTHER`).
- `filter` — text (например `PASS`).
- `min_qual` — number.
- `zygosity` — select (`heterozygous/homozygous/...`).
- `sort` — клик по заголовку колонки → собираем строку вида `chrom,position,-quality` (см. [`variant_handler.ListVariants`](../backend/internal/handlers/variant_handler.go:139)).
- `limit` (по умолчанию 100, max 1000) + `offset`.

> Поиск по тексту (`q`) на бэкенде пока **не реализован** — соответствующее поле прячем или делаем клиентский фильтр по уже подгруженной странице.

### Деталка варианта (drawer/modal)

Открывается по клику на строку. Запрос `GET /api/variants/{id}`. Показываем:

- Заголовок: `chr:pos ref>alt` (`rs_id` если есть).
- Секция **Annotations**: таблица всех `VariantAnnotation` (consequence, impact, HGVS, scores, ClinVar).
- Секция **Genes**: список из `genes[]` с символом, описанием, координатами.
- Секция **Phenotypes**: список из `phenotypes[]` с OMIM/Orpha кодами.
- Секция **Interpretations** (если есть): ACMG-классификация.
- Секция **Когорта**: «Этот вариант есть у N пациентов» (`patient_count`), кнопка «Показать пациентов».

### Когорта по варианту

По кнопке открываем суб-drawer / новую вкладку, грузим `GET /api/variants/{id}/patients` с пагинацией. Колонки: `external_id`, ФИО, `date_of_birth`, `sex`, ссылка «Открыть карточку» → `/patients/:id`.

---

## 6. Сквозные требования

### Состояние и кэширование

- React Query keys:
  - `["patients", { q, limit, offset }]`
  - `["patient", id]`
  - `["samples", patientId]`
  - `["sample", sampleId]` (с `refetchInterval` пока статус processing)
  - `["variants", patientId, filters]`
  - `["variant", variantId]`
  - `["cohort", variantId, { limit, offset }]`
- Инвалидации:
  - после `POST /patients` → инвалидируем `["patients"]`.
  - после успешного `sample` → инвалидируем `["samples", patientId]` и `["variants", patientId]`.

### Обработка ошибок

- Все ответы от бэка с ошибкой: `{ error: "...", request_id?: "..." }` (см. [`writeError`](../backend/internal/handlers/common.go:1)).
- Глобальный обработчик в axios interceptor + Mantine `Notifications`/`react-hot-toast`.
- На `401` — auto-logout.

### Локализация

UI-тексты на русском, формат дат через `dayjs.locale('ru')`. Числа — через `Intl.NumberFormat`.

### Доступность и UX

- Все формы — labelled inputs, ошибки видны по сабмиту.
- Кнопки с «опасными» действиями (delete patient) — confirm-модалка.
- Лоадеры/скелетоны для таблиц.

### Тесты (опционально, но желательно)

- **Vitest + React Testing Library** — формы (auth, patient).
- **MSW** — мокаем backend в тестах.

---

## 7. Чеклист последовательности работ

1. Скаффолд проекта: `npm create vite@latest frontend -- --template react-ts`, добавить ESLint/Prettier.
2. Базовая инфраструктура: axios client, AuthContext, ProtectedRoute, AppShell, react-router setup.
3. Страницы `LoginPage` + `RegisterPage`, интеграция с `/api/auth/*`.
4. `PatientsListPage` + `PatientNewPage` + `PatientDetailPage` (заглушки табов).
5. Таб «Образцы»: загрузка multipart, список, polling статуса.
6. Таб «Варианты»: таблица с фильтрами и сортировкой, выбор колонок.
7. Drawer деталки варианта + список когорты.
8. Полировка UX: тосты, скелетоны, обработка `401`/сетевых ошибок.
9. README в `frontend/` с инструкциями по запуску (`VITE_API_BASE_URL`, `npm run dev`).
10. (Опц.) Тесты ключевых сценариев на MSW.

---

## 8. Известные ограничения бэкенда (учесть на фронте)

| # | Ограничение | Как обойти на фронте |
|---|---|---|
| 1 | Нет WebSocket/SSE для статуса sample | polling `GET /api/samples/{id}` 1 раз в 3 секунды |
| 2 | Нет logout-эндпоинта (JWT stateless) | просто чистим `localStorage` |
| 3 | `panel_name` не сохраняется в БД ([`sample_handler`](../backend/internal/handlers/sample_handler.go:86)) | поле в форме оставляем, но в карточке sample не показываем |
| 4 | Нет скачивания исходного VCF | пока скрываем кнопку «Download VCF»; при необходимости попросить добавить `GET /api/samples/{id}/vcf` |
| 5 | `q` в `/patients/{id}/variants` зарезервирован, но не работает | используем только структурированные фильтры (`chrom`, `variant_type`, ...) |
| 6 | CORS по умолчанию `*` ([`router.New`](../backend/internal/router/router.go:36)) | в проде явно прописать `CORS_ORIGINS=https://app.example.com` |

---

## 9. Точки контакта с документацией бэкенда

- Список и формат всех ручек: [`backend/README.md`](../backend/README.md:77).
- DTO/модели: [`backend/internal/models/models.go`](../backend/internal/models/models.go:1) и [`backend/internal/models/extras.go`](../backend/internal/models/extras.go:1) — TS-типы делаем зеркальными.
- Сборка маршрутов: [`backend/internal/router/router.go`](../backend/internal/router/router.go:36).
- Ограничения парсинга ридов: [`backend/README.md`](../backend/README.md:261).
- Доступы к БД (для админских скриптов): [`DB_ACCESS.md`](../DB_ACCESS.md:1).

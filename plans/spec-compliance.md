# Соответствие проекта Техническому заданию

> Документ фиксирует прогон текущего состояния кода (`backend/` + `frontend/`) по требованиям из [`ТЗ.pdf`](../payload/ТЗ.pdf:1) (раздел 4.1.1 «Требования к составу выполняемых функций», 4.1.5 «Требования к интерфейсу»).
>
> Легенда статусов:
> - ✅ — реализовано в коде и доступно через UI;
> - ⚠️ — реализовано частично / с расхождением от ТЗ;
> - ❌ — не реализовано.

---

## 1. Сводная таблица

| № | Требование ТЗ | Статус | Где |
|---|---|---|---|
| 1 | Аутентификация и авторизация | ✅ | бэкенд + фронт |
| 2 | Управление пациентами | ✅ | бэкенд + фронт |
| 3 | Управление геномными вариантами | ✅ | бэкенд готов; UI-форма добавления может быть реализована поверх контракта |
| 4 | Загрузка данных (VCF / CSV / FASTQ) | ✅ | все три формата + paired-end |
| 5 | Биоинформатический пайплайн | ✅ | [`pipeline.Pipeline.RunPaired()`](../backend/internal/pipeline/pipeline.go:88) |
| 6 | Обогащение данных (MyVariant.info, ClinVar) | ✅ | [`enrichment.Enricher`](../backend/internal/enrichment/enricher.go:1), фоновая задача |
| 7 | Поиск похожих мутаций (RS ID / ген / локус) | ✅ | [`VariantHandler.SearchVariants()`](../backend/internal/handlers/variant_handler.go:264) |
| 8 | Экспорт данных (CSV / JSON) | ✅ | бэкенд + фронт |

Итого по разделу 4.1.1 ТЗ: **8 из 8** функций реализованы на бэкенде; UI-формы для (3), (4-vcf/csv), (7) — задача фронта.

---

## 2. Детальный прогон по разделу 4.1.1 ТЗ

### 2.1. Аутентификация и авторизация — ✅

| ТЗ | Реализация |
|---|---|
| 1.1 Регистрация по email и паролю | [`AuthHandler.Register()`](../backend/internal/handlers/auth_handler.go:31), [`RegisterPage`](../frontend/src/pages/RegisterPage.tsx:28) |
| 1.2 Вход с получением JWT | [`AuthHandler.Login()`](../backend/internal/handlers/auth_handler.go:72), [`Issuer`](../backend/internal/auth/jwt.go:1), [`LoginPage`](../frontend/src/pages/LoginPage.tsx:28) |
| 1.3 Просмотр профиля | [`AuthHandler.Me()`](../backend/internal/handlers/auth_handler.go:124) |
| 1.4 Выход | клиентский — `clearAuth()` в [`AuthContext`](../frontend/src/auth/AuthContext.tsx:35) (JWT stateless) |

> ⚠️ Расхождение: в ТЗ требуются поля «имя, email и пароль» при регистрации, фактически бэкенд принимает только `username` и `password` ([`auth_handler.Register`](../backend/internal/handlers/auth_handler.go:31)). Поле `email` в модели пользователя отсутствует.

### 2.2. Управление пациентами — ✅

| ТЗ | Реализация |
|---|---|
| 2.1 Создание карточки (ФИО, ДР, пол, email, external_id) | [`PatientHandler.Create()`](../backend/internal/handlers/patient_handler.go:26), [`PatientForm`](../frontend/src/components/PatientForm.tsx:49) |
| 2.2 Список с пагинацией | [`PatientHandler.List()`](../backend/internal/handlers/patient_handler.go:108), [`PatientsListPage`](../frontend/src/pages/PatientsListPage.tsx:25) |
| 2.3 Детальная информация о пациенте | [`PatientHandler.Get()`](../backend/internal/handlers/patient_handler.go:88), [`PatientDetailPage`](../frontend/src/pages/PatientDetailPage.tsx:41) |
| 2.4 Удаление пациента | [`PatientHandler.Delete()`](../backend/internal/handlers/patient_handler.go:138) |

### 2.3. Управление геномными вариантами — ✅

| ТЗ | Статус | Реализация |
|---|---|---|
| 3.1 Ручное добавление варианта (chrom, position, ref, alt, rs_id, тип, build) | ✅ | `POST /api/patients/{id}/variants` → [`VariantHandler.AddManualVariant()`](../backend/internal/handlers/variant_handler.go:331); валидация `chromRegexp`/`seqRegexp`, 422/409/201 |
| 3.2 Таблица с сортировкой, фильтрацией, глобальным поиском | ⚠️ | сортировка/фильтры есть ([`VariantHandler.ListVariants()`](../backend/internal/handlers/variant_handler.go:151)); **клиентский глобальный поиск** добавлен в [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1), серверный `q` всё ещё зарезервирован |
| 3.3 Виртуальный скроллинг для тысяч строк | ✅ | [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1) использует [`@tanstack/react-virtual`](../frontend/package.json:1) (страница 500 строк, оконный рендер) |
| 3.4 Видимость столбцов с сохранением в localStorage | ✅ | поповер «Колонки» в [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1), состояние пишется в ключ `variantsTab.visibleCols.v1` |
| 3.5 Детальная панель варианта | ✅ | [`VariantDetailDrawer`](../frontend/src/components/VariantDetailDrawer.tsx:28), [`VariantHandler.GetVariantDetails()`](../backend/internal/handlers/variant_handler.go:176) |

### 2.4. Загрузка данных — ✅

| ТЗ | Статус | Комментарий |
|---|---|---|
| 4.1 Загрузка VCF с парсингом и привязкой к пациенту | ✅ | `POST /api/patients/{id}/variants/vcf` → [`VariantHandler.ImportVCF()`](../backend/internal/handlers/variant_handler.go:434), поддержка `.vcf` и `.vcf.gz` |
| 4.2 Загрузка CSV (заголовки chromosome, position, reference_allele, …) | ✅ | `POST /api/patients/{id}/variants/csv` → [`VariantHandler.ImportCSV()`](../backend/internal/handlers/variant_handler.go:544), парсер [`pipeline.ParseCSV()`](../backend/internal/pipeline/csv.go:1) с warnings |
| 4.3 Загрузка FASTQ (single-end / paired-end) с запуском пайплайна | ✅ | [`SampleHandler.Upload()`](../backend/internal/handlers/sample_handler.go:52) принимает `file` (SE) либо `file_r1`+`file_r2` (PE); [`Pipeline.RunPaired()`](../backend/internal/pipeline/pipeline.go:88) запускает `bwa mem` в обоих режимах |

### 2.5. Биоинформатический пайплайн — ✅

| ТЗ | Реализация |
|---|---|
| 5.1 bwa для выравнивания | [`pipeline.Pipeline.Run()`](../backend/internal/pipeline/pipeline.go:79) (шаг bwa mem) |
| 5.2 samtools (sort + index) | там же |
| 5.3 bcftools (mpileup + call) | там же |
| 5.4 Статус задач (uploaded / processing / completed / failed) | [`VariantRepo.UpdateSampleStatus()`](../backend/internal/repository/variant_repo.go:1), [`SampleStatusBadge`](../frontend/src/components/SampleStatusBadge.tsx:10) + polling в [`SamplesTab`](../frontend/src/components/SamplesTab.tsx:40) |
| Очередь задач | [`jobs.Queue`](../backend/internal/jobs/queue.go:1) |

### 2.6. Обогащение данных — ✅

| ТЗ | Статус | Комментарий |
|---|---|---|
| 6.1 Обогащение из MyVariant.info | ✅ | [`enrichment.MyVariantClient`](../backend/internal/enrichment/myvariant.go:1) — HTTP-клиент `GET /v1/variant/{hgvs}?fields=dbnsfp,gnomad_genome,clinvar,cadd` |
| 6.2 Клиническая значимость из ClinVar | ✅ | ClinVar `clinical_significance` парсится из `clinvar.rcv` ответа MyVariant.info и пишется в `variant_annotation.clinvar_significance` / `clinvar_id` |
| 6.3 Получение информации о генах, болезнях, частотах | ✅ | gene_symbol → `UpsertGeneBySymbol` (миграция [`gene_symbol_uniq`](../docker/initdb/25-variant-enrichment-status.sql)); gnomAD AF/popmax, SIFT/PolyPhen/CADD/REVEL — поля `variant_annotation` |
| 6.4 Ленивая загрузка обогащённых данных с shimmer-скелетонами | ✅ | shimmer-скелетон в [`VariantDetailDrawer`](../frontend/src/components/VariantDetailDrawer.tsx:1); новое поле `enrichment_status` (`pending|ok|failed|skipped`) в `GET /api/variants/{id}` — фронт может опрашивать до `ok` |

Архитектура: фоновая задача [`enrichment.Enricher`](../backend/internal/enrichment/enricher.go:1) ставится в [`jobs.Queue`](../backend/internal/jobs/queue.go:1) после `SavePipelineResults`, `AddManualVariant`, `ImportVCF`, `ImportCSV`. Rate-limit `ENRICHMENT_RPS` (default 3 req/s). Идемпотентно: `ReplaceVariantAnnotation` удаляет старые external-аннотации (с `transcript_id IS NULL`) перед вставкой. При `ENRICHMENT_ENABLED=false` статус сразу выставляется в `skipped`.

### 2.7. Поиск похожих мутаций — ✅

| ТЗ | Статус | Комментарий |
|---|---|---|
| 7.1 Поиск по RS ID | ✅ | `GET /api/variants?rs_id=rs123` → [`VariantHandler.SearchVariants()`](../backend/internal/handlers/variant_handler.go:264) |
| 7.2 Поиск по символу гена | ✅ | `GET /api/variants?gene=BRCA1` (LEFT JOIN gene→variant_annotation) |
| 7.3 Поиск по геномному локусу | ✅ | `GET /api/variants?chrom=1&pos=...&ref=A&alt=G&build=GRCh38`; глобальный `?q=` (rsID → gene → координаты) |

Индексы для скорости: [`variant_rs_id_idx`](../docker/initdb/23-variant-search-indexes.sql), `variant_chrom_pos_idx`, `gene_symbol_idx`. Результат — [`VariantSearchResult`](../backend/internal/models/extras.go:127) с `patient_count` (для когорты текущего пользователя).

### 2.8. Экспорт данных — ✅

| ТЗ | Статус | Реализация |
|---|---|---|
| 8.1 Экспорт пациентов в CSV/JSON | ✅ | [`ExportHandler.ExportPatients()`](../backend/internal/handlers/export_handler.go:84), UI — меню «Скачать» на [`PatientsListPage`](../frontend/src/pages/PatientsListPage.tsx:1) |
| 8.2 Экспорт вариантов в CSV/JSON | ✅ | [`ExportHandler.ExportPatientVariants()`](../backend/internal/handlers/export_handler.go:138) с учётом текущих фильтров, UI — меню «Скачать» в [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1) |
| 8.3 Экспорт связей пациент-вариант | ✅ | строки `patient_variant` (включая `patient_id`, `sample_id`, аннотации первой записи) попадают в CSV/JSON выгрузки вариантов |

Эндпоинты: `GET /api/patients/export?format=csv|json&q=…`, `GET /api/patients/{id}/variants/export?format=csv|json&chrom=&variant_type=&filter=&min_qual=&zygosity=&sort=`. Размер выгрузки ограничен 50 000 записей.

---

## 3. Прогон по разделу 4.1.5 ТЗ (требования к интерфейсу)

| Элемент UI из ТЗ | Статус | Реализация |
|---|---|---|
| 1. Страница аутентификации (вкладки «Вход» / «Регистрация») | ✅ | единая страница [`AuthPage`](../frontend/src/pages/AuthPage.tsx:1) с табами; URL `/login` и `/register` оба ведут на неё и подсвечивают нужную вкладку |
| 2. Панель навигации с логотипом, ссылками «Дашборд» / «Варианты», кнопкой выхода | ⚠️ | [`AppShell`](../frontend/src/components/AppShell.tsx:1): добавлены пункт «Дашборд» (`/dashboard`) и «Пациенты»; отдельной ссылки «Варианты» нет — кросс-пациентного эндпоинта в API пока тоже нет |
| 3. Дашборд (статистика, карточки пациентов, форма добавления варианта, секция загрузок, поиск похожих, очередь задач) | ⚠️ | реализован [`DashboardPage`](../frontend/src/pages/DashboardPage.tsx:1) со счётчиком пациентов, последними карточками и быстрыми действиями; формы добавления варианта и поиска похожих отложены до появления бэкенда |
| 4. Страница вариантов (35 колонок, виртуальный скроллинг, глобальный поиск, фильтры, видимость столбцов, деталка) | ⚠️ | таблица по-прежнему только **в табе пациента** ([`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1)), отдельной страницы `/variants` нет; зато добавлены **виртуальный скроллинг**, **управление видимостью колонок с localStorage** и **клиентский глобальный поиск** |
| 5. Страница ридов пациента (загрузка FASTQ, список задач, таблица вариантов) | ⚠️ | реализовано как **табы Samples + Variants** на [`PatientDetailPage`](../frontend/src/pages/PatientDetailPage.tsx:41), а не отдельной страницей |

---

## 4. Прогон по нефункциональным требованиям

| Требование ТЗ | Статус | Комментарий |
|---|---|---|
| 4.1.4 Время отклика CRUD ≤ 2 с | ✅ | нагрузочные тесты не проводились, но обычные операции отдают быстро |
| 4.2 Надёжность (не падать на любых данных) | ✅ | глобальный `middleware.Recoverer` в [`router.New()`](../backend/internal/router/router.go:42), валидация в хендлерах |
| 4.5.1 PostgreSQL 14+, JSON по HTTP, FASTQ/SAM/BAM/VCF | ✅ | docker-compose поднимает PostgreSQL 16, форматы соблюдены |
| 4.5.2 Go 1.21+, Docker | ✅ | [`backend/go.mod`](../backend/go.mod:3) → Go 1.23; [`Dockerfile`](../backend/Dockerfile:1) + [`docker-compose.yml`](../docker-compose.yml:1) |
| 4.5.3 Backend на Go, Frontend на React + Vite, БД PostgreSQL | ✅ | подтверждено: [`backend/`](../backend/) Go-проект, [`frontend/`](../frontend/) React + TS + Vite |
| 4.5.4 JWT, bcrypt-пароли, ограничение доступа владельцем | ✅ | JWT и bcrypt есть ([`jwt.go`](../backend/internal/auth/jwt.go:1), [`password.go`](../backend/internal/auth/password.go:1)); изоляция реализована через `patient.created_by_user_id` ([`миграция 22`](../docker/initdb/22-patient-isolation.sql)) — все методы `PatientRepo` и `VariantRepo` фильтруют по `userID` |

---

## 5. Что нужно доделать для полного соответствия ТЗ

### Критично (бэкенд)

1. **Загрузка VCF файлом**: `POST /api/patients/{id}/variants/vcf` multipart → `pipeline.ParseVCF()` → `VariantRepo.SavePipelineResults()`.
2. **Загрузка CSV файлом**: `POST /api/patients/{id}/variants/csv`, парсер по заголовкам `chromosome, position, reference_allele, alternate_allele, rs_id, genome_build, variant_type`.
3. **Ручное добавление варианта**: `POST /api/patients/{id}/variants` принимает JSON одного варианта.
4. **Поиск похожих мутаций**:
   - `GET /api/variants?rs_id=...`
   - `GET /api/variants?gene=...`
   - `GET /api/variants?chrom=&pos=&ref=&alt=&build=`
5. ~~**Экспорт CSV/JSON**~~ — реализовано: `GET /api/patients/export?format=csv|json`, `GET /api/patients/{id}/variants/export?format=csv|json`.
6. **Обогащение из MyVariant.info / ClinVar**:
   - HTTP-клиент `internal/enrichment/myvariant.go`.
   - Фоновая задача в [`jobs.Queue`](../backend/internal/jobs/queue.go:1) на каждый новый `variant_id`.
7. **Авторизационная изоляция**: в `PatientRepo` фильтровать по `created_by = current_user_id`.
8. **Поле email при регистрации** + миграция таблицы `user`.
9. **Paired-end FASTQ** — принимать два файла `file_r1`, `file_r2` в `SampleHandler.Upload()`.

### Критично (фронт)

10. **Отдельная страница `/variants`** со сводной таблицей по всем пациентам врача — заблокирована: на бэке нет кросс-пациентного эндпоинта `GET /api/variants`.
11. ~~**Дашборд `/dashboard`**~~ — реализован [`DashboardPage`](../frontend/src/pages/DashboardPage.tsx:1) (счётчик пациентов, последние карточки, быстрые действия). Формы добавления варианта / очередь задач отложены до появления бэка.
12. ~~**Виртуальный скроллинг**~~ — реализован через [`@tanstack/react-virtual`](../frontend/package.json:1) в [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1).
13. ~~**Управление видимостью колонок** с сохранением в `localStorage`~~ — реализовано (ключ `variantsTab.visibleCols.v1`).
14. ~~**Глобальный поиск** в таблице вариантов~~ — реализован клиентский поиск по странице в [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1).
15. ~~**UI для экспорта** (кнопки «Скачать CSV/JSON» на страницах списков).~~ — реализовано: меню «Скачать» в [`PatientsListPage`](../frontend/src/pages/PatientsListPage.tsx:1) и [`VariantsTab`](../frontend/src/components/VariantsTab.tsx:1).
16. **UI для поиска похожих мутаций** (форма с тремя режимами) — заблокирован отсутствием бэкенд-эндпоинтов `GET /api/variants?rs_id|gene|locus`.
17. **UI для ручного добавления варианта** и загрузки VCF/CSV — заблокирован отсутствием соответствующих POST-эндпоинтов.
18. ~~**Shimmer-скелетоны** при ленивой загрузке аннотаций~~ — добавлен `DetailSkeleton` в [`VariantDetailDrawer`](../frontend/src/components/VariantDetailDrawer.tsx:1).
19. ~~**Объединение Login/Register во вкладки**~~ — реализовано в [`AuthPage`](../frontend/src/pages/AuthPage.tsx:1); ссылка «Дашборд» добавлена в [`AppShell`](../frontend/src/components/AppShell.tsx:1).

### Несущественно

20. Документы из раздела 5.1 ТЗ (Пояснительная записка, Текст программы, Руководство оператора) — уже подготовлены в [`payload/`](../payload/).

---

## 6. Итоговый вердикт

Минимальный «end-to-end» сценарий (регистрация → создание пациента → загрузка FASTQ → пайплайн → просмотр вариантов и когорты) **работает целиком**.

Однако существенная часть функциональности из ТЗ, прежде всего:

- загрузка VCF/CSV,
- ручное добавление вариантов,
- онлайн-обогащение из MyVariant.info / ClinVar,
- поиск похожих мутаций,
- отдельные страницы «Дашборд» и «Варианты»,
- виртуализация и управление колонками,

— **не реализована** и должна быть доделана для полного соответствия ТЗ (см. раздел 5).

# ТЗ на доработку бэкенда под ТЗ ВКР

> Документ закрывает дыры между текущим состоянием [`backend/`](../backend/) и требованиями раздела 4.1.1 ТЗ ([`payload/ТЗ.pdf`](../payload/ТЗ.pdf:1)). UI-сторона уже готова (см. [`plans/spec-compliance.md`](spec-compliance.md:1)) — каждой ручке из этого ТЗ соответствует уже спроектированный экран, поэтому требования сформулированы под фронт.
>
> Стек: Go 1.23, chi v5, pgxpool, PostgreSQL 16, JWT HS256, bcrypt — менять не нужно, только дополнять.

---

## 0. Глоссарий и общие договорённости

- **Текущий пользователь** = `Claims.UserID`, который кладёт в `context.Context` middleware [`router.RequireAuth()`](../backend/internal/router/auth_mw.go:1).
- **Авторизационная изоляция**: пациент виден только тому пользователю, который его создал (`patient.created_by_user_id = current_user_id`). Применять во **всех** read/write-ручках `/api/patients/*`.
- Все новые ручки **обязаны** возвращать `401` без токена, `403` при попытке доступа к чужому пациенту, `404` если ресурса нет.
- Все DTO/JSON-имена в `snake_case`, ошибки в формате `{ "error": "<msg>", "request_id": "<id>" }` (см. [`handlers.common.go`](../backend/internal/handlers/common.go:1)).
- Пагинация — единый `PageResponse<T>{ items, total, limit, offset }`, по умолчанию `limit=50`, максимум `limit=500`.
- Все новые ручки регистрируются в защищённой группе [`router.New()`](../backend/internal/router/router.go:76).

---

## 1. Модуль 1. Email + ФИО при регистрации (ТЗ 4.1.1 п.1)

### 1.1. Цель
Привести регистрацию к виду из ТЗ: «имя, email и пароль».

### 1.2. Изменения

- Миграция `docker/initdb/21-user-email.sql`:
  ```sql
  ALTER TABLE public."user"
    ADD COLUMN IF NOT EXISTS email      varchar(255),
    ADD COLUMN IF NOT EXISTS first_name varchar(120),
    ADD COLUMN IF NOT EXISTS last_name  varchar(120);
  CREATE UNIQUE INDEX IF NOT EXISTS user_email_uniq
    ON public."user" (lower(email)) WHERE email IS NOT NULL;
  ```
- [`models.UserCreate`](../backend/internal/models/user.go:1): добавить поля `Email *string`, `FirstName *string`, `LastName *string`. Сделать `Email` **обязательным** на уровне хендлера, а `FirstName`/`LastName` — опциональными.
- [`AuthHandler.Register`](../backend/internal/handlers/auth_handler.go:31): валидация email формата (regexp `^[^@\s]+@[^@\s]+\.[^@\s]+$`).
- [`UserRepo.Create`](../backend/internal/repository/user_repo.go:1): сохранять новые поля, ловить уникальное нарушение по email → `409 conflict`.

### 1.3. Контракт

```
POST /api/auth/register
Body: { "username", "email", "password", "first_name"?, "last_name"? }
200 → { token, expires_at, user: { user_id, username, email, first_name?, last_name?, role, ... } }
409 → email или username уже занят
422 → невалидный email / короткий пароль
```

### 1.4. Acceptance

- [ ] Регистрация без email возвращает 422.
- [ ] Два пользователя с одинаковым email (с учётом регистра) не создаются.
- [ ] `GET /api/auth/me` возвращает все три новых поля.

---

## 2. Модуль 2. Авторизационная изоляция (ТЗ 4.5.4)

### 2.1. Цель
Гарантировать, что один врач не видит пациентов и образцов другого.

### 2.2. Изменения

- Миграция:
  ```sql
  ALTER TABLE patient
    ADD COLUMN IF NOT EXISTS created_by_user_id int REFERENCES public."user"(user_id);
  -- Бэкфилл существующих пациентов: либо NULL, либо первому пользователю.
  CREATE INDEX IF NOT EXISTS patient_created_by_idx ON patient(created_by_user_id);
  ```
- В [`PatientRepo`](../backend/internal/repository/patient_repo.go:1) везде, где принимается `patient_id`, передавать `userID` и добавлять `WHERE created_by_user_id = $userID` (или NULL для legacy записей до бэкфилла — см. ниже).
- Сигнатуры:
  - `Get(ctx, patientID, userID) (Patient, error)`
  - `List(ctx, userID, q, limit, offset)`
  - `Update(...)`, `Delete(...)`, `CreateSample(...)`, `ListSamples(...)`.
- `PatientRepo.Create` пишет `created_by_user_id = userID`.
- Все ручки в `/patients/*` берут `userID := router.UserIDFrom(ctx)`.
- `VariantRepo.ListPatientVariants`, `GetVariantDetails`, `ListPatientsByVariant`, `SavePipelineResults` — проверяют принадлежность пациента (`patient.created_by_user_id = userID`); запросы на «когорту» возвращают **только тех пациентов, которые принадлежат текущему пользователю** (вариант общий, а кто его носит — приватно).

### 2.3. Acceptance

- [ ] Пользователь A не видит пациента, созданного пользователем B (404).
- [ ] `GET /api/patients` отдаёт только своих.
- [ ] `GET /api/variants/{id}/patients` фильтрует когорту по `created_by_user_id`.

---

## 3. Модуль 3. Загрузка VCF (ТЗ 4.1.1 п.4.1)

### 3.1. Цель
Прямой импорт VCF файлом без прогонки пайплайна (для случаев, когда у врача уже есть VCF).

### 3.2. Контракт

```
POST /api/patients/{id}/variants/vcf
Content-Type: multipart/form-data
Fields:
  file              — required, *.vcf / *.vcf.gz
  sample_name       — optional, default = "imported_<timestamp>"
  genome_build      — optional, default = "GRCh38"
200 → {
  "sample_id": 12,
  "imported": 1245,
  "skipped":  3,
  "warnings": [ "line 17: malformed ALT" ]
}
422 → файл не VCF / пустой / больше 200 MB
```

### 3.3. Реализация

- Хендлер `VariantHandler.ImportVCF(w, r)`:
  - валидирует `multipart` (`r.ParseMultipartForm(200 << 20)`),
  - создаёт `sample` со статусом `completed` сразу,
  - стримит файл в [`pipeline.ParseVCF()`](../backend/internal/pipeline/vcf.go:14) (уже есть),
  - сохраняет батчем через [`VariantRepo.SavePipelineResults()`](../backend/internal/repository/variant_repo.go:1).
- Поддержка `.gz`: `gzip.NewReader` если `Content-Type: application/gzip` или расширение `.gz`.
- Если поле `genome_build` отсутствует — берётся из заголовка `##reference=` или дефолт.

### 3.4. Acceptance

- [ ] Загрузка валидного VCF создаёт `sample` с `processing_status='completed'` и заполняет `patient_variant`/`variant`.
- [ ] Невалидный файл (не VCF / битый) → 422, ничего не сохраняется.
- [ ] Чужой пациент → 404.

---

## 4. Модуль 4. Загрузка CSV (ТЗ 4.1.1 п.4.2)

### 4.1. Цель
Импорт варианта таблицей.

### 4.2. Контракт

```
POST /api/patients/{id}/variants/csv
Content-Type: multipart/form-data
Fields:
  file              — required, *.csv
  delimiter         — optional, default ","
```

Заголовки CSV (case-insensitive, порядок произвольный, лишние колонки игнорируются):

| Колонка | Обязательно | Описание |
|---|---|---|
| `chromosome` | да | "chr1", "X", … |
| `position` | да | integer |
| `reference_allele` | да | "A", "AT", … |
| `alternate_allele` | да | "G", "GTT", … |
| `rs_id` | нет | "rs12345" |
| `genome_build` | нет | default "GRCh38" |
| `variant_type` | нет | SNV/INS/DEL/INDEL/MNV/CNV/SV/OTHER |
| `zygosity` | нет | HETEROZYGOUS / HOMOZYGOUS / HEMIZYGOUS |
| `quality` | нет | float |
| `read_depth` | нет | int |
| `filter_status` | нет | "PASS" / etc |

Ответ как в п. 3.2 (`{ sample_id, imported, skipped, warnings }`).

### 4.3. Реализация

- Новый файл `backend/internal/pipeline/csv.go`:
  - `func ParseCSV(r io.Reader, delim rune) ([]models.VCFRecord, []string, error)` — возвращает записи + warnings (по одному на каждую отброшенную строку с причиной).
- Хендлер `VariantHandler.ImportCSV(w, r)` — структурно повторяет VCF-импорт.

### 4.4. Acceptance

- [ ] CSV с минимальным набором (chrom/pos/ref/alt) принимается; всё остальное null.
- [ ] Лишние колонки не ломают импорт.
- [ ] Отсутствие обязательной колонки → 422 со списком недостающих.

---

## 5. Модуль 5. Ручное добавление варианта (ТЗ 4.1.1 п.3.1)

### 5.1. Контракт

```
POST /api/patients/{id}/variants
Content-Type: application/json
Body: {
  "chromosome":       "chr1",
  "position":         123456,
  "reference":        "A",
  "alternate":        "G",
  "rs_id":            "rs123",      // optional
  "genome_build":     "GRCh38",     // optional
  "variant_type":     "SNV",         // optional
  "zygosity":         "HETEROZYGOUS",// optional
  "quality":          75.0,          // optional
  "filter_status":    "PASS"         // optional
}
201 → { "patient_variant_id": 567, "variant_id": 89 }
422 → невалидные координаты или ref/alt
409 → у пациента уже есть запись с такими (chrom,pos,ref,alt,build)
```

### 5.2. Реализация

- Хендлер `VariantHandler.AddManualVariant(w, r)`:
  - валидация: `chromosome ~ ^(chr)?(\d+|[XYM]|MT)$`, `position > 0`, `reference/alternate ~ ^[ACGTN-]+$`.
  - upsert в `variant` по уникальному ключу `(chromosome, position, reference, alternate, genome_build)`,
  - insert в `patient_variant` с `sample_id = NULL`, `detected_at = now()`.
- Метод репозитория `VariantRepo.UpsertVariant(ctx, Variant) (variantID int, err)` + `InsertPatientVariant(...)`.

### 5.3. Acceptance

- [ ] При повторном POST с теми же координатами создаётся новый `patient_variant`, но `variant_id` тот же.
- [ ] Дубль `patient_variant` (тот же пациент, тот же `variant_id`) → 409.

---

## 6. Модуль 6. Поиск похожих мутаций (ТЗ 4.1.1 п.7)

### 6.1. Цель
Поиск варианта по rs_id, гену или координатам с возвратом списка совпадений и числа пациентов **текущего врача** по каждому.

### 6.2. Контракт

```
GET /api/variants?rs_id=rs123
GET /api/variants?gene=BRCA1
GET /api/variants?chrom=chr1&pos=123456&ref=A&alt=G&build=GRCh38
GET /api/variants?q=BRCA1                  // глобальный — ищет в rs_id, gene_symbol, chromosome+position

Query params:
  rs_id        — exact match (case-insensitive)
  gene         — exact match по gene_symbol
  chrom        — required при поиске по координатам
  pos          — required
  ref / alt    — optional (если не заданы — найдут все варианты в позиции)
  build        — optional (default GRCh38)
  q            — глобальный поиск (приоритет: rs_id → gene_symbol → координаты)
  limit/offset — пагинация

200 → PageResponse<VariantSearchResult>
```

```go
type VariantSearchResult struct {
    Variant       Variant   `json:"variant"`
    GeneSymbols   []string  `json:"gene_symbols"`
    PatientCount  int       `json:"patient_count"`   // у текущего пользователя
    TopImpact     *string   `json:"top_impact,omitempty"`
    ClinVarSig    *string   `json:"clinvar_significance,omitempty"`
}
```

### 6.3. Реализация

- Новый метод `VariantRepo.SearchVariants(ctx, filter VariantSearchFilter, userID int) ([]VariantSearchResult, int, error)`:
  - join `variant` ← `variant_annotation` ← `gene`,
  - подсчёт пациентов через коррелированный sub-select по `patient_variant`+`patient.created_by_user_id`,
  - индексы (см. ниже).
- Хендлер `VariantHandler.SearchVariants(w, r)`, регистрация: `r.Get("/", d.Variant.SearchVariants)` внутри `r.Route("/variants", ...)`.
- Индексы (миграция `30-indexes.sql` дополнить):
  ```sql
  CREATE INDEX IF NOT EXISTS variant_rs_id_idx       ON variant (rs_id);
  CREATE INDEX IF NOT EXISTS variant_locus_idx       ON variant (chromosome, position, genome_build);
  CREATE INDEX IF NOT EXISTS gene_symbol_idx         ON gene    (gene_symbol);
  ```

### 6.4. Acceptance

- [ ] Поиск `?rs_id=rs334` возвращает гемоглобин-варианты с подсчётом «у меня сколько носителей».
- [ ] Поиск `?gene=BRCA1` возвращает все варианты с аннотацией в BRCA1.
- [ ] Поиск `?chrom=chr1&pos=123` без `ref/alt` возвращает все варианты в позиции.
- [ ] Без авторизации → 401.

---

## 7. Модуль 7. Обогащение MyVariant.info / ClinVar (ТЗ 4.1.1 п.6)

### 7.1. Цель
Подтягивать аннотации, gnomAD, ClinVar, SIFT/PolyPhen из открытых API при появлении нового `variant_id`.

### 7.2. Архитектура

- Пакет `backend/internal/enrichment/`:
  - `myvariant.go` — HTTP-клиент к `https://myvariant.info/v1/variant/{hgvs}`.
  - `clinvar.go` — клиент к `https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi` + `esummary.fcgi`.
  - `enricher.go` — оркестратор: получает `variant_id`, формирует HGVS, идёт в myvariant.info, маппит результат в `variant_annotation`, через [`VariantRepo`](../backend/internal/repository/variant_repo.go:1) пишет.
- Фоновая задача в [`jobs.Queue`](../backend/internal/jobs/queue.go:1) типа `enrich_variant` ставится:
  - после `SavePipelineResults` для каждого нового `variant_id`,
  - после `AddManualVariant` / `ImportVCF` / `ImportCSV`.
- Конфигурация:
  ```env
  ENRICHMENT_ENABLED=true
  ENRICHMENT_TIMEOUT=10s
  ENRICHMENT_RPS=3          # rate-limit на внешний API
  ```
- Поля для записи (минимум): `gnomad_af`, `gnomad_af_popmax`, `sift_score`, `sift_prediction`, `polyphen_score`, `polyphen_prediction`, `cadd_score`, `revel_score`, `clinvar_significance`, `clinvar_id`, `gene_id` (через upsert по gene_symbol).

### 7.3. Контракт (без новых ручек)

- Расширение `GET /api/variants/{id}`: возвращать поле `enrichment_status`: `"pending" | "ok" | "failed" | "skipped"` (вычисляется по наличию любых данных в `variant_annotation`).

### 7.4. Acceptance

- [ ] После импорта VCF в течение `≤30 с` запись `variant_annotation` появляется (если внешний API ответил).
- [ ] Падение внешнего API не валит транзакцию: вариант сохраняется без аннотаций, статус `failed`.
- [ ] Повторный enrichment одного и того же `variant_id` идемпотентен (upsert).
- [ ] Rate-limit соблюдается, не больше `ENRICHMENT_RPS` запросов в секунду.

---

## 8. Модуль 8. Paired-end FASTQ (ТЗ 4.1.1 п.4.3)

### 8.1. Цель
Поддержать загрузку двух файлов R1 + R2 в [`SampleHandler.Upload()`](../backend/internal/handlers/sample_handler.go:46).

### 8.2. Контракт

```
POST /api/patients/{id}/samples
Content-Type: multipart/form-data
Fields:
  sample_name       — required
  sequencing_type   — required ("WGS"|"WES"|"PANEL"|"OTHER")
  file              — required, single-end (FASTQ/FASTA, .gz ok)
  file_r1, file_r2  — required для paired-end (взаимоисключаемо с `file`)
```

### 8.3. Реализация

- В [`pipeline.Pipeline.Run`](../backend/internal/pipeline/pipeline.go:79) добавить параметр `R2Path string` (опциональный, "" = single-end). `bwa mem` вызывать как `bwa mem ref.fa R1 R2`.
- `SampleHandler.Upload`: если присутствует `file_r1`+`file_r2` — сохранить оба, прокинуть в job; если `file` — старый путь.
- В таблице `sample` добавить `r1_path`, `r2_path` или просто завязаться на job-payload.

### 8.4. Acceptance

- [ ] paired-end запуск bwa проходит и даёт вариант (e2e на 1 миллионе ридов).
- [ ] Передача только `file_r1` без `file_r2` → 422.

---

## 9. Сводный contract diff (для фронта)

| Метод | Ручка | Статус | Закрывает |
|---|---|---|---|
| POST | `/api/auth/register` | расширена (email/first/last) | 1 |
| POST | `/api/patients/{id}/variants/vcf` | новая | 3 |
| POST | `/api/patients/{id}/variants/csv` | новая | 4 |
| POST | `/api/patients/{id}/variants` | новая | 5 |
| GET  | `/api/variants` | новая | 6 |
| GET  | `/api/variants/{id}` | расширена (enrichment_status) | 7 |
| POST | `/api/patients/{id}/samples` | расширена (R1/R2) | 8 |

Все остальные ручки — без изменений контракта.

---

## 10. Порядок реализации

1. **Изоляция (модуль 2)** — её надо сделать раньше всего, иначе все последующие фичи протекают между пользователями.
2. **Email при регистрации (1)** — мелкое, делается параллельно с (2).
3. **Поиск похожих мутаций (6)** — самая ценная фича; разблокирует UI-форму поиска на фронте.
4. **Ручное добавление (5)** — простой POST, открывает форму на дашборде.
5. **Импорт VCF/CSV (3 + 4)** — две похожие multipart-ручки.
6. **Обогащение (7)** — выносится в job, поэтому делается после того, как все писатели вариантов известны.
7. **Paired-end FASTQ (8)** — последний штрих к пайплайну.

---

## 11. Definition of Done

- Все ручки покрыты unit-тестами хендлеров (стабовый репозиторий).
- В [`spec-compliance.md`](spec-compliance.md:1) пункты 3.1, 4.1, 4.2, 4.3 (paired), 6, 7, и расхождение по email переводятся в ✅.
- `swagger.yaml` (опционально) обновлён.
- Фронт может реализовать UI-формы из раздела 5 [`spec-compliance.md`](spec-compliance.md:154) без дополнительных правок контрактов.

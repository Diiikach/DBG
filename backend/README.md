# Backend REST API (Go)

REST API для управления пациентами, выравнивания коротких ридов и получения геномных вариантов пациента.

## Архитектура

```
backend/
├── cmd/api/main.go              # точка входа
├── internal/
│   ├── auth/                    # JWT (HS256) и bcrypt
│   ├── config/                  # конфигурация (ENV)
│   ├── db/                      # пул соединений pgx
│   ├── jobs/                    # фоновая очередь задач (workerpool)
│   ├── models/                  # доменные модели и DTO
│   ├── repository/              # доступ к Postgres (user, patient, sample, variant, ...)
│   ├── pipeline/                # bwa-mem + samtools + bcftools, парсер VCF/FASTA/FASTQ
│   ├── handlers/                # HTTP-обработчики (auth, patient, sample, variant)
│   └── router/                  # сборка маршрутов (chi) + auth middleware
└── data/
    ├── reference/               # ref.fa + индексы bwa/samtools (внешние)
    ├── work/                    # рабочие папки прогонов пайплайна
    └── uploads/
```

## Требования к окружению

- Go 1.22+
- PostgreSQL с залитой схемой (`variant_db`). См. [`DB_ACCESS.md`](../DB_ACCESS.md:1).
- Установленные системные утилиты:
  - [`bwa`](https://github.com/lh3/bwa) (BWA-MEM)
  - [`samtools`](https://www.htslib.org/)
  - [`bcftools`](https://www.htslib.org/)
- Подготовленный референс:
  ```bash
  bwa index data/reference/ref.fa
  samtools faidx data/reference/ref.fa
  ```

На macOS: `brew install bwa samtools bcftools`.

## Конфигурация (ENV)

| Переменная | По умолчанию | Описание |
|---|---|---|
| `HTTP_ADDR` | `:8080` | адрес HTTP-сервера |
| `DATABASE_URL` | `postgresql://genadmin:genadmin@localhost:5433/variant_db?sslmode=disable` | DSN PostgreSQL (БД в контейнере, см. [`docker-compose.yml`](../docker-compose.yml:1)) |
| `REFERENCE_FA` | `./data/reference/ref.fa` | путь к референсному геному (FASTA) |
| `WORK_DIR` | `./data/work` | рабочая папка для пайплайна |
| `UPLOADS_DIR` | `./data/uploads` | папка для загрузок |
| `GENOME_BUILD` | `GRCh38` | имя сборки генома (для `variant.genome_build`) |
| `BWA_BIN` | `bwa` | имя/путь бинарника BWA |
| `SAMTOOLS_BIN` | `samtools` | |
| `BCFTOOLS_BIN` | `bcftools` | |
| `CORS_ORIGINS` | `*` | список разрешённых origin'ов через запятую |
| `JWT_SECRET` | `dev-secret-change-me` | секрет для подписи JWT (HS256). **обязательно** менять в проде |
| `JWT_TTL` | `24h` | время жизни токена (формат [`time.ParseDuration`](https://pkg.go.dev/time#ParseDuration)) |
| `MAX_UPLOAD_MB` | `256` | лимит размера multipart-загрузки sample-файла |
| `JOBS_WORKERS` | `2` | число воркеров фоновой очереди |
| `JOBS_QUEUE_SIZE` | `64` | размер буфера очереди задач |

## Сборка и запуск

```bash
cd backend
go mod tidy
go build -o bin/api ./cmd/api
set -a && source ../db.env && set +a
./bin/api
```

Health-check:
```bash
curl http://localhost:8080/healthz
```

## REST API

> Все ручки в `/api/*`, кроме `POST /api/auth/register` и `POST /api/auth/login`,
> требуют заголовок `Authorization: Bearer <JWT>`.
> Без валидного токена возвращается `401 Unauthorized`.

### Аутентификация

#### `POST /api/auth/register` — регистрация
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"s3cret!","email":"alice@example.com"}'
```
Ответ `201`: `{ "token": "...", "expires_at": 1234567890, "user": {...} }`.

#### `POST /api/auth/login` — вход
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"s3cret!"}'
```
Ответ `200` с тем же форматом, что у `register`.

#### `GET /api/auth/me` — текущий пользователь (требует токена)

### Пациенты

#### `POST /api/patients` — создать пациента
```bash
curl -X POST http://localhost:8080/api/patients \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "Иван",
    "last_name": "Петров",
    "date_of_birth": "1990-05-12",
    "sex": "male",
    "email": "ivan@example.com",
    "phenotype_description": "подозрение на BRCA1"
  }'
```
Ответ: `201 Created` с объектом пациента.

#### `GET /api/patients?limit=50&offset=0&q=петров` — список пациентов
Поддерживает фильтр `q` (подстрока first/last/external_id, регистр игнорируется)
и возвращает `{ items, total, limit, offset }`.

#### `GET /api/patients/{id}` — получить пациента

#### `PATCH /api/patients/{id}` — частичное обновление
Принимает любое подмножество полей `PatientCreate`.

#### `DELETE /api/patients/{id}` — удалить

### Образцы (samples)

#### `POST /api/patients/{id}/samples` — асинхронная загрузка ридов
`multipart/form-data`:
- `sample_name` (text, required)
- `sample_type` (text, default `short-reads`)
- `sequencing_type` (text, default `OTHER`)
- `panel_name` (text, optional)
- `file` (FASTA или FASTQ, опционально gzip-сжатый, required)

```bash
curl -X POST http://localhost:8080/api/patients/1/samples \
  -H "Authorization: Bearer $TOKEN" \
  -F "sample_name=panel-001" \
  -F "file=@reads.fastq.gz"
```
Ответ `202 Accepted`: `{ "sample_id": 42, "patient_id": 1, "status": "processing" }`.

#### `GET /api/samples/{id}` — статус и метаданные образца

#### `GET /api/patients/{id}/samples` — все образцы пациента

### Выравнивание ридов и поиск вариантов

#### `POST /api/patients/{id}/align`
Принимает массив коротких ридов (≤ 2000 нуклеотидов каждый), запускает
полный пайплайн:

```
bwa mem ref.fa reads.fq > aln.sam
samtools view -bS aln.sam | samtools sort -o aln_sorted.bam
samtools index aln_sorted.bam
bcftools mpileup -f ref.fa aln_sorted.bam | bcftools call -mv -Ov -o snps.vcf
```

Затем парсит VCF и сохраняет варианты в БД (`variant`, `patient_variant`, `sample`).

Запрос:
```bash
curl -X POST http://localhost:8080/api/patients/1/align \
  -H "Content-Type: application/json" \
  -d '{
    "sample_name": "panel-001",
    "reads": [
      "ACGTACGTACGTAGCTAGCTAGCTAGCTGATCGATCG...",
      "TTGCAACGTACGTAGCTAGCTAGCTAGCTGATCGATCG..."
    ]
  }'
```

Ответ:
```json
{
  "sample_id": 42,
  "patient_id": 1,
  "vcf_path": "data/work/run-20260519T230000.000000000/snps.vcf",
  "status": "completed",
  "variants": [
    { "Chromosome": "chr1", "Position": 10500, "Reference": "A", "Alternate": "T",
      "Quality": 99.9, "Filter": "PASS", "Zygosity": "heterozygous",
      "ReadDepth": 35, "AlleleDepthRef": 17, "AlleleDepthAlt": 18, "GenotypeQuality": 99 }
  ]
}
```

### Получение вариантов пациента

#### `GET /api/patients/{id}/variants?limit=100&offset=0`

Поддерживаемые query-параметры:
- `limit`, `offset` — пагинация (`limit ≤ 1000`).
- `chrom` — точное совпадение хромосомы (`chr1`, `1`, ...).
- `variant_type` — `SNV | INS | DEL | INDEL | MNV | CNV | SV | OTHER`.
- `filter` — точное совпадение `pv.filter_status` (`PASS`, ...).
- `min_qual` — минимальное `pv.quality` (число).
- `zygosity` — `heterozygous | homozygous | ...`.
- `sort` — список колонок через запятую с префиксом `-` для убывания.
  Допустимы: `chrom`, `position`, `quality`, `depth`.
  Пример: `?sort=chrom,position,-quality`.

Каждая строка содержит агрегированный список `gene_symbols`
(полученный через `variant_annotation ⋈ gene`).

Возвращает все варианты пациента (JOIN `patient_variant ⋈ variant`):

```json
{
  "patient_id": 1,
  "count": 1,
  "limit": 100,
  "offset": 0,
  "items": [
    {
      "patient_variant_id": 123,
      "patient_id": 1,
      "sample_id": 42,
      "variant": {
        "variant_id": 555,
        "chromosome": "chr1",
        "position": 10500,
        "reference": "A",
        "alternate": "T",
        "rs_id": null,
        "genome_build": "GRCh38",
        "variant_type": "SNV"
      },
      "zygosity": "heterozygous",
      "quality": 99.9,
      "read_depth": 35,
      "allele_depth_ref": 17,
      "allele_depth_alt": 18,
      "genotype_quality": 99,
      "filter_status": "PASS",
      "detected_at": "2026-05-19T23:00:00Z"
    }
  ]
}
```

### Деталка варианта и когорта

#### `GET /api/variants/{id}` — детали варианта
Агрегирует `variant ⋈ variant_annotation ⋈ gene ⋈ gene_phenotype ⋈ phenotype`
и `variant_interpretation`. Возвращает также `patient_count` — сколько
пациентов имеют этот же `variant_id`.

#### `GET /api/variants/{id}/patients?limit=100&offset=0` — когорта
Список пациентов с указанным `variant_id` (для просмотра «других пациентов
с тем же вариантом»). Возвращает `{ items, total, limit, offset }`.

## Ограничения

- Длина одного рида: до 2000 нуклеотидов (`MaxReadLen`).
- Максимальное число ридов в одном запросе: 10000 (`MaxReadsCount`).
- Алфавит: `A`, `C`, `G`, `T`, `N`. Любые другие символы → 400.

## Замечания по БД

- Варианты дедуплицируются по `(chromosome, position, reference, alternate, genome_build)` через `idx_variant_unique`.
- `patient_variant` имеет уникальный индекс `(patient_id, variant_id)`: повторная загрузка того же варианта обновит метрики (UPSERT).
- `sample.processing_status` переходит `processing → completed` (или `failed` при ошибке).
- Поле `sample.sequencing_type` записывается как `OTHER` (короткие риды без явного типа).

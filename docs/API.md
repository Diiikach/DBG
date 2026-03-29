# API (минимальный набор)

Базовые эндпоинты для UI. Формат — JSON.

## Health
`GET /health`  
Ответ: `{"status":"ok","db":"up|down"}`

## Авторизация
`POST /api/auth/register`
```json
{
  "username": "doctor1",
  "email": "doc@example.com",
  "full_name": "Ivan Petrov",
  "password": "secret"
}
```

`POST /api/auth/login`
```json
{"username":"doctor1","password":"secret"}
```

`GET /api/auth/me` — профиль текущего пользователя.  
`POST /api/auth/logout`

Все остальные `/api/*` требуют заголовок:
`Authorization: Bearer <token>`

## Пациенты
`POST /api/patients`
```json
{
  "external_id": "P-001",
  "first_name": "Ivan",
  "last_name": "Petrov",
  "date_of_birth": "1989-12-01",
  "sex": "male",
  "email": "ivan@example.com"
}
```

`GET /api/patients?patient_id=1`  
`GET /api/patients?external_id=P-001`  
`GET /api/patients?limit=100&offset=0`

## Варианты
`POST /api/variants`
```json
{
  "patient_id": 1,
  "chromosome": "7",
  "position": 140453136,
  "reference_allele": "A",
  "alternate_allele": "T",
  "rs_id": "rs121913529",
  "genome_build": "GRCh38",
  "variant_type": "SNV"
}
```

`GET /api/variants?rs_id=rs121913529`  
`GET /api/variants?chromosome=7&position=140453136&reference_allele=A&alternate_allele=T`

## Связь пациент-вариант
`POST /api/patient-variants`
```json
{
  "patient_id": 1,
  "variant_id": 10,
  "zygosity": "heterozygous",
  "quality": 99.8
}
```

`GET /api/patient-variants?patient_id=1` — список вариантов пациента с ClinVar и болезнями.

## Аннотации (описание мутаций)
`POST /api/variant-annotations` — upsert аннотации для `variant_id`.

## Поиск похожих мутаций
`GET /api/variants/similar?by=rsid&rs_id=rs121913529`  
`GET /api/variants/similar?by=gene&gene_id=12`  
`GET /api/variants/similar?by=gene&gene_symbol=BRCA1`  
`GET /api/variants/similar?by=locus&chromosome=7&position=140453136&reference_allele=A&alternate_allele=T`

## Подтягивание данных из публичных источников
`POST /api/variants/enrich`  
```json
{"variant_id": 10}
```
или
```json
{"rs_id": "rs121913529"}
```
Ответ содержит сохранённый `payload` и `external_id`.

## Загрузка VCF
`POST /api/upload/vcf?patient_id=1&sample_id=2&genome_build=GRCh38`  
Multipart поле `file`.

## Загрузка CSV
`POST /api/upload/csv?patient_id=1&sample_id=2`  
Multipart поле `file`. Заголовки:  
`chromosome,position,reference_allele,alternate_allele,rs_id,genome_build,variant_type`

## Загрузка FASTQ и запуск пайплайна
`POST /api/upload/fastq?patient_id=1&sample_name=S1&genome_build=GRCh38`  
Multipart поля: `read1` (обязателен), `read2` (опционально).

## Очередь задач
`GET /api/jobs` — список задач пайплайна.

## Получение сохранённых внешних данных
`GET /api/external/variants?variant_id=10`  
`GET /api/external/variants?rs_id=rs121913529`

## Клиническая сводка по варианту
`GET /api/variants/clinical?variant_id=10`

## Экспорт
`GET /api/export/patients?format=csv|json`  
`GET /api/export/variants?format=csv|json`  
`GET /api/export/patient-variants?format=csv|json`

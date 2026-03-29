# База данных (PostgreSQL)

Проект рассчитан на PostgreSQL 14+. Схема описана в:
- `db_init/` для инициализации Docker
- `src/migrations/` для миграций приложения (зеркало `db_init/`)

## Подключение (Docker по умолчанию)
Согласно `docker-compose.yml`:
- БД: `DGV_BD`
- Пользователь: `user`
- Пароль: `users_pswd`
- Хост/порт: `localhost:5432`

## Основные таблицы
Сущности верхнего уровня:
- `patient`
- `sample` (FK -> `patient`)
- `variant`
- `variant_annotation` (FK -> `variant`, `gene`)
- `patient_variant` (связь `patient` x `variant`, опционально `sample`)
- `gene`
- `disease`
- `gene_disease` (связь `gene` x `disease`)
- `"user"` (пользователи системы, требуется кавычка)
- `variant_interpretation` (FK -> `patient_variant`, `"user"`, опционально `disease`)
- `external_variant_data` (сырые данные из внешних источников)
- `sample_read` (пути к FASTQ)
- `variant_call_job` (очередь пайплайна)
- `variant_disease` (связь вариант‑болезнь)
- `user_session` (сессии пользователей)

## Enum-типы
Определены как PostgreSQL enum:
- `sex_enum` (`male`, `female`, `other`)
- `seq_type` (`WGS`, `WES`, `PANEL`, `RNA-SEQ`, `OTHER`)
- `status` (`uploaded`, `processing`, `annotated`, `completed`, `failed`)
- `variant_type` (`SNV`, `INS`, `DEL`, `INDEL`, `MNV`, `CNV`, `SV`, `OTHER`)
- `gen_build` (`GRCh37`, `GRCh38`)
- `impact_type`, `sift_prediction_type`, `polyphen_prediction_type`, `clinvar_significance_type`
- `zygosity_type`
- `direction`
- `inheritance_pattern`
- `association_type`
- `evidence_level`
- `user_role`
- `acmg_classification`

## Представления
Определены в `db_init/011_create_views.sql`:
- `v_variant_full`
- `v_patient_variants`
- `v_variant_statistics`

## Примечания
- `updated_at` поддерживается триггерами в `db_init/012_create_triggers.sql` для:
  - `patient`
  - `disease`
  - `variant_interpretation`
- `variant_interpretation.acmg_criteria` использует `JSONB`.
- Таблица `"user"` требует экранирования в запросах.
- Поле `patient.owner_user_id` ограничивает доступ к данным конкретного врача.

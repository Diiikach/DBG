# Тестовые скрипты для геномного пайплайна

Набор Python-скриптов для генерации коротких ридов и проверки работы
выравнивания (`bwa mem → samtools sort/index → bcftools mpileup/call`).

## Состав

| Файл | Назначение |
|------|------------|
| [`simulate_reads.py`](simulate_reads.py:1) | Симулятор ридов из FASTA-региона: покрытие, ошибки, SNP, RC. |
| [`gen_random_reads.py`](gen_random_reads.py:1) | Полностью случайные риды (негативный тест: «выравнивания нет»). |
| [`check_pipeline.py`](check_pipeline.py:1) | E2E smoke-test локального пайплайна с проверкой VCF. |

Все скрипты учитывают лимиты бекенда из [`config.go`](../../backend/internal/config/config.go:49):
`MaxReadLen=2000`, `MaxReadsCount=10000`. Форматы вывода (`fasta` / `fastq` /
`plain`) соответствуют распознаваемым парсером в
[`pipeline/fasta.go`](../../backend/internal/pipeline/fasta.go:18).

## Зависимости

* Python 3.10+ (только stdlib);
* для `check_pipeline.py` — `bwa`, `samtools`, `bcftools` в `PATH`;
* проиндексированная FASTA: `bwa index ref.fa && samtools faidx ref.fa`.

## Типовые сценарии

### 1. Smoke-test парсера ридов (без референса)

Просто гоняем простую последовательность через `simulate_reads.py`, чтобы
получить валидный FASTQ для ручной заливки через `POST /api/samples/...`:

```bash
python3 scripts/reads/simulate_reads.py \
    --sequence "ACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGT" \
    --read-len 32 --coverage 20 \
    --format fastq -o /tmp/smoke.fq
```

### 2. Положительный тест: внесение известных SNP

Выбираем регион референса, вносим SNP, прогоняем пайплайн, ожидаем что
`bcftools call` найдёт ровно эти позиции:

```bash
python3 scripts/reads/check_pipeline.py \
    --ref backend/data/reference/ref.fa \
    --region chr1:1000000-1001000 \
    --snp 1000250:A --snp 1000500:T --snp 1000750:G \
    --read-len 100 --coverage 40 --error-rate 0.005
```

Скрипт сравнит ожидаемые `(POS, ALT)` с выдачей `bcftools` и упадёт с
exit-code `1`, если хоть один SNP не подхвачен.

### 3. Негативный тест: шум, который не должен находить варианты

```bash
python3 scripts/reads/gen_random_reads.py \
    -n 500 --read-len 75 --format fastq -o /tmp/noise.fq
```

После прогона `noise.fq` через пайплайн ожидается пустой VCF (или варианты с
крайне низким QUAL).

### 4. Подготовка plain-текстового ввода

Бекенд принимает «голый» список ридов по одной последовательности на строку
(см. [`parsePlain`](../../backend/internal/pipeline/fasta.go:141)):

```bash
python3 scripts/reads/simulate_reads.py \
    --ref backend/data/reference/ref.fa \
    --region chr1:1000000-1001000 \
    --read-len 100 --coverage 10 \
    --format plain -o /tmp/reads.txt
```

## Соответствие лимитам бекенда

| Параметр | Лимит | Где задаётся |
|----------|-------|--------------|
| `MaxReadLen` | 2000 п.н. | [`config.go`](../../backend/internal/config/config.go:49) |
| `MaxReadsCount` | 10 000 | [`config.go`](../../backend/internal/config/config.go:50) |
| Алфавит | `ACGTN` | [`validateReads`](../../backend/internal/pipeline/pipeline.go:38) |
| Качества в FASTQ | игнорируются на парсинге | [`parseFastq`](../../backend/internal/pipeline/fasta.go:104) |

Все скрипты пишут ровно `ACGTN`, выпускают `≤ MaxReadsCount` ридов и в FASTQ
ставят качество `'I'` (Q40) — как и сам бекенд в
[`writeFastq`](../../backend/internal/pipeline/pipeline.go:66).

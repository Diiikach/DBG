#!/usr/bin/env python3
"""
simulate_reads.py — симулятор коротких ридов из референсной FASTA.

Назначение: подготовить тестовый набор ридов для проверки геномного
пайплайна (bwa mem → samtools sort/index → bcftools mpileup/call).

Особенности:
  * вырезает регион из FASTA (по координатам chrom:start-end);
  * нарезает на риды заданной длины с заданным покрытием (coverage);
  * умеет вносить случайные ошибки секвенирования и фиксированные SNP;
  * пишет вывод в FASTA, FASTQ или plain (одна последовательность на строку) —
    все три формата поддерживаются бекендом (см. pipeline/fasta.go).

Пример:
  python3 simulate_reads.py \\
      --ref data/reference/ref.fa \\
      --region chr1:1000000-1001000 \\
      --read-len 100 --coverage 30 \\
      --snp 500:A --snp 750:T \\
      --format fastq -o reads.fq

  # быстрые "сырые" риды без референса (для смок-теста парсера):
  python3 simulate_reads.py --sequence ACGTACGTACGT... --read-len 50 \\
      --coverage 10 --format plain -o reads.txt
"""
from __future__ import annotations

import argparse
import random
import sys
from pathlib import Path
from typing import Iterable, Iterator

NUCS = ("A", "C", "G", "T")


# ---------- FASTA I/O --------------------------------------------------------

def load_fasta(path: Path) -> dict[str, str]:
    """Загрузить FASTA целиком в память: {имя_контига: последовательность}."""
    seqs: dict[str, list[str]] = {}
    name: str | None = None
    with path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.rstrip("\n\r")
            if not line:
                continue
            if line.startswith(">"):
                # имя контига — до первого пробела (как принято в samtools)
                name = line[1:].split()[0]
                seqs[name] = []
            else:
                if name is None:
                    raise ValueError("FASTA: последовательность до заголовка")
                seqs[name].append(line.strip().upper())
    return {k: "".join(v) for k, v in seqs.items()}


def parse_region(region: str) -> tuple[str, int, int]:
    """Разобрать 'chrom:start-end' в (chrom, start, end). Координаты 1-based, включительные."""
    if ":" not in region or "-" not in region:
        raise ValueError(f"region '{region}': ожидалось 'chrom:start-end'")
    chrom, coords = region.split(":", 1)
    start_s, end_s = coords.split("-", 1)
    start, end = int(start_s.replace(",", "")), int(end_s.replace(",", ""))
    if start < 1 or end < start:
        raise ValueError(f"region '{region}': некорректные координаты")
    return chrom, start, end


# ---------- мутации/ошибки ---------------------------------------------------

def apply_snps(seq: str, snps: list[tuple[int, str]], region_start: int) -> str:
    """
    Применить SNP к последовательности региона.

    snps — список (genomic_pos_1based, alt_base). region_start — 1-based позиция,
    с которой начинается seq в геноме.
    """
    arr = list(seq)
    for pos, alt in snps:
        idx = pos - region_start
        if idx < 0 or idx >= len(arr):
            print(f"[warn] SNP в позиции {pos} вне региона — пропущено", file=sys.stderr)
            continue
        ref = arr[idx]
        if ref == alt.upper():
            print(f"[warn] SNP {pos}: ALT={alt} совпадает с REF={ref}", file=sys.stderr)
        arr[idx] = alt.upper()
    return "".join(arr)


def add_errors(seq: str, error_rate: float, rng: random.Random) -> str:
    """С вероятностью error_rate в каждой позиции — заменить нуклеотид случайным другим."""
    if error_rate <= 0:
        return seq
    out = []
    for c in seq:
        if c != "N" and rng.random() < error_rate:
            alt = rng.choice([n for n in NUCS if n != c])
            out.append(alt)
        else:
            out.append(c)
    return "".join(out)


# ---------- генерация ридов --------------------------------------------------

def sliding_reads(
    seq: str,
    read_len: int,
    coverage: int,
    rng: random.Random,
) -> Iterator[str]:
    """
    Нарезать seq на риды длиной read_len со случайными позициями.
    Общее число ридов ≈ coverage * len(seq) / read_len.
    """
    if len(seq) < read_len:
        raise ValueError(f"длина региона {len(seq)} < read_len {read_len}")
    n_reads = max(1, (coverage * len(seq)) // read_len)
    max_start = len(seq) - read_len
    for _ in range(n_reads):
        start = rng.randint(0, max_start)
        yield seq[start : start + read_len]


def reverse_complement(seq: str) -> str:
    comp = {"A": "T", "T": "A", "C": "G", "G": "C", "N": "N"}
    return "".join(comp.get(b, "N") for b in reversed(seq))


# ---------- запись ----------------------------------------------------------

def write_output(reads: Iterable[str], fmt: str, out_path: Path | None) -> None:
    fh = out_path.open("w", encoding="utf-8") if out_path else sys.stdout
    try:
        for i, r in enumerate(reads, 1):
            if fmt == "fasta":
                fh.write(f">read_{i}\n{r}\n")
            elif fmt == "fastq":
                # Q40 ('I' в Phred+33) — как делает бекенд в writeFastq()
                fh.write(f"@read_{i}\n{r}\n+\n{'I' * len(r)}\n")
            else:  # plain
                fh.write(f"{r}\n")
    finally:
        if out_path:
            fh.close()


# ---------- CLI -------------------------------------------------------------

def parse_snp(s: str) -> tuple[int, str]:
    """Парс '500:A' → (500, 'A')."""
    if ":" not in s:
        raise argparse.ArgumentTypeError(f"--snp: ожидалось 'POS:ALT', получено {s!r}")
    pos_s, alt = s.split(":", 1)
    alt = alt.strip().upper()
    if alt not in NUCS:
        raise argparse.ArgumentTypeError(f"--snp: ALT должен быть A/C/G/T, получено {alt!r}")
    return int(pos_s), alt


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Симулятор коротких ридов для тестирования геномного пайплайна",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    src = ap.add_mutually_exclusive_group(required=True)
    src.add_argument("--ref", type=Path, help="путь к референсной FASTA")
    src.add_argument("--sequence", type=str, help="ДНК-последовательность напрямую (для smoke-тестов)")

    ap.add_argument("--region", type=str, help="регион chrom:start-end (1-based, требуется с --ref)")
    ap.add_argument("--read-len", type=int, default=100, help="длина одного рида (≤ 2000)")
    ap.add_argument("--coverage", type=int, default=30, help="среднее покрытие, влияет на число ридов")
    ap.add_argument("--error-rate", type=float, default=0.0, help="вероятность ошибки на позицию (0..1)")
    ap.add_argument(
        "--snp",
        action="append",
        type=parse_snp,
        default=[],
        help="внести SNP вида POS:ALT (можно повторять). POS — 1-based геномная.",
    )
    ap.add_argument(
        "--reverse-fraction",
        type=float,
        default=0.5,
        help="доля ридов, выпускаемых в reverse-complement (имитация двух нитей)",
    )
    ap.add_argument("--seed", type=int, default=42, help="seed для воспроизводимости")
    ap.add_argument("--format", choices=("fasta", "fastq", "plain"), default="fastq")
    ap.add_argument("-o", "--output", type=Path, help="выходной файл (по умолчанию stdout)")
    ap.add_argument(
        "--max-reads",
        type=int,
        default=10000,
        help="жёсткий лимит выпускаемых ридов (соответствует MaxReadsCount бекенда)",
    )
    args = ap.parse_args()

    if args.read_len > 2000:
        ap.error("--read-len > 2000 — бекенд отвергнет (MaxReadLen=2000)")

    rng = random.Random(args.seed)

    # 1. достаём исходную последовательность региона
    if args.ref:
        if not args.region:
            ap.error("--region обязателен при использовании --ref")
        chrom, start, end = parse_region(args.region)
        fasta = load_fasta(args.ref)
        if chrom not in fasta:
            ap.error(f"контиг {chrom!r} не найден в {args.ref}. Доступны: {sorted(fasta)[:10]}…")
        full = fasta[chrom]
        if end > len(full):
            ap.error(f"end={end} > длины контига {chrom} ({len(full)})")
        seq = full[start - 1 : end]
        seq = apply_snps(seq, args.snp, region_start=start)
    else:
        seq = args.sequence.strip().upper()
        if any(c not in "ACGTN" for c in seq):
            ap.error("--sequence содержит символы вне алфавита ACGTN")
        if args.snp:
            # для прямой последовательности считаем что start=1
            seq = apply_snps(seq, args.snp, region_start=1)

    # 2. нарезаем риды
    reads: list[str] = []
    for r in sliding_reads(seq, args.read_len, args.coverage, rng):
        r = add_errors(r, args.error_rate, rng)
        if rng.random() < args.reverse_fraction:
            r = reverse_complement(r)
        reads.append(r)
        if len(reads) >= args.max_reads:
            break

    # 3. пишем
    write_output(reads, args.format, args.output)

    print(
        f"[ok] сгенерировано ридов: {len(reads)}, "
        f"длина региона: {len(seq)}, формат: {args.format}",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""
gen_random_reads.py — генератор полностью случайных ридов.

Используется для «негативных» тестов пайплайна: такие риды, как правило, не
выравниваются на референс bwa mem'ом, или выравниваются с очень низким MAPQ,
из-за чего bcftools call не находит вариантов. Полезно проверить, что
система корректно отрабатывает кейс «вариантов 0».

Пример:
  python3 gen_random_reads.py -n 100 --read-len 75 --format fastq -o noise.fq
"""
from __future__ import annotations

import argparse
import random
import sys
from pathlib import Path

NUCS = ("A", "C", "G", "T")


def gen_random(read_len: int, rng: random.Random) -> str:
    return "".join(rng.choice(NUCS) for _ in range(read_len))


def main() -> int:
    ap = argparse.ArgumentParser(description="Генератор случайных ридов (для негативных тестов)")
    ap.add_argument("-n", "--count", type=int, default=100, help="число ридов (≤ 10000)")
    ap.add_argument("--read-len", type=int, default=75)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--format", choices=("fasta", "fastq", "plain"), default="fastq")
    ap.add_argument("-o", "--output", type=Path)
    args = ap.parse_args()

    if args.count > 10000:
        ap.error("--count > 10000 (MaxReadsCount бекенда)")
    if args.read_len > 2000:
        ap.error("--read-len > 2000 (MaxReadLen бекенда)")

    rng = random.Random(args.seed)
    reads = [gen_random(args.read_len, rng) for _ in range(args.count)]

    fh = args.output.open("w", encoding="utf-8") if args.output else sys.stdout
    try:
        for i, r in enumerate(reads, 1):
            if args.format == "fasta":
                fh.write(f">rand_{i}\n{r}\n")
            elif args.format == "fastq":
                fh.write(f"@rand_{i}\n{r}\n+\n{'I' * len(r)}\n")
            else:
                fh.write(f"{r}\n")
    finally:
        if args.output:
            fh.close()

    print(f"[ok] выпущено {len(reads)} случайных ридов длины {args.read_len}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())

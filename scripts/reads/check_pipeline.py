#!/usr/bin/env python3
"""
check_pipeline.py — end-to-end smoke-test геномного пайплайна.

Идея: на любой проиндексированной FASTA (bwa + samtools faidx) сгенерировать
синтетические риды через simulate_reads.py, прогнать локально команды
bwa mem → samtools sort/index → bcftools mpileup/call и убедиться, что в
итоговом VCF присутствуют ожидаемые SNP.

Требует установленных в PATH утилит: bwa, samtools, bcftools, python3.

Пример:
  python3 check_pipeline.py \\
      --ref data/reference/ref.fa \\
      --region chr1:1000000-1001000 \\
      --snp 1000500:A --snp 1000750:T \\
      --read-len 100 --coverage 30
"""
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent


def run(cmd: str, cwd: Path | None = None) -> str:
    """Запустить shell-команду; стримим stderr, возвращаем stdout."""
    print(f"$ {cmd}", file=sys.stderr)
    res = subprocess.run(
        cmd, shell=True, cwd=cwd, check=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
    )
    if res.stderr.strip():
        print(res.stderr, file=sys.stderr)
    return res.stdout


def ensure_binaries() -> None:
    for b in ("bwa", "samtools", "bcftools", "python3"):
        if shutil.which(b) is None:
            print(f"[err] не найден бинарь {b} в PATH", file=sys.stderr)
            sys.exit(2)


def parse_vcf_positions(vcf: Path) -> list[tuple[str, int, str, str]]:
    """Вернуть список (CHROM, POS, REF, ALT) из VCF, игнорируя multi-allelic."""
    out = []
    with vcf.open("r", encoding="utf-8") as f:
        for line in f:
            if line.startswith("#") or not line.strip():
                continue
            parts = line.rstrip("\n").split("\t")
            if len(parts) < 5:
                continue
            chrom, pos, _id, ref, alt = parts[:5]
            out.append((chrom, int(pos), ref, alt))
    return out


def main() -> int:
    ensure_binaries()

    ap = argparse.ArgumentParser(description="Smoke-test пайплайна выравнивания на синтетических ридах")
    ap.add_argument("--ref", type=Path, required=True, help="индексированная референсная FASTA")
    ap.add_argument("--region", type=str, required=True)
    ap.add_argument("--snp", action="append", default=[], help="POS:ALT (можно повторять)")
    ap.add_argument("--read-len", type=int, default=100)
    ap.add_argument("--coverage", type=int, default=30)
    ap.add_argument("--error-rate", type=float, default=0.005)
    ap.add_argument("--workdir", type=Path, help="рабочая папка (по умолчанию tmp)")
    args = ap.parse_args()

    work = args.workdir or Path(tempfile.mkdtemp(prefix="pipeline-check-"))
    work.mkdir(parents=True, exist_ok=True)
    print(f"[info] workdir: {work}", file=sys.stderr)

    reads_fq = work / "reads.fq"
    sam = work / "aln.sam"
    bam = work / "aln_sorted.bam"
    vcf = work / "snps.vcf"

    # 1. симулируем риды
    sim_cmd = (
        f"python3 {HERE / 'simulate_reads.py'} "
        f"--ref {args.ref} --region {args.region} "
        f"--read-len {args.read_len} --coverage {args.coverage} "
        f"--error-rate {args.error_rate} --format fastq -o {reads_fq}"
    )
    for s in args.snp:
        sim_cmd += f" --snp {s}"
    run(sim_cmd)

    # 2. bwa mem → SAM
    run(f"bwa mem -t 2 {args.ref} {reads_fq} > {sam}")

    # 3. samtools view/sort → sorted BAM
    run(f"samtools view -bS {sam} | samtools sort -o {bam}")
    run(f"samtools index {bam}")

    # 4. bcftools mpileup/call → VCF
    run(f"bcftools mpileup -f {args.ref} {bam} | bcftools call -mv -Ov -o {vcf}")

    # 5. проверка SNP
    called = parse_vcf_positions(vcf)
    print(f"\n[result] вариантов в VCF: {len(called)}", file=sys.stderr)
    for row in called:
        print("  ", row, file=sys.stderr)

    expected = []
    for s in args.snp:
        pos_s, alt = s.split(":")
        expected.append((int(pos_s), alt.upper()))

    missing = []
    for pos, alt in expected:
        hit = next((c for c in called if c[1] == pos and c[3].upper() == alt), None)
        if hit is None:
            missing.append((pos, alt))

    if missing:
        print(f"[FAIL] не найдены ожидаемые SNP: {missing}", file=sys.stderr)
        return 1
    print(f"[OK] все {len(expected)} ожидаемых SNP присутствуют в VCF", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())

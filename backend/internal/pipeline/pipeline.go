// Package pipeline реализует геномный пайплайн:
//   bwa mem ref.fa reads.fq > aln.sam
//   samtools view -bS aln.sam | samtools sort -o aln_sorted.bam
//   samtools index aln_sorted.bam
//   bcftools mpileup -f ref.fa aln_sorted.bam | bcftools call -mv -Ov -o snps.vcf
package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/term-paper-2026/backend/internal/config"
)

// Pipeline инкапсулирует пути к утилитам и референсу.
type Pipeline struct {
	cfg config.Config
}

func New(cfg config.Config) *Pipeline { return &Pipeline{cfg: cfg} }

// Result — продукт работы пайплайна.
type Result struct {
	WorkDir string
	SAMPath string
	BAMPath string
	VCFPath string
	Stderr  string
}

// validateReads валидирует риды: длина и нуклеотидный алфавит.
func (p *Pipeline) validateReads(reads []string) error {
	if len(reads) == 0 {
		return fmt.Errorf("reads must be non-empty")
	}
	if len(reads) > p.cfg.MaxReadsCount {
		return fmt.Errorf("too many reads: %d > %d", len(reads), p.cfg.MaxReadsCount)
	}
	for i, r := range reads {
		r = strings.TrimSpace(strings.ToUpper(r))
		if r == "" {
			return fmt.Errorf("read #%d is empty", i+1)
		}
		if len(r) > p.cfg.MaxReadLen {
			return fmt.Errorf("read #%d too long: %d > %d", i+1, len(r), p.cfg.MaxReadLen)
		}
		for _, c := range r {
			switch c {
			case 'A', 'C', 'G', 'T', 'N':
			default:
				return fmt.Errorf("read #%d has invalid nucleotide %q", i+1, c)
			}
		}
		reads[i] = r
	}
	return nil
}

// writeFastq записывает риды в FASTQ-файл с искусственным качеством 'I' (Q40).
func writeFastq(path string, reads []string) error {
	var buf bytes.Buffer
	for i, seq := range reads {
		buf.WriteString(fmt.Sprintf("@read_%d\n", i+1))
		buf.WriteString(seq)
		buf.WriteString("\n+\n")
		buf.WriteString(strings.Repeat("I", len(seq)))
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Run выполняет полный пайплайн для набора ридов и возвращает путь к VCF.
// Single-end: reads2 == nil. Paired-end: reads2 содержит R2-ленту той же длины.
// Старая сигнатура сохранена через обёртку: вызовы Run(ctx, reads) продолжают работать.
func (p *Pipeline) Run(ctx context.Context, reads []string) (*Result, error) {
	return p.RunPaired(ctx, reads, nil)
}

// RunPaired — расширенная версия Run с поддержкой paired-end FASTQ (модуль 8).
// Если reads2 == nil или пустой — single-end. Иначе bwa mem вызывается как
// `bwa mem ref.fa R1.fq R2.fq`.
func (p *Pipeline) RunPaired(ctx context.Context, reads1, reads2 []string) (*Result, error) {
	if err := p.validateReads(reads1); err != nil {
		return nil, fmt.Errorf("R1: %w", err)
	}
	paired := len(reads2) > 0
	if paired {
		if err := p.validateReads(reads2); err != nil {
			return nil, fmt.Errorf("R2: %w", err)
		}
		if len(reads1) != len(reads2) {
			return nil, fmt.Errorf("paired-end: R1 (%d) and R2 (%d) read counts mismatch",
				len(reads1), len(reads2))
		}
	}
	if _, err := os.Stat(p.cfg.ReferenceFA); err != nil {
		return nil, fmt.Errorf("reference not found at %s: %w", p.cfg.ReferenceFA, err)
	}

	stamp := time.Now().UTC().Format("20060102T150405.000000000")
	workDir := filepath.Join(p.cfg.WorkDir, "run-"+stamp)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir workdir: %w", err)
	}

	fqPath := filepath.Join(workDir, "reads.fq")
	fq2Path := filepath.Join(workDir, "reads_r2.fq")
	samPath := filepath.Join(workDir, "aln.sam")
	bamPath := filepath.Join(workDir, "aln_sorted.bam")
	vcfPath := filepath.Join(workDir, "snps.vcf")

	if err := writeFastq(fqPath, reads1); err != nil {
		return nil, fmt.Errorf("write fastq R1: %w", err)
	}
	if paired {
		if err := writeFastq(fq2Path, reads2); err != nil {
			return nil, fmt.Errorf("write fastq R2: %w", err)
		}
	}

	var stderr bytes.Buffer

	// 1. bwa mem ref.fa R1.fq [R2.fq] > aln.sam
	bwaCmd := fmt.Sprintf("%s mem -t 2 %q %q > %q",
		p.cfg.BwaBinary, p.cfg.ReferenceFA, fqPath, samPath)
	if paired {
		bwaCmd = fmt.Sprintf("%s mem -t 2 %q %q %q > %q",
			p.cfg.BwaBinary, p.cfg.ReferenceFA, fqPath, fq2Path, samPath)
	}
	if err := runShell(ctx, &stderr, bwaCmd); err != nil {
		return nil, fmt.Errorf("bwa mem: %w; stderr=%s", err, stderr.String())
	}

	// 2. samtools view -bS aln.sam | samtools sort -o aln_sorted.bam
	if err := runShell(ctx, &stderr,
		fmt.Sprintf("%s view -bS %q | %s sort -o %q",
			p.cfg.SamtoolsBin, samPath, p.cfg.SamtoolsBin, bamPath),
	); err != nil {
		return nil, fmt.Errorf("samtools sort: %w; stderr=%s", err, stderr.String())
	}

	// 3. samtools index aln_sorted.bam
	if err := runShell(ctx, &stderr,
		fmt.Sprintf("%s index %q", p.cfg.SamtoolsBin, bamPath),
	); err != nil {
		return nil, fmt.Errorf("samtools index: %w; stderr=%s", err, stderr.String())
	}

	// 4. bcftools mpileup -f ref.fa aln_sorted.bam | bcftools call -mv -Ov -o snps.vcf
	if err := runShell(ctx, &stderr,
		fmt.Sprintf("%s mpileup -f %q %q | %s call -mv -Ov -o %q",
			p.cfg.BcftoolsBin, p.cfg.ReferenceFA, bamPath, p.cfg.BcftoolsBin, vcfPath),
	); err != nil {
		return nil, fmt.Errorf("bcftools call: %w; stderr=%s", err, stderr.String())
	}

	return &Result{
		WorkDir: workDir,
		SAMPath: samPath,
		BAMPath: bamPath,
		VCFPath: vcfPath,
		Stderr:  stderr.String(),
	}, nil
}

// runShell запускает команду через /bin/sh -c (нужно для пайпов и redirect).
func runShell(ctx context.Context, stderr *bytes.Buffer, cmd string) error {
	c := exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	c.Stderr = stderr
	c.Stdout = stderr // лог тоже сольём в stderr-буфер для диагностики
	return c.Run()
}

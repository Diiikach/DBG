package pipeline

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

// ParseReads читает поток в формате FASTA или FASTQ (опционально gzip-сжатый) и
// возвращает срез последовательностей. Тип определяется по первому символу:
//
//	'>' — FASTA, '@' — FASTQ, иначе считаем что входной поток уже plain reads,
//	по одной последовательности на строку.
//
// maxReads и maxLen ограничивают размер ввода (защита от OOM на больших файлах).
func ParseReads(src io.Reader, gzipped bool, maxReads, maxLen int) ([]string, error) {
	var r io.Reader = src
	if gzipped {
		gz, err := gzip.NewReader(src)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		r = gz
	}

	br := bufio.NewReaderSize(r, 1<<20)
	// Подсмотрим первый непустой символ.
	first, err := peekFirstNonSpace(br)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	switch first {
	case '>':
		return parseFasta(br, maxReads, maxLen)
	case '@':
		return parseFastq(br, maxReads, maxLen)
	default:
		return parsePlain(br, maxReads, maxLen)
	}
}

func peekFirstNonSpace(br *bufio.Reader) (byte, error) {
	for {
		b, err := br.Peek(1)
		if err != nil {
			return 0, err
		}
		if b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t' {
			if _, err := br.ReadByte(); err != nil {
				return 0, err
			}
			continue
		}
		return b[0], nil
	}
}

func parseFasta(br *bufio.Reader, maxReads, maxLen int) ([]string, error) {
	scanner := bufio.NewScanner(br)
	scanner.Buffer(make([]byte, 1<<16), 1<<22)
	out := []string{}
	var cur strings.Builder
	flush := func() error {
		if cur.Len() == 0 {
			return nil
		}
		s := strings.ToUpper(cur.String())
		if len(s) > maxLen {
			return fmt.Errorf("fasta read too long: %d > %d", len(s), maxLen)
		}
		out = append(out, s)
		cur.Reset()
		if len(out) > maxReads {
			return fmt.Errorf("too many reads: > %d", maxReads)
		}
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line[0] == '>' {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		cur.WriteString(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseFastq(br *bufio.Reader, maxReads, maxLen int) ([]string, error) {
	scanner := bufio.NewScanner(br)
	scanner.Buffer(make([]byte, 1<<16), 1<<22)
	out := []string{}
	// FASTQ: 4 строки на запись. Состояние: 0=header,1=seq,2=plus,3=qual.
	state := 0
	for scanner.Scan() {
		line := scanner.Text()
		switch state {
		case 0:
			if !strings.HasPrefix(line, "@") {
				return nil, fmt.Errorf("fastq: expected '@' header, got %q", line)
			}
		case 1:
			s := strings.ToUpper(strings.TrimSpace(line))
			if len(s) > maxLen {
				return nil, fmt.Errorf("fastq read too long: %d > %d", len(s), maxLen)
			}
			out = append(out, s)
			if len(out) > maxReads {
				return nil, fmt.Errorf("too many reads: > %d", maxReads)
			}
		case 2:
			if !strings.HasPrefix(line, "+") {
				return nil, fmt.Errorf("fastq: expected '+' separator, got %q", line)
			}
		case 3:
			// quality string — игнорируем
		}
		state = (state + 1) % 4
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parsePlain(br *bufio.Reader, maxReads, maxLen int) ([]string, error) {
	scanner := bufio.NewScanner(br)
	scanner.Buffer(make([]byte, 1<<16), 1<<22)
	out := []string{}
	for scanner.Scan() {
		s := strings.ToUpper(strings.TrimSpace(scanner.Text()))
		if s == "" {
			continue
		}
		if len(s) > maxLen {
			return nil, fmt.Errorf("read too long: %d > %d", len(s), maxLen)
		}
		out = append(out, s)
		if len(out) > maxReads {
			return nil, fmt.Errorf("too many reads: > %d", maxReads)
		}
	}
	return out, scanner.Err()
}

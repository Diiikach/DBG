package pipeline

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/term-paper-2026/backend/internal/models"
)

// ParseVCF — простой парсер VCF (поддержка одной колонки семпла).
// Извлекает CHROM, POS, REF, ALT, QUAL, FILTER и из FORMAT/SAMPLE: GT, DP, AD, GQ.
func ParseVCF(path string) ([]models.VCFRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []models.VCFRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 8 {
			continue
		}
		pos, err := strconv.ParseInt(cols[1], 10, 64)
		if err != nil {
			continue
		}

		// Несколько ALT-аллелей: разворачиваем в отдельные записи.
		alts := strings.Split(cols[4], ",")
		var qual *float64
		if q, err := strconv.ParseFloat(cols[5], 64); err == nil {
			qual = &q
		}
		filter := cols[6]
		if filter == "." || filter == "" {
			filter = "PASS"
		}

		var format []string
		var sample []string
		if len(cols) >= 10 {
			format = strings.Split(cols[8], ":")
			sample = strings.Split(cols[9], ":")
		}

		// Колонка ID может содержать "." (нет идентификатора) или один/несколько
		// rsID через ';' (стандарт VCF 4.x). Берём первый ненулевой.
		var rsID *string
		if id := strings.TrimSpace(cols[2]); id != "" && id != "." {
			first := strings.SplitN(id, ";", 2)[0]
			if first != "" {
				v := first
				rsID = &v
			}
		}

		for idx, alt := range alts {
			rec := models.VCFRecord{
				Chromosome: normalizeChromosome(cols[0]),
				Position:   pos,
				Reference:  cols[3],
				Alternate:  alt,
				RsID:       rsID,
				Quality:    qual,
				Filter:     filter,
			}
			fillFormat(&rec, format, sample, idx+1) // alt index 1-based для GT
			out = append(out, rec)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// fillFormat заполняет поля рекорда из FORMAT:SAMPLE-строк VCF.
func fillFormat(rec *models.VCFRecord, format, sample []string, altIdx int) {
	if len(format) == 0 || len(sample) == 0 {
		return
	}
	for i, f := range format {
		if i >= len(sample) {
			break
		}
		v := sample[i]
		if v == "." || v == "" {
			continue
		}
		switch f {
		case "GT":
			// Определяем зиготность относительно текущего altIdx.
			zyg := zygosityFromGT(v, altIdx)
			if zyg != "" {
				rec.Zygosity = &zyg
			}
		case "DP":
			if n, err := strconv.Atoi(v); err == nil {
				rec.ReadDepth = &n
			}
		case "GQ":
			if n, err := strconv.Atoi(v); err == nil {
				rec.GenotypeQuality = &n
			}
		case "AD":
			parts := strings.Split(v, ",")
			if len(parts) >= 1 {
				if n, err := strconv.Atoi(parts[0]); err == nil {
					rec.AlleleDepthRef = &n
				}
			}
			if altIdx < len(parts) {
				if n, err := strconv.Atoi(parts[altIdx]); err == nil {
					rec.AlleleDepthAlt = &n
				}
			}
		}
	}
}

// zygosityFromGT парсит "0/1", "1|1", "1/2" и т. п. относительно altIdx.
func zygosityFromGT(gt string, altIdx int) string {
	sep := "/"
	if strings.Contains(gt, "|") {
		sep = "|"
	}
	parts := strings.Split(gt, sep)
	if len(parts) < 2 {
		return ""
	}
	a, errA := strconv.Atoi(parts[0])
	b, errB := strconv.Atoi(parts[1])
	if errA != nil || errB != nil {
		return ""
	}
	hasAlt := a == altIdx || b == altIdx
	if !hasAlt {
		return ""
	}
	if a == b {
		return "homozygous"
	}
	return "heterozygous"
}

// refseqToChrom — соответствие RefSeq-аксешн GRCh38 → каноническое имя контига.
// Полный геном NCBI приходит с именами вида "NC_000022.11"; маппим
// в человекочитаемые "chr1"…"chrM" для удобства поиска и отображения.
var refseqToChrom = map[string]string{
	"NC_000001": "chr1", "NC_000002": "chr2", "NC_000003": "chr3",
	"NC_000004": "chr4", "NC_000005": "chr5", "NC_000006": "chr6",
	"NC_000007": "chr7", "NC_000008": "chr8", "NC_000009": "chr9",
	"NC_000010": "chr10", "NC_000011": "chr11", "NC_000012": "chr12",
	"NC_000013": "chr13", "NC_000014": "chr14", "NC_000015": "chr15",
	"NC_000016": "chr16", "NC_000017": "chr17", "NC_000018": "chr18",
	"NC_000019": "chr19", "NC_000020": "chr20", "NC_000021": "chr21",
	"NC_000022": "chr22", "NC_000023": "chrX", "NC_000024": "chrY",
	"NC_012920": "chrM",
}

// normalizeChromosome приводит имя контига к каноническому виду "chr*":
//   - "NC_000022.11" → "chr22"
//   - "chr22"        → "chr22" (без изменений)
//   - "22" / "MT"    → "chr22" / "chrM"
//   - всё прочее возвращается как есть (поле в БД — varchar(32)).
func normalizeChromosome(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Убираем версию у RefSeq-аксешна: "NC_000022.11" -> "NC_000022".
	if i := strings.IndexByte(s, '.'); i > 0 && strings.HasPrefix(s, "NC_") {
		if v, ok := refseqToChrom[s[:i]]; ok {
			return v
		}
	}
	if v, ok := refseqToChrom[s]; ok {
		return v
	}
	// "22" / "X" / "MT" -> "chr22" / "chrX" / "chrM"
	if !strings.HasPrefix(s, "chr") {
		alt := s
		if alt == "MT" {
			alt = "M"
		}
		return "chr" + alt
	}
	return s
}

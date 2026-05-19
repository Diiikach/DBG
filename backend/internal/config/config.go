package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — конфигурация приложения, читается из переменных окружения.
type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	ReferenceFA   string // путь к референсному геному (FASTA, проиндексированный bwa+samtools)
	WorkDir       string // рабочая директория для временных файлов пайплайна
	UploadsDir    string
	GenomeBuild   string // GRCh38/hg38/GRCh37
	BwaBinary     string
	SamtoolsBin   string
	BcftoolsBin   string
	MaxReadLen    int // лимит длины одного рида
	MaxReadsCount int

	// --- HTTP ---
	CORSOrigins []string // список разрешённых origin'ов; ["*"] = разрешить все

	// --- Auth ---
	JWTSecret string
	JWTTTL    time.Duration

	// --- Uploads (samples) ---
	MaxUploadMB    int // лимит размера multipart-загрузки в мегабайтах
	JobsWorkers    int // число воркеров фоновой очереди задач
	JobsQueueSize  int // буфер очереди задач

	// --- Enrichment (модуль 7) ---
	EnrichmentEnabled bool
	EnrichmentTimeout time.Duration
	EnrichmentRPS     int // rate-limit на внешний API (запросов/сек)
}

// Load загружает конфиг из ENV с разумными дефолтами.
func Load() Config {
	c := Config{
		HTTPAddr:      getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:   getenv("DATABASE_URL", "postgresql://genadmin:genadmin@localhost:5433/variant_db?sslmode=disable"),
		ReferenceFA:   getenv("REFERENCE_FA", "./data/reference/ref.fa"),
		WorkDir:       getenv("WORK_DIR", "./data/work"),
		UploadsDir:    getenv("UPLOADS_DIR", "./data/uploads"),
		GenomeBuild:   getenv("GENOME_BUILD", "GRCh38"),
		BwaBinary:     getenv("BWA_BIN", "bwa"),
		SamtoolsBin:   getenv("SAMTOOLS_BIN", "samtools"),
		BcftoolsBin:   getenv("BCFTOOLS_BIN", "bcftools"),
		MaxReadLen:    2000,
		MaxReadsCount: 10000,

		CORSOrigins: splitCSV(getenv("CORS_ORIGINS", "*")),

		JWTSecret: getenv("JWT_SECRET", "dev-secret-change-me"),
		JWTTTL:    parseDuration(getenv("JWT_TTL", "24h"), 24*time.Hour),

		MaxUploadMB:   getenvInt("MAX_UPLOAD_MB", 256),
		JobsWorkers:   getenvInt("JOBS_WORKERS", 2),
		JobsQueueSize: getenvInt("JOBS_QUEUE_SIZE", 64),

		EnrichmentEnabled: getenvBool("ENRICHMENT_ENABLED", false),
		EnrichmentTimeout: parseDuration(getenv("ENRICHMENT_TIMEOUT", "10s"), 10*time.Second),
		EnrichmentRPS:     getenvInt("ENRICHMENT_RPS", 3),
	}
	return c
}

func getenvBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func parseDuration(s string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

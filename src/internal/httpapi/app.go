package httpapi

import (
	"database/sql"
	"net/http"
	"os"
	"time"
)

type App struct {
	db         *sql.DB
	httpClient *http.Client
	storageDir string
}

func NewApp(db *sql.DB) *App {
	storage := envOrDefault("DGV_STORAGE_DIR", "/data")
	return &App{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		storageDir: storage,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

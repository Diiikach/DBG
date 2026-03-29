package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"dgv/internal/db"
	"dgv/internal/httpapi"
)

func main() {
	dsn := envOrDefault("DGV_DB_DSN", "")
	conn, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("db init error: %v", err)
	}
	if conn != nil {
		defer conn.Close()
	}

	app := httpapi.NewApp(conn)
	mux := httpapi.NewRouter(app)
	app.StartPipelineWorker()

	addr := envOrDefault("DGV_HTTP_ADDR", ":8080")
	readTimeout := envDuration("DGV_HTTP_READ_TIMEOUT", 5*time.Second)
	writeTimeout := envDuration("DGV_HTTP_WRITE_TIMEOUT", 10*time.Second)
	idleTimeout := envDuration("DGV_HTTP_IDLE_TIMEOUT", 60*time.Second)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	log.Printf("listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("invalid duration for %s=%q, using default %s", key, v, fallback)
		return fallback
	}
	return d
}

# ---------- build stage ----------
FROM golang:1.22-bookworm AS builder

WORKDIR /src

# Кэш зависимостей
COPY go.mod go.sum ./
RUN go mod download || true

# Исходники
COPY . .

# Подтягиваем недостающие зависимости и собираем статический бинарник
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# ---------- runtime stage ----------
FROM debian:bookworm-slim AS runtime

# Биоинформатические утилиты, нужные пайплайну (bwa/samtools/bcftools)
# + ca-certificates для исходящих HTTPS-запросов (на всякий случай)
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        ca-certificates \
        bwa \
        samtools \
        bcftools \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/api /app/api

# Каталоги для данных (монтируются томами из docker-compose)
RUN mkdir -p /app/data/work /app/data/uploads /app/data/reference

ENV HTTP_ADDR=:8080 \
    WORK_DIR=/app/data/work \
    UPLOADS_DIR=/app/data/uploads \
    REFERENCE_FA=/app/data/reference/ref.fa

EXPOSE 8080

ENTRYPOINT ["/app/api"]

FROM golang:1.21-bookworm AS build

WORKDIR /app

COPY src/go.mod ./
RUN go mod download

COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/dgv-api ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    bwa \
    samtools \
    bcftools \
  && rm -rf /var/lib/apt/lists/*

RUN useradd -m -u 10001 app

USER app
WORKDIR /app

COPY --from=build /bin/dgv-api /app/dgv-api

EXPOSE 8080

ENTRYPOINT ["/app/dgv-api"]

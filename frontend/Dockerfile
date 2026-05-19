# syntax=docker/dockerfile:1.7
# ──────────────── builder ────────────────
FROM node:20-alpine AS builder
WORKDIR /app

# Сначала ставим зависимости — лучше кэшируется.
COPY package.json package-lock.json* ./
RUN npm ci

# VITE_API_BASE_URL встраивается на этапе сборки (build-time).
ARG VITE_API_BASE_URL=http://localhost:8081
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

COPY . .
RUN npm run build

# ──────────────── runtime ────────────────
FROM nginx:1.27-alpine AS runtime
WORKDIR /usr/share/nginx/html

# Кастомный nginx-конфиг с SPA fallback.
RUN rm -f /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /app/dist ./

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]

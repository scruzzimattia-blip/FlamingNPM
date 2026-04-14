# ============================================
# Stage 1: Frontend bauen
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /build

COPY web/frontend/package.json web/frontend/package-lock.json* ./
RUN npm ci --no-audit 2>/dev/null || npm install --no-audit

COPY web/frontend/ ./
RUN npm run build

# ============================================
# Stage 2: Go-Backend bauen
# ============================================
FROM golang:1.22-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /waf ./cmd/waf

# ============================================
# Stage 3: Minimales Laufzeit-Image
# ============================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs tzdata \
    && mkdir -p /data /app/web/dist

COPY --from=backend-builder /waf /app/waf
COPY --from=frontend-builder /build/dist /app/web/dist

WORKDIR /app

EXPOSE 8080 8443

ENV BACKEND_URL=http://nginx-proxy-manager:81 \
    LISTEN_ADDR=:8080 \
    API_ADDR=:8443 \
    DB_PATH=/data/waf.db \
    MAX_BODY_SIZE=1048576 \
    RATE_LIMIT_MAX=100 \
    RATE_LIMIT_WINDOW=60 \
    TZ=Europe/Zurich

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8443/api/stats || exit 1

ENTRYPOINT ["/app/waf"]

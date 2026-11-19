# syntax=docker/dockerfile:1

FROM node:18-bookworm-slim AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.11-bookworm AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gritloom ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S gritloom \
    && adduser -S -G gritloom gritloom \
    && mkdir -p /data \
    && chown gritloom:gritloom /data
COPY --from=server /out/gritloom /usr/local/bin/gritloom
COPY --from=server /src/web/dist /app/web/dist
ENV GRITLOOM_ENV=production \
    GRITLOOM_DATA_DIR=/data \
    GRITLOOM_WEB_ROOT=/app/web/dist \
    PORT=8080 \
    TMPDIR=/tmp \
    SQLITE_TMPDIR=/tmp
USER gritloom
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O - "http://127.0.0.1:8080/api/v1/health" | grep -q '"status":"ok"' || exit 1
ENTRYPOINT ["/usr/local/bin/gritloom"]

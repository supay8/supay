# Supay API — multi-stage Go build
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/supay ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/supay-migrate ./cmd/migrate

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/supay /app/supay
COPY --from=builder /app/supay-migrate /app/supay-migrate
# Storage dir for the self-hosted STORAGE_DRIVER=local path.
RUN mkdir -p /app/storage/pdfs
# Se mantiene root para que los bind mounts/volúmenes del compose self-hosted
# sigan siendo escribibles en instalaciones existentes.
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q --spider "http://127.0.0.1:${PORT:-8080}/health" || exit 1
ENTRYPOINT ["/app/supay"]

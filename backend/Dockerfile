# Supay API — multi-stage Go build
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/supay ./cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/supay /app/supay
# Storage dir for STORAGE_DRIVER=local (optional, not required for none)
RUN mkdir -p /app/storage/pdfs
EXPOSE 8081
ENTRYPOINT ["/app/supay"]

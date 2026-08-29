# Supay Self-Hosted

## Resumen

- **selfhosted** = Bring Your Own Infra, con salida a internet (obligatorio para SIAT SOAP). `DEPLOYMENT_MODE=selfhosted` (default).
- **cloud** = Infra administrada (R2 + DB gestionada). `DEPLOYMENT_MODE=cloud`.

PDFs: `STORAGE_DRIVER=local` (disco, default selfhosted) | `none` (on-demand) | `r2` (Cloudflare R2, default cloud). Emisión genera PDF automáticamente en disco/R2.

## Quickstart selfhosted (disco por defecto)

```bash
cp backend/.env.example backend/.env
# Edita backend/.env: DATABASE_URL, API_KEY, SIAT_* (STORAGE_DRIVER=local ya por defecto)
docker compose --profile selfhosted up --build
# o con wrapper legacy:
./scripts/run.sh --build
# PDF queda en ./storage/pdfs/{id}.pdf tras POST /invoices/{id}/emit
```

`docker-compose.yml` ya mapea `./storage:/app/storage` y `STORAGE_PATH=/app/storage/pdfs`. El driver `local` (`internal/pdf/storage_local.go`) escribe atómicamente (`tmp+rename`).

## On-demand sin disco (opt-in)

```bash
# backend/.env
STORAGE_DRIVER=none
```

Sin volumen: `GET /invoices/{id}/pdf` genera al vuelo en `internal/pdf/service.go:41` sin persistir.

## Modo cloud (R2)

```bash
# backend/.env
DEPLOYMENT_MODE=cloud
STORAGE_DRIVER=r2
R2_ACCOUNT_ID=...
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=supay-pdfs
# R2_ENDPOINT opcional, default https://<account>.r2.cloudflarestorage.com
```

```bash
docker compose --profile cloud up --build
```

`internal/pdf/storage_r2.go` usa API S3 compatible. `selfhosted + r2` se fuerza a `local` con warn (no exigir R2 a self-hosted).

## Compose profiles

- Único `docker-compose.yml` con `profiles: ["selfhosted","cloud"]`.
- `postgres` y `api` llevan ambos profiles, comparten definición (no duplicación).
- Fuente de verdad en Go: `internal/config/config.go:Load()`.

## Variables

| Var | Default | Notas |
|-----|---------|-------|
| `DEPLOYMENT_MODE` | `selfhosted` | `selfhosted|cloud` |
| `SELF_HOSTED` | - | Legacy alias `true|1` => `selfhosted` |
| `STORAGE_DRIVER` | `local` en selfhosted, `r2` en cloud | `local|none|r2` (vacío => auto según modo) |
| `STORAGE_PATH` | `/app/storage/pdfs` (container) | Solo `local` |
| `R2_*` | - | Solo `cloud+r2` |

## Verificación

```bash
go vet ./...
go test ./internal/pdf -v
# Emitir genera PDF en disco automáticamente:
curl -X POST -H "X-API-Key: $API_KEY" http://localhost:8081/invoices/{id}/emit
ls ./storage/pdfs/{id}.pdf
curl -H "X-API-Key: $API_KEY" http://localhost:8081/invoices/{id}/pdf --output factura.pdf
```

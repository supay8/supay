# Supay Self-Hosted

## Resumen

- **selfhosted** = Bring Your Own Infra, con salida a internet (obligatorio para SIAT SOAP). `DEPLOYMENT_MODE=selfhosted` (default).
- **cloud** = Infra administrada (R2 + DB gestionada). `DEPLOYMENT_MODE=cloud`.

PDFs/XML: `STORAGE_DRIVER=local` es el default self-hosted. `r2` queda disponible como opción explícita; el SaaS cloud lo exige. La aplicación falla al arrancar si el driver elegido está incompleto.

## Quickstart selfhosted (disco por defecto)

```bash
cp backend/.env.example backend/.env
# Edita backend/.env: JWT_SECRET, BACKEND_SECRET, STORAGE_SIGNING_SECRET y SIAT_*.
# DATABASE_URL dentro de Compose es configurado por docker-compose.yml.
docker compose --profile selfhosted up --build
# o con wrapper legacy:
./scripts/run.sh --build
# PDF queda en ./storage/pdfs/{id}.pdf tras POST /invoices/{id}/emit
```

`docker-compose.yml` ya configura `AUTO_MIGRATE=true` para la única réplica, escucha internamente en `8080` y publica `8081` por defecto. También mapea `./storage:/app/storage` y `filedata:/app/data/files`.

## R2 opcional

```bash
# backend/.env
STORAGE_DRIVER=r2
R2_ACCOUNT_ID=...
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=supay-pdfs
# R2_ENDPOINT opcional, default https://<account>.r2.cloudflarestorage.com
```

```bash
docker compose --profile selfhosted up --build
```

`internal/pdf/storage_r2.go` usa la API S3 compatible. Configurar `r2` nunca degrada silenciosamente a almacenamiento noop: credenciales o bucket faltantes detienen el arranque.

El despliegue SaaS en Cloud Run está documentado en `backend/deploy/cloudrun/README.md`; allí `DEPLOYMENT_MODE=cloud` exige R2 y las migraciones se ejecutan mediante un Job separado.

## Compose profiles

- Único `docker-compose.yml` con `profiles: ["selfhosted","cloud"]`.
- `postgres` y `api` llevan ambos profiles, comparten definición (no duplicación).
- Fuente de verdad en Go: `internal/config/config.go:Load()`.

## Variables

| Var | Default | Notas |
|-----|---------|-------|
| `DEPLOYMENT_MODE` | `selfhosted` | `selfhosted|cloud` |
| `SELF_HOSTED` | - | Legacy alias `true|1` => `selfhosted` |
| `STORAGE_DRIVER` | `local` | `local|r2`; cloud exige `r2` |
| `STORAGE_PATH` | `/app/storage/pdfs` (container) | Solo `local` |
| `R2_*` | - | Necesario cuando `STORAGE_DRIVER=r2` |
| `AUTO_MIGRATE` | `true` en Compose | Válido solo para self-hosted de una réplica |
| `DB_MAX_OPEN` | `60` self-hosted | Opcional; Cloud Run usa `10` y rechaza valores mayores a `15` |

## Verificación

```bash
go vet ./...
go test ./internal/pdf -v
# Emitir genera PDF en disco automáticamente:
curl -X POST -H "X-API-Key: $API_KEY" http://localhost:8081/invoices/{id}/emit
ls ./storage/pdfs/{id}.pdf
curl -H "X-API-Key: $API_KEY" http://localhost:8081/invoices/{id}/pdf --output factura.pdf
```

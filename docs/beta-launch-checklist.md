# Checklist de lanzamiento — Beta cerrada Supay (piloto SIAT)

> Alcance: beta cerrada, 2–3 pilotos, sectores base (1, 8, 11, 24/47/48), **solo
> ambiente PILOTO del SIAT**. Cloud (Cloud Run + R2) y self-hosted.
> Fuera de alcance: beta abierta, producción SIAT, sectores 51–55 como homologados.
> Detalle y veredicto: análisis previo en el chat del 2026-10-03.

## Fase 0 — Higiene de secretos (ambos tracks, primero)

- [x] Verificar que `backend/.env` **nunca se commiteó**: `git log --all -- backend/.env`
      vacío y `.gitignore` lo excluye (`.gitignore:10`).
- [x] Rotación por exposición en Git: no aplica; `backend/.env` no aparece en el
      historial. Si se detecta una exposición futura, rotar antes de la beta las
      claves R2, OpenRouter, `SIAT_CODIGO_SISTEMA` y el password de DB.
- [x] Regenerar valores débiles/placeholder del `.env` local:
      `BACKEND_SECRET`, `ENCRYPTION_KEY` (base64 32B), `JWT_SECRET` (≥32),
      `STORAGE_SIGNING_SECRET` (≥32). Se usaron 32 bytes para `ENCRYPTION_KEY` y
      48 bytes para los demás secretos.
- [x] DoD: `grep -Eri "cambiar-por|tu_clave" backend/.env` sin resultados.

## Fase 1 — Fixes chicos (común a ambos tracks)

- [x] **Ejemplos de API inconsistentes.** Los ejemplos de `backend/docs/api-v1.md` y
      la colección Postman que envían solo `sku/quantity/price` devuelven
      `400 "items[0].description es obligatorio"`. El ítem exige snapshot completo
      (`description`, `codigo_producto_sin`, `unidad_medida`; `customer` sin `id`).
      Corregir ejemplos. Referencia: `backend/docs/sdk-facturacion-una-factura.md:62-67`.
- [x] **`Update()` sin filtro de tenant.** `backend/internal/repository/postgres/invoice_repo.go:150-170`
      usa `Where("id=?")`. Cambiar a `Where("tenant_id=? AND id=?")` + test
      tenantA×tenantB (leer/emitir/anular ajeno → 404/401). Hoy solo lo mitiga el
      handler (`modules/invoice/handler.go:501-528`).
- [x] **`UNIQUE(outbox.event_type,aggregate_id)`** (`migrations/000007_phase9_outbox.up.sql:20`)
      impide re-encolar tras `REJECTED`. Si no se corrige (parcial por status o tabla
      de intentos → v1.1), documentar workaround: el re-emit crea factura nueva.
- [x] DoD: `go test ./...` verde + `TEST_DATABASE_URL=... go test ./internal/repository/postgres -run TestInvoice -count=1` verde.

## Fase 2 — Track self-hosted (~medio día)

- [x] Clon fresco reproducible: `cp backend/.env.example backend/.env`, completar los
      5 valores de Fase 0, `docker compose --profile selfhosted up` → `postgres healthy`
      → `api healthy` sin intervención. (`docker-compose.yml:42` ya trae `AUTO_MIGRATE=true`.)
      Validado con proyecto Compose aislado: migraciones automáticas completas,
      `schema_migrations` y `tenant_configs` presentes, `GET /health` 200.
- [x] Frontend: fijar `VITE_API_URL` a la URL pública de la API (`frontend/.env.example`
      trae `localhost:8081`; `frontend/src/main.tsx:13` ya arranca en `mode="self-hosted"`).
      `pnpm --dir frontend build` verde.
- [ ] Smoke self-hosted: `GET /health` → emitir piloto → `GET .../xml|pdf` (descarga
      firmada local, `STORAGE_PRESIGN_TTL=5m`) → anular.
      `scripts/self-hosted-smoke.sh` ya ejecuta el flujo completo, obtiene el ID de
      la emisión, valida XML/PDF y envía `codigo_motivo`; falta correrlo con el POS,
      CUFD y API key del primer piloto (bootstrap de Fase 4).
- [x] Definir `pg_dump` nocturno del volumen `pgdata` antes de meter NITs reales:
      `scripts/self-hosted-backup.sh`, programado desde cron del host.
- [x] No tocar: imagen corre como root a propósito (`backend/Dockerfile:19-20`) para
      bind mounts; `STORAGE_DRIVER=local` default.

## Fase 3 — Track cloud (~1–2 días)

- [ ] Renderizar los YAML aplicables (`PROJECT_ID`, `PROJECT_NUMBER`, `REGION`,
      `TAG`, `AUTH_HOST`, `APP_HOST`, `EMAIL_WORKER_URL`, `SMTP_HOST`, `EMAIL_FROM`).
      DoD: ningún `${...}` en `backend/deploy/cloudrun/rendered/`, imagen
      inmutable por TAG. (`backend/deploy/cloudrun/README.md`.)
      Los YAML ahora son templates; `deploy/cloudrun/render.sh` exige valores
      reales, rechaza `TAG=latest` y valida los manifiestos renderizados. Falta
      proporcionar proyecto, región y dominios reales.
- [ ] Crear secretos + IAM lector para `supay-runtime`: `supay-database-url`,
      `supay-backend-secret`, `supay-encryption-key`, `supay-r2-account-id`,
      `supay-r2-access-key-id`, `supay-r2-secret-access-key`, `supay-siat-codigo-sistema`
      (+ SMTP solo si se activa email).
      `bootstrap-gcp.sh` crea contenedores/IAM de forma idempotente y
      `check-gcp.sh` exige una versión habilitada; falta ejecutarlos en GCP.
- [x] Email: recomendado **OFF** en beta (`invoice_email_enabled=false` por empresa,
      default). Si se enciende: cola Cloud Tasks `invoice-emails` + 2 service accounts +
      binding IAM + SMTP + desplegar `email-worker.yaml`. (`backend/docs/invoice-email-cloud.md:44-60`.)
      El servicio fija `INVOICE_EMAIL_ENABLED=false`; los manifiestos de email
      solo se renderizan con `ENABLE_EMAIL=true` y todas sus variables presentes.
- [ ] Orden de release estricto: `jobs replace migrate → jobs execute supay-migrate --wait`
      (verifica `schema_migrations` + `tenant_configs`) y **solo después** `services replace`.
      El servicio lleva `AUTO_MIGRATE=false` + `DEPLOYMENT_MODE=cloud` + `STORAGE_DRIVER=r2`.
      `deploy.sh` impone ese orden y aborta antes del servicio si migra falla;
      falta ejecutarlo contra el proyecto real.
- [x] Semana launch: `minScale 0 → 1` en `service.yaml:14` (River duerme en scale-to-zero).
      Volver a `0` con cold start aceptable. Pool `10/5` ya correcto.
- [ ] LB: External LB enruta la API pero **excluye `/metrics` y `/v1/metrics`**
      (públicos a nivel app, `router.go:147,168`; solo red interna per `docs/observability.md:3-11`).
      Nada de `run.app` público directo.
      El servicio ya exige ingress `internal-and-cloud-load-balancing`; falta
      crear/auditar el LB real y verificar ambas rutas desde internet.
- [ ] CORS + Better Auth reales: `BETTER_AUTH_URL/ISSUER/AUDIENCE=https://AUTH_HOST`,
      `CORS_ALLOWED_ORIGINS=https://APP_HOST` (`service.yaml:60-67`).
      DoD: `signup→login→me→companies` 201/200 (`docs/api-v1.md:23-54`).
      El renderer exige hosts reales y HTTPS; falta el despliegue y el smoke con
      los dominios definitivos.

## Fase 4 — Bootstrap piloto + smoke (gate de apertura)

- [ ] Walkthrough manual del **primer** piloto (repetible para el 2º/3º): crear empresa →
      API key → sucursal → POS → certificado P12 → sincronizar catálogos → CUIS/CUFD
      vigente en PILOTO → producto mapeado al sector 1. (Precondiciones en
      `backend/docs/sdk-facturacion-una-factura.md:13-20`; páginas `setup`, `certificate`,
      `siat` del dashboard.) El procedimiento y la evidencia obligatoria quedaron
      definidos en `docs/pilot-beta-runbook.md`; falta ejecutarlo con el piloto real.
- [ ] Smoke cloud (detiene el release si falla, `deploy/cloudrun/README.md:69-84`):
      `curl /health` → bootstrap `/internal/companies` → `POST /v1/invoices/emit` piloto →
      estado/`siat-status` → XML/PDF desde R2 privado (firmados, jamás `R2_PUBLIC_URL`) → anular.
      `scripts/pilot-smoke.sh` automatiza el gate completo y acepta la API key del
      tenant B; falta correrlo contra Cloud Run y el SIAT PILOTO reales.
- [ ] Click-through del dashboard: crear→emitir→descargar→anular. El contrato crítico
      ya tiene cobertura automatizada en `frontend/tests/self-hosted-host.test.mjs`
      y se corrigió el payload de anulación a `codigo_motivo`; falta la pasada real
      de navegador indicada en `docs/pilot-beta-runbook.md`.
- [x] Matriz tenantA×tenantB en facturas → 404/401 (DoD F1). Cubierta en handlers
      y repositorio; el gate real la repite con `TENANT_B_API_KEY`.
- [x] DoD decimales: `PDF total == DB total == SIAT total` en fixtures sector 1 y 11.
- [ ] Alertas mínimas activas: `5xx` POS, outbox sin publicar con tráfico, `>50 PENDING 5m`.
      Las tres reglas están en `backend/deploy/observability/alerts.yml` y el backend
      expone `supay_outbox_pending`; falta cargarlas en el Prometheus/Alertmanager real.

## Riesgos aceptados por escrito (comunicar a pilotos)

- Sectores 51–55 `experimental`: fallar en SIAT piloto es feedback, no homologación.
- Re-emit tras `REJECTED` = factura nueva (bug `UNIQUE outbox`, fix en v1.1).
- Aislamiento tenant total fuera de facturas, worker dedicado, `EmitAsync 202`, OTel,
      SDK, cobertura 41.9% < 70%, sin CI → v1.1. No abren beta cerrada, sí bloquean
      beta abierta y producción SIAT.

## Go / No-Go

- Self-hosted 1 cliente, piloto → **Go** con Fases 0–2 + 4.
- Cloud beta cerrada 2–3 pilotos, piloto → **Go** con Fases 0–4 + `minScale=1`.
- Beta abierta / producción SIAT → **No-Go** hasta v1.1.

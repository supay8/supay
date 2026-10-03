# Plan Beta Supay — lanzar ya (Cloud Run + R2)

> **Meta:** beta abierta usable esta semana. Fallar en SIAT es feedback, no incidente.
> **Decisiones:** beta abierta (no solo cerrada) · todos los sectores compilan (`soportado/experimental`) · **un solo servicio `RUN_MODE=both`** (web + River juntos, dormible) · SaaS beta usa **Cloudflare R2** (verificado con PDFs/XMLs); **self-hosted sigue con `local` por defecto, sin cambios** · self-hosted de terceros fuera de alcance operativo, pero el plan no debe romperlo.

## F0 — Alcance beta (congelado)

Flujos día 1: bootstrap tenant (`POST /internal/companies`) → API key (`X-API-Key`) → crear → emitir síncrono → estado → descargar XML/PDF → anular. Sectores base que ya compilan; `GET sectores` marca `soportado/experimental`.

Fuera de beta (v1.1 con tracción): worker dedicado always-allocated, `EmitAsync 202`, OTel full, dual-write decimal completo, sweep tenant total fuera de facturas, canary 90/10.

### Compatibilidad self-hosted (no se obliga a Run)

- `RUN_MODE` default `both`: un contenedor corre web + River, igual que `docker compose --profile selfhosted up`.
- `DEPLOYMENT_MODE=selfhosted` y `STORAGE_DRIVER=local` siguen default (`.env.example:71,76`). Solo nuestro SaaS usa `cloud + r2`.
- `docker-compose.yml:26 api` monta `./storage` y `filedata` — no se tocan. El split web/worker futuro es solo nuestro deploy.
- `AUTO_MIGRATE=true` válido con 1 réplica self-hosted; Job `migrate` solo para Run.
- **Principio anti-ruptura:** todo comportamiento SaaS-only (exigir `r2`, desactivar `AUTO_MIGRATE`, secretos por Secret Manager) se activa **únicamente** con `DEPLOYMENT_MODE=cloud`. Con `selfhosted` (default) el código conserva el comportamiento actual: `local` válido (`config.go:186,293`), `AUTO_MIGRATE=true` funcional, `.env` como fuente de secretos. `ValidateStorage` no gana rechazos globales; solo el deploy SaaS fija `STORAGE_DRIVER=r2` por entorno.

---

## F1 — Seguridad mínima facturas (gate de apertura, 2-3 días)

Base: EPIC P0-1 del análisis (IDOR cross-tenant). Recorte beta: solo facturas + guard común; resto en v1.1.

**Archivos:** `repository/postgres/invoice_repo.go:62 ListFiltered, :93 GetByID, :155 TransitionStatus, :212 ClaimForEmission, :262 GetByIdempotencyKey, :283 GetByIDs` · `domain/invoice.go:106 InvoiceListFilter, :115 InvoiceRepository` · `usecase/invoice_usecase.go:722 GetByID, :758 ListByPointOfSale, :770 ListInvoices` · `usecase/emission.go:69 Emit, :79 ProcessEmission, :437 VerifyStatus, :480 Annul, :555 RevertAnnul` · `modules/invoice/handler.go:179 getByID, :218 list, :278 emit, :294 siatStatus, :309 annul, :331 revertAnnul, :363 downloadFile (referencia ok), :407 sectores` · `delivery/http/context.go:22 CompanyIDFromContext` · `middleware.go:32,54,104,186,193,213` · `router.go:58,88,126` · `modules/siat/handler.go:391 authenticatedCompany (referencia ok)`.

**Cambio:**

- `InvoiceListFilter += TenantID` obligatorio (`"" → ErrMissingCompanyID`); repo filtra `tenant_id=?` en todo `WHERE` + join/check `points_of_sale.tenant_id`.
- Firmas: `GetByID(tenantID,id)`, `GetByIds(tenantID,ids)`, `TransitionStatus(tenantID,...)`, `ClaimForEmission(tenantID,id)`, `GetByIdempotencyKey(tenantID,posID,key)`.
- Nuevo `delivery/http/tenant.go: RequireTenant(r)` → `401` si `!ok`; post-check `inv.CompanyId == ctx` (responder `404`, no `403`-oráculo); `sectores` usa contexto.
- Fail-closed sin romper tests self-hosted: `lookup==nil` → fatal **solo** cuando `DEPLOYMENT_MODE!=test` y `GO_ENV/APP_ENV!=test` (hoy `router.go:88` documenta `lookup==nil` para tests de handlers aislados: se conserva vía flag explícito de test); bootstrap con `secret==""` → denegar; `extractKeyPrefix` inválido → no-lookup; quitar `log.Println(err)` con eco (`handler.go:67,74,80,115,121`).
- Bug real verificado `router.go:135`: `RateLimitIP(10/60, 10, ...)` es división entera `=0` → tras 10 reqs el bootstrap `/internal/companies` queda bloqueado para siempre. Cambiar a `RateLimitIP(10, 60, ...)` como la ruta `/v1` (`router.go:154`). Sin esto no hay onboarding de tenants.
- Compat firmas F1: cambiar firmas (`GetByID(tenantID,id)`…) obliga a actualizar **todos** los callers, incluidos fakes de tests (`emission_test.go:148,163`) y callers self-hosted. Alternativa de menor blast-radius para beta (recomendada): mantener firmas y hacer el check de pertenencia en usecase/handler (`inv.CompanyId == ctx`, `404`), dejando el cambio de firmas para v1.1.

**Tests:**

```bash
go test ./internal/repository/postgres -run 'TestInvoiceTenant|TestInvoiceIdempotency|TestTransitionStatus' -count=1 -v
TEST_DATABASE_URL=postgres://... go test ./internal/repository/postgres -run TestInvoice -count=1
go test ./internal/delivery/http/... -run 'TestTenant|TestEmit|TestAnnul|TestSiatRequiresAuthenticatedTenant' -count=1 -v
go test ./... -count=1
```

**DoD F1:** matriz tenantA×tenantB en facturas (leer/emitir/anular/listar ajeno → 404/401).

## F2 — River integrado dormible (0.5-1 día, prioridad: lanzar)

Base: EPIC P0-2 recortado a un servicio. Sin `cmd/worker`, sin segundo servicio en beta.

**Archivos:** `cmd/server/main.go:26-48` · `internal/app/emission_queue.go:13 StartEmissionQueue` · `internal/app/container.go:515 EmissionQueue` · `emissionqueue/service.go:45,97,118` · `dispatcher.go:65` · `worker.go:54` · `config/config.go:108,222` · `repository/database/db.go:98 MigrateDB (rivermigrate)` · `app/reaper.go:13`.

**Cambio:**

- `RUN_MODE=web|worker|both` (default `both`); `main.go` inicia River salvo `RUN_MODE=web`; `signal.NotifyContext(SIGTERM/SIGINT)` + `Stop` con `SoftStopTimeout ≤10s` (gracia real de Run; lo no drenado se recupera por `outbox` + `ReleaseStaleSending`).
- Comportamiento declarado: River procesa con instancia viva; en scale-to-zero duerme; al despertar retoma (`outbox PENDING + SKIP LOCKED`, River `UniqueOpts ByArgs+ByQueue` en `service.go:155`).
- Emisión POS sigue síncrona (no depende de River). Gatillo futuro (escrito, no implementado): `outbox PENDING>X` o `>N tenants` → desdoblar worker.

**Tests:**

```bash
go test ./internal/emissionqueue -run 'TestDispatcher|TestWorker|TestCircuit|TestLimiter|TestRetry|TestUnique' -count=1 -v
go test ./internal/app -run 'TestEmissionQueue|TestMaintenance|TestReaper' -count=1 -v
```

**DoD F2:** `INSERT outbox PENDING → ACCEPTED` con tráfico; matar instancia → reintento ≤60s; log `cola de emisión iniciada`.

## F3 — Deploy beta (0.5 día)

**Archivos:** `Dockerfile:2-16` · `.env.example` · `backend/deploy/observability/*` (prometheus + grafana) · `docs/observability.md` · `docker-compose.yml` · `cmd/migrate/main.go` (ya existe: golang-migrate + rivermigrate + verificación, reutilizar como Cloud Run Job).

**Cambio:**

- Job `migrate` (golang-migrate + rivermigrate) una vez por release; web sin `AUTO_MIGRATE`.
- Un servicio: `concurrency 20-80`, `min 0` (`min 1` la semana launch), `CPU request-only`, `timeout 120s` (cubre SIAT 45s + firma + PDF + R2 dentro de `WriteTimeout 60s`), `DB_MAX_OPEN` chico por instancia (5-15, nuevo env opcional con default igual al actual; nunca 40-60 con autoscaling), probes `/health` en el servicio (Run ignora `HEALTHCHECK` Dockerfile).
- Storage por entorno (sin romper self-hosted): SaaS beta fija `STORAGE_DRIVER=r2` + secretos R2 por Secret Manager y falla en arranque si falta (`DEPLOYMENT_MODE=cloud` únicamente). Self-hosted conserva `STORAGE_DRIVER=local` + `STORAGE_LOCAL_PATH` + `STORAGE_SIGNING_SECRET≥32` (`config.go:293-300`, `object_factory.go:16-17`, `factory.go:63-69`), sin nuevos requisitos.
- Migraciones por entorno: SaaS Run usa Job `migrate` con `AUTO_MIGRATE=false`; self-hosted conserva `AUTO_MIGRATE=true` de 1 réplica (compose sin cambios).
- Dockerfile: `supay` (+ futuro `supay-worker`), `USER nonroot` **solo si** los volúmenes compose (`./storage`, `filedata`) siguen escribibles (fijar `UID:GID` + `chown` en docs o mantener usuario root en imagen self-hosted); bind `:$PORT` (Run inyecta 8080, `config.go:252` ya lee `$PORT`; alinear `EXPOSE`/compose que hoy fijan 8081).
- Self-hosted cero-touch (verificado roto hoy): no existe `backend/.env` (solo `.env.example`, `env_file` falla) y `AUTO_MIGRATE` no está en compose ni documentado (`SELF_HOSTED.md`), así que un clon fresco arranca sin tablas. Fijar: `AUTO_MIGRATE=true` en `api.environment` de compose (1 réplica) + doc `cp .env.example .env`.
- SaaS fail-fast real: `pdf/storage_factory.go:33` hace `warn+noop` si falta `R2_BUCKET` (vs `object_factory.go` estricto) → el SaaS podría arrancar sin persistir PDFs en silencio. Con `DEPLOYMENT_MODE=cloud`, convertir en fatal.
- `/metrics` y `/v1/metrics` públicos (`router.go:129`): en Run exponerlos solo en ingress interno o con IAM; en self-hosted documentar red interna (`docs/observability.md` ya lo exige).
- Smoke: `migrate → bootstrap → crear → emitir (piloto) → XML/PDF en R2 → anular`. Alertas: `outbox PENDING>50 5m`, `5xx POS↑`.

**Tests:**

```bash
docker build -t supay:beta .
go test ./internal/config -run 'TestLoad|TestValidateStorage|TestValidateAuth' -count=1 -v
curl -s $SVC/health && curl -s $SVC/metrics | grep supay_ || true
```

## F4 — Estados + decimal mínimos + loop feedback (continuo)

Base: EPIC P1/P2 recortados. Sin dual-write ni worker en beta.

- **SENT (mantener):** DB `chk_invoices_status` (`000004:28`) ya incluye `SENT`; se usa en `siat_batches.go:435` y `sent_package_repo.go:341` pero falta en `transitionAllowed` (`invoice_state.go:43`). Beta: añadir `SENDING→SENT (BATCH_RESERVED)`, `SENT→ACCEPTED/OBSERVED/REJECTED`, `OFFLINE→SENT`; `UpdateBatch` por `TransitionStatus` en happy path. Docs completos en v1.1.
- **Decimal (guarda mínima):** `invoice_repo.go:330-358` `NewFromFloat → NewFromString(+Round)`; dual-write completo en v1.1. DoD beta: `PDF total == DB total == SIAT total` en fixtures base.
- **Loop:** cada rechazo SIAT = `sector + codigoEstado + mensajes` (sin PII) → issue → `experimental→soportado` + fixture.

```bash
go test ./internal/domain -run 'TestInvoiceState|TestTransition' -count=1 -v
go test ./internal/usecase -run 'TestBatch|TestPackage|TestMasiva|TestEmit' -count=1 -v
go test ./internal/adapters/siat -run 'TestTotales|TestCalcular' -count=1 -v
```

---

## F5 — Contrato frontend v1 — completada

Implementado y verificado para la beta self-hosted:

- PDF/XML se descargan con `fetch` autenticado y `Blob`; ya no se usan enlaces sin headers.
- Emisión directa usa una `Idempotency-Key` estable por intento para evitar duplicados por doble click o reintento de red.
- `preview/emit` envía snapshot completo de cliente e ítems (`description`, actividad, código SIN, unidad de medida) y `payment.exchange_rate`.
- La beta queda declarada como self-hosted; `BetterAuthHost` y modo cloud permanecen fuera de alcance hasta v1.1.
- Documentación corregida: auth JWT + `X-Company-ID`, Postman histórico marcado deprecated y OpenAPI parcial renombrado a `fern/una-factura.openapi.yml`.

Verificación: `pnpm --dir packages/dashboard typecheck`, `lint`, `build`; `pnpm --dir frontend test`.

## v1.1 (con tracción, fuera de beta)

Sweep tenant total (`1.2/1.3`: branch/pos/customer/catalog/cert/cufd-cuis), worker dedicado (`min 1`, always-allocated, `max 1-2`, CPU 2), `EmitAsync 202`, OTel/Cloud Monitoring (fuera scrape `/metrics`), dual-write decimal + corte `float64`, `SENT` formal + docs, canary 90/10 con migraciones aditivas, corrección `UNIQUE(outbox.event_type,aggregate_id)` (`000007:20`) que hoy impide re-encolar una factura ya publicada (re-emit tras `REJECTED`) — parcial por `(event_type,aggregate_id,status)` o tabla de intentos.

## Riesgos aceptados en beta (declarados)

- Tenant fuera de facturas se cierra en v1.1 (beta etiquetada, keys por tenant).
- Sectores experimentales fallarán en SIAT: es el feedback, no se facturan como homologados.
- Reintentos async atados a instancia viva (`min 1` temporal si `PENDING` crece).

**Orden:** F1 → F2 → F3 → abrir beta → F4. Estimación: ~4-6 días foco.

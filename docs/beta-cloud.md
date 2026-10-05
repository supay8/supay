# Runbook Beta — Cloud (Cloud Run + R2, beta cerrada)

> Audiencia: devs/terceros que ejecutan sin asumir contexto del proyecto.
> Alcance: **beta cerrada, 2–3 pilotos, sectores base (1, 8, 11, 24/47/48), solo PILOTO SIAT.**
> Tiempo estimado: **1–2 días** (ver §11). Doc hermano self-hosted: `beta-selfhosted.md`.
> Índice general: `beta-launch-checklist.md`.
> Referencia de manifiestos: `backend/deploy/cloudrun/README.md`.

## 0. Arquitectura del despliegue

- Un servicio `supay-beta` con web + River juntos (`RUN_MODE=both`, `DEPLOYMENT_MODE=cloud`,
  `STORAGE_DRIVER=r2`, `AUTO_MIGRATE=false`).
- Un Job `supay-migrate` (misma imagen, `/app/supay-migrate`) que se ejecuta **antes** de
  cada release. Email opcional: servicio `supay-email-worker` + Job `supay-email-dispatcher`
  + cola Cloud Tasks (recomendado **OFF** en beta, §7).
- Ingress del servicio: `internal-and-cloud-load-balancing`. La API se publica vía
  External Application Load Balancer; `/metrics` y `/v1/metrics` **nunca** públicos (§8).
- Semana de lanzamiento: `minScale=1` (River duerme en scale-to-zero); después volver a `0`.

## 1. Pre-requisitos

| # | Requisito | Dueño | Verificación |
|---|-----------|-------|--------------|
| 1 | Proyecto GCP + facturación, región elegida (ej. `us-central1`) | Operador | `gcloud projects describe $PROJECT_ID` |
| 2 | Artifact Registry (`$REGION-docker.pkg.dev/$PROJECT_ID/supay/backend`) | Dev/Ops | `gcloud artifacts repositories list` |
| 3 | PostgreSQL gestionada (Cloud SQL) accesible desde Cloud Run (conector o IP privada) | Operador | `psql "$DATABASE_URL" -c 'select 1'` desde red equivalente |
| 4 | Bucket **privado** Cloudflare R2 + credenciales (nunca `R2_PUBLIC_URL` para fiscal) | Operador | `aws s3 ls s3://supay-beta-private --endpoint-url $R2_ENDPOINT` (o dashboard R2) |
| 5 | `SIAT_CODIGO_SISTEMA`, NITs/token/P12 de los 2–3 pilotos | Piloto/empresa | valores en mano antes del §9 |
| 6 | Dominio app (`APP_HOST`) + auth (`AUTH_HOST`) con TLS en el LB | Operador | `curl -sI https://$APP_HOST | head -1` |
| 7 | (Solo si email ON) SMTP + cola Cloud Tasks (ver §7) | Operador | credenciales SMTP probadas |

CLI: `gcloud`, `docker`, `curl`, `jq`.

## 2. Higiene de secretos (primero, bloqueante)

- [ ] `git log -- backend/.env` vacío y `.env` en `.gitignore`. Si se commiteó, **rotar**:
      R2, OpenRouter, SIAT, DB.
- [ ] Generar nuevos (no reutilizar placeholders del repo):
      `openssl rand -base64 48` → `BACKEND_SECRET`, `JWT_SECRET`;
      `openssl rand -base64 32` → `ENCRYPTION_KEY`.
- [ ] Crear los 7 secretos (la cuenta `supay-runtime` necesita lectura):
```bash
for s in supay-database-url supay-backend-secret supay-encryption-key \
         supay-r2-account-id supay-r2-access-key-id supay-r2-secret-access-key \
         supay-siat-codigo-sistema; do
  printf 'valor de %s: ' "$s"
  gcloud secrets create "$s" --replication-policy=automatic --region="$REGION" --quiet \
    || echo "(ya existe, actualizar versión)"
done
printf '%s' "$DATABASE_URL" | gcloud secrets versions add supay-database-url --data-file=-
printf '%s' "$BACKEND_SECRET" | gcloud secrets versions add supay-backend-secret --data-file=-
# ... repetir por secreto (usar --data-file=- evita exponer el valor en historial)
gcloud secrets add-iam-policy-binding supay-database-url \
  --member="serviceAccount:supay-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/secretmanager.secretAccessor --region="$REGION"
# ... repetir por secreto
```
- [ ] DoD: los 7 secretos existen con ≥ 1 versión y el binding de lectura otorgado.

## 3. Preparar manifiestos (placeholders → valores)

En `backend/deploy/cloudrun/*.yaml` sustituir **todos**:

| Placeholder | Valor | Archivos |
|-------------|-------|----------|
| `PROJECT_ID` | ID del proyecto | todos |
| `PROJECT_NUMBER` | número del proyecto | `service.yaml`, `migrate-job.yaml` |
| `REGION` | ej. `us-central1` | todos |
| `TAG` | `git rev-parse --short HEAD` (imagen inmutable) | todos |
| `AUTH_HOST` | ej. `auth.supay.bo` | `service.yaml` (`BETTER_AUTH_*`) |
| `APP_HOST` | ej. `app.supay.bo` | `service.yaml` (`CORS_ALLOWED_ORIGINS`) |
| `EMAIL_WORKER_URL` | URL del email-worker | `service.yaml` (o quitar si email OFF) |
| `SMTP_HOST`, `EMAIL_FROM` | proveedor SMTP | `email-worker.yaml` (o no desplegar si email OFF) |

```bash
export PROJECT_ID="mi-proyecto" REGION="us-central1"
export TAG="$(git rev-parse --short HEAD)"
export IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/supay/backend:${TAG}"
grep -rn "PROJECT_ID\|PROJECT_NUMBER\|AUTH_HOST\|APP_HOST\|EMAIL_WORKER_URL\|SMTP_HOST\|EMAIL_FROM\|:TAG" \
  backend/deploy/cloudrun/   # debe quedar VACÍO antes de seguir
```

Semana launch: en `service.yaml` cambiar `autoscaling.knative.dev/minScale: "0"` → `"1"`.

## 4. Build de imagen

```bash
gcloud builds submit backend --tag "$IMAGE"
# La misma imagen lleva /app/supay + /app/supay-migrate + /app/supay-email-dispatcher
# (backend/Dockerfile:7-16). Los 3 recursos usan LA MISMA imagen inmutable.
```

## 5. Migraciones (Job, antes de mover tráfico — siempre)

```bash
gcloud run jobs replace backend/deploy/cloudrun/migrate-job.yaml --region "$REGION"
gcloud run jobs execute supay-migrate --region "$REGION" --wait
# Éxito = schema_migrations al día + verificación tenant_configs
# (cmd/migrate/main.go:23-35). Si falla: NO desplegar el servicio, ir a §10.2.
```

El servicio lleva `AUTO_MIGRATE=false` y el backend lo exige con `DEPLOYMENT_MODE=cloud`
(`config.go:406-411`): migrar por Job no es opcional.

## 6. Desplegar el servicio

```bash
gcloud run services replace backend/deploy/cloudrun/service.yaml --region "$REGION"
curl -fsS "$SVC/health"   # {"status":"ok"} — $SVC = URL pública del LB (NO run.app directo)
```

Parámetros operativos (`service.yaml`): puerto `8080`, concurrencia `40`, timeout `120s`
(cubre SIAT 45s + firma + PDF + R2 dentro de `WriteTimeout 60s`), pool DB `10/5`
(el backend rechaza `DB_MAX_OPEN > 15` en cloud), CPU request-only + startup boost,
probes `/health`.

## 7. Email de factura (recomendado OFF en beta)

Default por empresa `invoice_email_enabled=false`: con eso no se crea ninguna notificación
(`docs/invoice-email-cloud.md:11-20`) aunque el servicio lleve `INVOICE_EMAIL_ENABLED=true`.
Para encenderlo (solo si hay SMTP probado):

```bash
# 7.1. Cola + cuentas
gcloud tasks queues create invoice-emails --location="$REGION" \
  --max-attempts=12 --min-backoff=10s --max-backoff=3600s --max-concurrent-dispatches=20
gcloud iam service-accounts create supay-email-tasks
gcloud iam service-accounts create supay-email-worker
gcloud run services add-iam-policy-binding supay-email-worker --region="$REGION" \
  --member="serviceAccount:supay-email-tasks@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/run.invoker
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:supay-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/cloudtasks.enqueuer
gcloud iam service-accounts add-iam-policy-binding \
  "supay-email-tasks@${PROJECT_ID}.iam.gserviceaccount.com" \
  --member="serviceAccount:supay-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/iam.serviceAccountUser
# NUNCA roles/run.invoker a allUsers sobre el worker.

# 7.2. Secretos SMTP + despliegue (misma imagen) + scheduler cada minuto
printf '%s' "$SMTP_USER" | gcloud secrets versions add supay-smtp-username --data-file=-
printf '%s' "$SMTP_PASS" | gcloud secrets versions add supay-smtp-password --data-file=-
gcloud run services replace backend/deploy/cloudrun/email-worker.yaml --region="$REGION"
gcloud run services replace backend/deploy/cloudrun/service.yaml --region="$REGION"
gcloud run jobs replace backend/deploy/cloudrun/email-dispatcher-job.yaml --region="$REGION"
gcloud scheduler jobs create http supay-email-dispatcher --location="$REGION" \
  --schedule="* * * * *" \
  --uri="https://${REGION}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${PROJECT_ID}/jobs/supay-email-dispatcher:run" \
  --http-method=POST \
  --oauth-service-account-email="supay-scheduler@${PROJECT_ID}.iam.gserviceaccount.com"
# EMAIL_WORKER_URL debe ser idéntica en API, worker y dispatcher.
```

## 8. Red: LB y `/metrics` interno (bloqueante de seguridad)

- `GET /metrics` y `GET /v1/metrics` son **públicos a nivel app** (`router.go:147,168`).
  El External LB debe enrutar la API pero **excluir** ambas rutas; el scraper accede por
  ruta interna (`docs/observability.md:3-11`).
- Si se usó el dominio `run.app` para un smoke temporal, revertirlo antes de abrir la beta.
- DoD:
```bash
curl -s -o /dev/null -w '%{http_code}\n' "$SVC/metrics"      # 404/403 desde internet
curl -s -o /dev/null -w '%{http_code}\n' "$SVC/v1/metrics"   # 404/403 desde internet
curl -s "$SVC/health"                                        # {"status":"ok"}
```

## 9. Bootstrap pilotos + smoke (gate de apertura)

Repetir el onboarding completo por piloto (empresa → API key → sucursal → POS → P12 →
setup CUIS/CUFD → sincronizar → readiness). Secuencia exacta con curl en
`beta-selfhosted.md §4` (los endpoints son idénticos; solo cambia `$BASE` al `$SVC` cloud).

Smoke automatizado (`scripts/pilot-smoke.sh`: health → emit → verifica `ACCEPTED|OBSERVED` →
descarga XML/PDF → anula). Dos modos:

```bash
# Modo A: bootstrap en el propio smoke (crea empresa nueva)
API_URL="$SVC" BACKEND_SECRET="$BACKEND_SECRET" \
SMOKE_BOOTSTRAP_JSON='{"nit":"990000001","business_name":"Smoke Beta","ambiente":"PILOTO"}' \
SMOKE_INVOICE_JSON="$(cat invoice-piloto.json)" \
./scripts/pilot-smoke.sh   # imprime la API key: GUARDARLA, solo se muestra una vez

# Modo B: piloto ya creado
API_URL="$SVC" SMOKE_API_KEY="$API_KEY" \
SMOKE_INVOICE_JSON="$(cat invoice-piloto.json)" SMOKE_ANNUL_REASON=1 \
./scripts/pilot-smoke.sh
```

`invoice-piloto.json`: payload mínimo real (snapshot completo — ver `beta-selfhosted.md §5`;
`sku` solo devuelve 400). El XML/PDF deben venir del bucket **privado** `supay-beta-private`
vía descargas firmadas; jamás configurar `R2_PUBLIC_URL` para fiscal.
DoD: smoke verde en los 2–3 pilotos + click-through del dashboard + matriz tenantA×tenantB
(ajeno → 404/401) + `PDF == DB == SIAT` en sector 1.

## 10. Troubleshooting cloud

| Síntoma | Causa probable | Acción |
|---------|----------------|--------|
| Job migrate falla | `DATABASE_URL` (conector Cloud SQL / secreto) | Revisar secreto + conectividad; NO tocar el servicio hasta Job verde |
| Servicio `403` en todo | `BACKEND_SECRET`/secretos no montados | `gcloud run services describe supay-beta` → revisar `secretKeyRef` y bindings §2 |
| Login cloud falla | `BETTER_AUTH_*` placeholder | Verificar `AUTH_HOST` real y JWKS accesible; en self-hosted se usa JWT local, no aplica aquí |
| Emisión `SIAT_UNAVAILABLE` | red/salida a SIAT o CUFD vencido | `siat-status` del POS; `POST /v1/siat/cufd/{pos}`; reintentar sobre el ID (no duplicar POST) |
| `SIAT_REJECTED` 422 | datos/campo sectorial | `details[]` → issue (solo sector+código+mensajes, sin PII) → fixture → fix; en beta: factura nueva |
| Outbox `PENDING` crece con `minScale=0` | instancia dormida | Subir a `minScale=1`; SQL: `select count(*) from outbox where status='PENDING'` |
| Métricas públicas | LB con ruta `/*` | Excluir `/metrics|/v1/metrics` (§8) |
| `VALIDATION_ERROR unknown field` | typo en payload | Campos exactos en `beta-selfhosted.md §5`; programar contra `code`, no `message` |

Rollback: `gcloud run services replace` con el YAML del TAG anterior (imagen inmutable
conservada en Artifact Registry); el Job migrate solo avanza (migraciones reversibles en
`internal/repository/database/migrations/*.down.sql` si hiciera falta).

## 11. Plan de ejecución y responsables

| Fase | Tarea | Dueño | Tiempo est. | DoD |
|------|-------|-------|-------------|-----|
| 0 | Secretos §2 | Operador | 1 h | 7 secretos + bindings |
| 1 | Manifiestos §3 + build §4 | Dev/Ops | 2 h | grep placeholders vacío + imagen |
| 2 | Migrate Job §5 + servicio §6 | Dev/Ops | 1 h | `/health` ok |
| 3 | Red §8 (+ email §7 si aplica) | Operador | 2 h | metrics no públicos |
| 4 | Pilotos + smoke §9 | Dev + piloto | 3–4 h | smoke verde × pilotos |
| 5 | Alertas + apertura | Operador | 1 h | §12 activo |

Alertas mínimas día 1 (§12): `5xx` POS, outbox sin publicar con tráfico,
`>50 PENDING 5m` (SQL o métrica), p95 emisión sobre SLO. Riesgos aceptados por escrito
a pilotos: sectores 51–55 experimentales; re-emit tras `REJECTED` = factura nueva
(`UNIQUE outbox`, fix v1.1); tenant total fuera de facturas, worker dedicado,
`EmitAsync 202`, OTel, SDK, cobertura 41.9%, sin CI → v1.1 (bloquean abierta/producción,
no esta beta).

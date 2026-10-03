# Cloud Run beta

Los YAML de este directorio son templates. `render.sh` genera manifiestos
aplicables en `rendered/` y falla si queda una variable sin resolver. La beta
arranca con email desactivado; el worker y el Job de recuperación solo se
renderizan al definir `ENABLE_EMAIL=true`.

## Parámetros operativos

- Puerto `8080`, concurrencia `40` y timeout HTTP `120s`.
- CPU solo durante requests, `minScale=1` durante la semana de lanzamiento,
  `maxScale=10` y startup CPU boost. Vuelva a `0` cuando el tráfico real
  confirme que el arranque en frío es aceptable.
- Pool PostgreSQL `10/5` por instancia; el backend rechaza `DB_MAX_OPEN > 15`
  cuando `DEPLOYMENT_MODE=cloud`.
- `AUTO_MIGRATE=false` en el servicio. El mismo artefacto contiene
  `/app/supay-migrate`, que aplica tanto migraciones de aplicación como River.
- R2 es obligatorio y privado en cloud. No configure `R2_PUBLIC_URL` para los
  artefactos fiscales; use las descargas firmadas de la API.

Cloud Run inyecta `PORT` al contenedor de servicio y exige que escuche en esa
dirección/puerto. Los Jobs deben terminar en vez de levantar HTTP. Los archivos
siguen el [contrato de contenedores de Cloud Run](https://cloud.google.com/run/docs/container-contract),
la [referencia YAML v1](https://cloud.google.com/run/docs/reference/yaml/v1) y
la configuración oficial de [health checks](https://cloud.google.com/run/docs/configuring/healthchecks).

## Release

Desde la raíz del repositorio:

```bash
export PROJECT_ID="mi-proyecto"
export REGION="us-central1"
export TAG="$(git rev-parse --short HEAD)"
export IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/supay/backend:${TAG}"
export PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')"
export AUTH_HOST="auth.example.com"
export APP_HOST="app.example.com"

gcloud builds submit backend --tag "$IMAGE"
backend/deploy/cloudrun/render.sh
```

`TAG=latest` se rechaza. Antes de continuar, inspeccione los manifiestos de
`backend/deploy/cloudrun/rendered/` y confirme proyecto, región, dominios e
imagen.

1. Cree o actualice los secretos `supay-database-url`, `supay-backend-secret`,
   `supay-encryption-key`, `supay-r2-account-id`, `supay-r2-access-key-id` y
   `supay-r2-secret-access-key`, además de `supay-siat-codigo-sistema`. La cuenta
   `supay-runtime` necesita acceso de lectura a esas versiones.

```bash
APPLY_GCP=yes backend/deploy/cloudrun/bootstrap-gcp.sh
# Agregue una versión habilitada a cada secreto; el script nunca recibe valores.
backend/deploy/cloudrun/check-gcp.sh
```

2. Ejecute migraciones antes de mover tráfico. `deploy.sh` impone el orden y
   termina si el Job falla; solo entonces reemplaza el servicio:

```bash
APPLY_GCP=yes backend/deploy/cloudrun/deploy.sh
```

El script equivale a `jobs replace` → `jobs execute --wait` → `services
replace`; no continúe manualmente si la ejecución de `supay-migrate` falla.

Email permanece apagado (`INVOICE_EMAIL_ENABLED=false`) durante la beta. La
configuración opcional de Cloud Tasks, el worker privado y el Job que republica
notificaciones pendientes se documentan en
[`docs/invoice-email-cloud.md`](../../docs/invoice-email-cloud.md).

El ingress del template es `internal-and-cloud-load-balancing`. Publique la API
mediante un External Application Load Balancer y no cree rutas públicas para
`/metrics` ni `/v1/metrics`; el backend de métricas debe ser interno. Si se usa
el dominio `run.app` durante un smoke temporal, revierta esa excepción antes de
abrir la beta.

El URL map o la política de seguridad del LB debe rechazar explícitamente
`/metrics` y `/v1/metrics`. Verifique desde internet que ambas rutas fallan y,
desde el scraper interno, que siguen respondiendo. El ingress evita que el
dominio `run.app` sea un bypass directo del balanceador.

## Smoke de release

```bash
curl -fsS "$SVC/health"

BOOTSTRAP="$(curl -fsS -X POST "$SVC/internal/companies" \
  -H "X-Backend-Token: $BACKEND_SECRET" \
  -H 'Content-Type: application/json' \
  -d '{"nit":"990000001","business_name":"Smoke Beta","ambiente":"PILOTO"}')"
API_KEY="$(printf '%s' "$BOOTSTRAP" | jq -r '.api_key')"
test -n "$API_KEY" && test "$API_KEY" != null
```

Continúe con el fixture piloto de `docs/sdk-facturacion-una-factura.md` usando
`scripts/pilot-smoke.sh`. El script crea y emite, comprueba estado y aislamiento
tenant, descarga XML/PDF desde R2 y anula. El release se detiene si cualquier
paso falla. El walkthrough y la evidencia exigida están en
[`docs/pilot-beta-runbook.md`](../../../docs/pilot-beta-runbook.md).

Las reglas listas para Prometheus están en
`deploy/observability/alerts.yml`. Alertas mínimas de lanzamiento:

- tasa de respuestas `5xx` en POS;
- ausencia de publicaciones del outbox con tráfico;
- más de 50 eventos `PENDING` durante cinco minutos, consultado desde PostgreSQL
  o exportado como métrica por el sistema de monitoreo.

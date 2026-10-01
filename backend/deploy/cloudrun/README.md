# Cloud Run beta

Estos manifiestos despliegan una sola revisión `RUN_MODE=both` y un Job separado
para las migraciones. Reemplace `PROJECT_ID`, `PROJECT_NUMBER`, `REGION`, `TAG`,
`AUTH_HOST` y `APP_HOST` antes de aplicarlos.

## Parámetros operativos

- Puerto `8080`, concurrencia `40` y timeout HTTP `120s`.
- CPU solo durante requests, `minScale=0`, `maxScale=10` y startup CPU boost.
- Para la semana de lanzamiento cambie temporalmente `minScale` a `1`; vuelva a
  `0` cuando el tráfico real confirme que el arranque en frío es aceptable.
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

gcloud builds submit backend --tag "$IMAGE"
```

1. Sustituya los placeholders de ambos YAML por valores reales y por la misma
   imagen inmutable.
2. Cree o actualice los secretos `supay-database-url`, `supay-backend-secret`,
   `supay-encryption-key`, `supay-r2-account-id`, `supay-r2-access-key-id` y
   `supay-r2-secret-access-key`, además de `supay-siat-codigo-sistema`. La cuenta
   `supay-runtime` necesita acceso de lectura a esas versiones.
3. Ejecute migraciones antes de mover tráfico:

```bash
gcloud run jobs replace backend/deploy/cloudrun/migrate-job.yaml --region "$REGION"
gcloud run jobs execute supay-migrate --region "$REGION" --wait
```

4. Solo después del Job exitoso despliegue el servicio:

```bash
gcloud run services replace backend/deploy/cloudrun/service.yaml --region "$REGION"
```

El ingress del template es `internal-and-cloud-load-balancing`. Publique la API
mediante un External Application Load Balancer y no cree rutas públicas para
`/metrics` ni `/v1/metrics`; el backend de métricas debe ser interno. Si se usa
el dominio `run.app` durante un smoke temporal, revierta esa excepción antes de
abrir la beta.

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

Continúe con el fixture piloto de `docs/sdk-facturacion-una-factura.md`: crear y
emitir, comprobar estado, descargar XML/PDF desde R2 y anular. El release se
detiene si cualquier paso falla.

Alertas mínimas de lanzamiento:

- tasa de respuestas `5xx` en POS;
- ausencia de publicaciones del outbox con tráfico;
- más de 50 eventos `PENDING` durante cinco minutos, consultado desde PostgreSQL
  o exportado como métrica por el sistema de monitoreo.

# Email de factura por Cloud Tasks

Esta función existe únicamente con `DEPLOYMENT_MODE=cloud` y tiene dos niveles
de activación: la infraestructura global requiere `INVOICE_EMAIL_ENABLED=true`
y cada empresa debe habilitar `invoice_email_enabled`. Cuando una factura pasa
atómicamente a `ACCEPTED` u `OBSERVED`, tanto en emisión individual como
masiva/paquete, la misma transacción PostgreSQL crea una fila
`invoice_email_notifications` únicamente si la preferencia de la empresa está
activa y el snapshot de la factura contiene un email no vacío.

La preferencia se configura al crear la empresa o mediante su actualización:

```json
{
  "invoice_email_enabled": true
}
```

Si se omite, su valor predeterminado es `false`. Deshabilitarla no elimina
notificaciones ya publicadas; evita que nuevas facturas creen notificaciones.

La API intenta publicar inmediatamente la notificación en Cloud Tasks antes de
responder. Un error de `CreateTask` no revierte una emisión fiscal exitosa: la
fila vuelve a `PENDING`, conserva el error y una fecha de próximo intento. El
Job `supay-email-dispatcher` recupera esas filas; no se usa ninguna goroutine
posterior a la respuesta HTTP.

Cloud Tasks invoca:

```text
POST /internal/tasks/send-invoice-email
Authorization: Bearer <OIDC de supay-email-tasks>
Content-Type: application/json

{"notificacion_id":"<uuid>"}
```

El servicio `RUN_MODE=email-worker` expone solamente `/health` y ese endpoint.
Valida el ID token y el email exacto de la cuenta de servicio, genera el PDF,
lee el XML privado y envía ambos por SMTP. Solo responde `200` después del
envío; cualquier fallo devuelve `5xx` para que Cloud Tasks reintente.

## Recursos e IAM

Use la misma región y proyecto para la cola y los servicios:

```bash
gcloud tasks queues create invoice-emails \
  --location="$REGION" \
  --max-attempts=12 \
  --min-backoff=10s \
  --max-backoff=3600s \
  --max-concurrent-dispatches=20

gcloud iam service-accounts create supay-email-tasks
gcloud iam service-accounts create supay-email-worker

gcloud run services add-iam-policy-binding supay-email-worker \
  --region="$REGION" \
  --member="serviceAccount:supay-email-tasks@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/run.invoker

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:supay-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/cloudtasks.enqueuer

gcloud iam service-accounts add-iam-policy-binding \
  "supay-email-tasks@${PROJECT_ID}.iam.gserviceaccount.com" \
  --member="serviceAccount:supay-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/iam.serviceAccountUser
```

No otorgue `roles/run.invoker` a `allUsers` sobre el worker. La cuenta
`supay-email-worker` necesita acceso a PostgreSQL, R2 y a los secretos SMTP.

## Despliegue

1. Aplique la migración `000019_invoice_email_notifications` con el Job normal
   de migraciones.
2. Sustituya los placeholders de `email-worker.yaml`, despliegue el worker sin
   acceso no autenticado y obtenga su URL HTTPS.
3. Sustituya `EMAIL_WORKER_URL` por esa URL tanto en el worker como en la API y
   en el Job de recuperación. Si usa `EMAIL_WORKER_OIDC_AUDIENCE`, debe coincidir
   exactamente en los tres.
4. Configure los secretos `supay-smtp-username` y `supay-smtp-password`, además
   de los secretos ya usados por PostgreSQL y R2.
5. Despliegue la API y el Job de recuperación con la misma imagen inmutable.

```bash
gcloud run services replace backend/deploy/cloudrun/email-worker.yaml --region="$REGION"
gcloud run services replace backend/deploy/cloudrun/service.yaml --region="$REGION"
gcloud run jobs replace backend/deploy/cloudrun/email-dispatcher-job.yaml --region="$REGION"
```

Programe el Job cada minuto. La identidad del Scheduler necesita
`roles/run.invoker` sobre el Job:

```bash
gcloud scheduler jobs create http supay-email-dispatcher \
  --location="$REGION" \
  --schedule="* * * * *" \
  --uri="https://${REGION}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${PROJECT_ID}/jobs/supay-email-dispatcher:run" \
  --http-method=POST \
  --oauth-service-account-email="supay-scheduler@${PROJECT_ID}.iam.gserviceaccount.com"
```

## Operación

La tarea usa un nombre determinista derivado de la notificación. Si Cloud Tasks
creó la tarea pero la respuesta se perdió, una nueva publicación recibe
`AlreadyExists` y se considera exitosa. Vigile especialmente:

- filas `PENDING` cuya `available_at` ya venció;
- filas `PUBLISHING` o `SENDING` con `locked_at` antiguo;
- crecimiento de `publish_attempts` o `delivery_attempts`;
- errores SMTP y respuestas `5xx` del worker.

El envío es al menos una vez: una caída después de que SMTP acepta el mensaje y
antes de marcar `SENT` puede causar un duplicado. Use un proveedor con clave de
idempotencia si necesita semántica exactamente una vez.

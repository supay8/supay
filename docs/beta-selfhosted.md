# Runbook Beta — Self-Hosted (1 cliente / demo)

> Audiencia: devs/terceros que ejecutan sin asumir contexto del proyecto.
> Alcance: **beta cerrada, 1 tenant, sectores base (1, 8, 11, 24/47/48), solo PILOTO SIAT.**
> Tiempo estimado: **medio día** (ver §9). Doc hermano cloud: `beta-cloud.md`.
> Índice general: `beta-launch-checklist.md`.

## 0. Qué es este modo

- `DEPLOYMENT_MODE=selfhosted` (default). El operador aporta infra + salida a internet
  (obligatoria para SIAT SOAP). Storage `local` en disco. Una sola réplica con
  `AUTO_MIGRATE=true`. Web + workers River en un contenedor (`RUN_MODE=both`).
- La imagen corre como **root a propósito** (`backend/Dockerfile:19-20`) para que los
  bind mounts (`./storage`, `filedata`) sigan escribibles. No cambiar.

## 1. Pre-requisitos

| # | Requisito | Dueño | Verificación |
|---|-----------|-------|--------------|
| 1 | Host Linux con Docker + Compose v2, `curl`, `jq` | Operador | `docker compose version && curl --version && jq --version` |
| 2 | Salida a internet al SIAT piloto | Operador | `curl -sI https://pilotosiatservicios.impuestos.gob.bo/v2 | head -1` |
| 3 | Certificado digital P12 del NIT piloto + su password | Piloto/empresa | archivo `.p12/.pfx` ≤ 5 MB |
| 4 | Token delegado + `SIAT_CODIGO_SISTEMA` del piloto | Piloto/empresa | valores en mano antes de §5 |
| 5 | Node 20+ y pnpm (solo si se sirve el dashboard) | Dev | `node -v && pnpm -v` |

Herramientas del repo (no instalar nada): `scripts/run.sh` (wrapper legacy),
`scripts/self-hosted-backup.sh` (backup pg_dump), `scripts/self-hosted-smoke.sh` (smoke).

## 2. Instalación paso a paso

```bash
git clone <repo> supay && cd supay

# 2.1. Variables de entorno (obligatorio: los 5 secretos del §3)
cp backend/.env.example backend/.env
$EDITOR backend/.env

# 2.2. Levantar (construye imagen, migra esquema+datos, inicia API)
docker compose --profile selfhosted up --build -d
# wrapper legacy equivalente: ./scripts/run.sh --build

# 2.3. Verificar arranque
docker compose ps
curl -s http://localhost:8081/health   # {"status":"ok"}
docker compose logs api | tail -20     # "Migraciones completadas." + "Servidor escuchando"
```

Puertos y volúmenes (`docker-compose.yml:43-50`): `${PORT:-8081}:8080`,
`./storage:/app/storage`, `filedata:/app/data/files`, `pgdata` (postgres).
La API escucha internamente en `8080` y publica `8081`.

## 3. Variables de entorno (las 5 obligatorias)

| Variable | Generación | Notas |
|----------|------------|-------|
| `BACKEND_SECRET` | `openssl rand -base64 48` | Protege `POST /internal/companies`. Sin esto la API no arranca (`cmd/server/main.go:37-39`). |
| `ENCRYPTION_KEY` | `openssl rand -base64 32` | AES-GCM para credenciales por empresa. |
| `JWT_SECRET` | `openssl rand -base64 48` | ≥ 32 caracteres (`config.go:438-462`). Auth del dashboard self-hosted. |
| `STORAGE_SIGNING_SECRET` | `openssl rand -base64 48` | ≥ 32 caracteres, descargas firmadas locales. |
| `SIAT_CODIGO_SISTEMA` | Lo entrega el SIN | Global de la plataforma, no por tenant. |
| `DATABASE_URL` | default del ejemplo | Dentro de Compose la inyecta `docker-compose.yml`; no editar salvo host externo. |
| `SIAT_AMBIENTE=2` | fijo | **2 = piloto. No cambiar a 1 en beta.** |
| `CORS_ALLOWED_ORIGINS` | URLs del dashboard | coma-separadas, ej. `https://panel.cliente.bo` |

DoD: `grep -ri "cambiar-por\|tu_clave" backend/.env` vacío. Nunca commitear `.env`.

## 4. Onboarding del tenant piloto (orden exacto, 8 pasos)

Base: `BASE=http://localhost:8081`. Todas las rutas valen con y sin prefijo `/v1`
(se recomienda `/v1`). Auth integraciones: `X-API-Key`. Auth dashboard: `Bearer JWT + X-Company-ID`.

```bash
# 4.1. Bootstrap empresa (única vez que se ve la API key — guardarla)
BOOTSTRAP=$(curl -s -X POST "$BASE/v1/internal/companies" \
  -H "X-Backend-Token: $BACKEND_SECRET" -H 'Content-Type: application/json' \
  -d '{"nit":"<NIT-PILOTO>","business_name":"<RAZON SOCIAL>",
       "ambiente":"PILOTO","modalidad":1,"municipio":"La Paz",
       "direccion":"Av. ...","telefono":"...","codigo_actividad":"<COD>"}')
echo "$BOOTSTRAP" | jq .
API_KEY=$(echo "$BOOTSTRAP" | jq -r .api_key)
CID=$(echo "$BOOTSTRAP" | jq -r .company.id)
# 201 esperado. 401 texto plano = BACKEND_SECRET mal. 409 = NIT ya existe.
```

```bash
# 4.2. Config operativa (webhook cert + email; token delegado SIAT)
curl -s -X PATCH "$BASE/v1/companies/$CID" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"token_delegado":"<TOKEN-DELEGADO>",
       "certificate_webhook_url":"https://<host>/webhooks/cert",
       "invoice_email_enabled":false}' | jq .
# invoice_email_enabled=false en beta (email es función cloud, ver beta-cloud.md).

# 4.3. Sucursal (company_id + name obligatorios)
BRANCH=$(curl -s -X POST "$BASE/v1/branches/" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d "{\"company_id\":\"$CID\",\"codigo_sucursal\":0,\"name\":\"Casa Matriz\",\"address\":\"...\"}")
BID=$(echo "$BRANCH" | jq -r .id)

# 4.4. Punto de venta (branch_id + name + description obligatorios; guardar el id)
POS=$(curl -s -X POST "$BASE/v1/point-of-sales/" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d "{\"branch_id\":\"$BID\",\"name\":\"Caja 1\",\"description\":\"Punto principal\",\"tipo_punto_venta\":2}")
POSID=$(echo "$POS" | jq -r .id)

# 4.5. Certificado P12 (multipart, campo "p12_file", ≤ 5 MB, .p12/.pfx)
curl -s -X POST "$BASE/v1/companies/$CID/certificates/" -H "X-API-Key: $API_KEY" \
  -F "p12_file=@cert.p12;type=application/x-pkcs12" \
  -F "p12_password=<CLAVE-P12>" -F "name=cert-principal" -F "type=P12" | jq .

# 4.6. Setup SIAT idempotente (CUIS + CUFD de una vez)
curl -s -X POST "$BASE/v1/siat/setup/$POSID" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d "{\"point_of_sale_id\":\"$POSID\"}" | jq .
# Alternativa granular:
# curl -s -X POST "$BASE/v1/siat/cuis/$POSID" -H "X-API-Key: $API_KEY" -d '{}'
# curl -s -X POST "$BASE/v1/siat/cufd/$POSID" -H "X-API-Key: $API_KEY" -d '{}'

# 4.7. Sincronizar catálogos (sin ?operation = todas; con ?operation=tipoMoneda = una)
curl -s -X POST "$BASE/v1/siat/sincronizar/$POSID" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{}' | jq '.operations, .readiness'

# 4.8. Verificar readiness + insumos del primer payload
curl -s "$BASE/v1/companies/$CID/catalogs/readiness?point_of_sale_id=$POSID" \
  -H "X-API-Key: $API_KEY" | jq '.ready, .missing'
curl -s "$BASE/v1/companies/$CID/catalogs/emision-bootstrap?codigo_actividad=<COD>" \
  -H "X-API-Key: $API_KEY" | jq 'keys'
# Elegir: codigo_producto_sin + unidad_medida + documento sector (1) para el §5.
```

## 5. Primera factura (ruta estrella `POST /v1/invoices/emit`)

Payload mínimo **real** (snapshot completo — `sku` solo NO alcanza):

```bash
INVOICE_JSON=$(cat <<JSON
{"point_of_sale_id":"$POSID",
 "customer":{"document_type":"ci","document_number":"1234567",
             "name":"Ada Lovelace","email":"ada@example.com"},
 "items":[{"sku":"PLAN-PRO","description":"Plan profesional",
           "codigo_actividad":"<COD>","codigo_producto_sin":5113100,
           "unidad_medida":58,"quantity":1,"price":150,"discount":0}],
 "invoice_type":"sale","sector":"auto","data":{}}
JSON
)
# 5.1. Preview (no persiste, valida gratis)
curl -s -X POST "$BASE/v1/invoices/preview" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d "$INVOICE_JSON" | jq .

# 5.2. Crear + emitir (síncrono; OFFLINE también es 200 si cae la red SIAT)
EMIT=$(curl -s -X POST "$BASE/v1/invoices/emit" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d "$INVOICE_JSON")
echo "$EMIT" | jq '{id, status, cuf}'
IID=$(echo "$EMIT" | jq -r .id)

# 5.3. Ciclo completo
curl -s "$BASE/v1/invoices/$IID" -H "X-API-Key: $API_KEY" | j
...[truncated 6154 chars]
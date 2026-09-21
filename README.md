# 👹 Supay

<div align="center">

### Plataforma Open Source para Facturación Electrónica en Bolivia 🇧🇴

**Multi-tenant · SIAT · SOAP · XML · CUIS · CUFD · Go · React · PostgreSQL · River**

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react&logoColor=black" alt="React"/>
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL"/>
  <img src="https://img.shields.io/badge/go--siat-v2.1.1-00ADD8?style=for-the-badge" alt="go-siat"/>
  <img src="https://img.shields.io/badge/License-SupayAPI%20v1.0-blue?style=for-the-badge" alt="SupayAPI License v1.0"/>
</p>

---

**Supay es una plataforma SaaS multi-tenant y self-hostable que abstrae toda la complejidad del SIAT (SIN) para que integres facturación electrónica moderna sin pelearte con SOAP, XML, firmas XAdES, CUIS/CUFD ni contingencias.**

</div>

---

## ✨ ¿Qué es Supay?

Supay no es solo un SDK. Es **una API + Dashboard + motor fiscal** pensada para que cada empresa (tenant) emita sus propias facturas ante el SIAT con sus propias credenciales, certificados y puntos de venta.

<table>
<tr>
<td width="50%">

### 😵 Sin Supay

* XML manual por sector
* SOAP + WSDLs del SIN
* Gestión de CUIS/CUFD por POS
* Firmas XAdES-BES con P12
* CUF + hash + gzip + Base64
* Eventos significativos / contingencia
* 51 sectores, cada uno con su XSD
* Catálogos SIN versionados
* Errores SIAT crípticos

</td>
<td width="50%">

### 😎 Con Supay

* `POST /v1/invoices/emit` con 6 campos
* Tipado, validación y `preview` sin SIAT
* CUIS/CUFD auto-renovados por POS
* P12 con cadena completa (AES-256)
* CUF, firma y compresión automáticos
* Emisión síncrona + fallback `OFFLINE` + paquetes
* 51 perfiles sectoriales vía `go-siat` v2
* Catálogos sincronizados por tenant
* Errores accionables con `code`/`field`/`acción`

</td>
</tr>
</table>

> **Modelo de producto:** cada tenant se registra con `POST /internal/companies` (bootstrap con `BACKEND_SECRET`), recibe su `X-API-Key` y desde ahí todo se aísla por `tenant_id`: sucursales, puntos de venta, certificados, clientes, productos, facturas, CUIS/CUFD y paquetes.

---

## 🏗️ Arquitectura

```mermaid
flowchart LR
    subgraph Client
      POS[Tu POS / ERP]
      Dash[Dashboard React]
    end
    POS -- "X-API-Key + POST /v1/invoices/emit" --> API
    Dash -- "X-API-Key" --> API

    subgraph API [Supay API - Go + Chi]
      Router[Router Chi + Middleware]
      UC[Usecases]
      FiscalPort{{"FiscalService (puerto)"}}
      Simplifier["InvoiceSimplifier<br/>(payload 6 campos → dominio)"]
    end

    subgraph Adapters
      SIAT["Adapter SIAT<br/>(go-siat v2 + 51 sectores)"]
      Sandbox["Sandbox (tests/CI)"]
      Storage{{"Storage PDF<br/>(none/local/R2)"}}
      River["River queue<br/>(PostgreSQL)"]
    end

    Router --> UC --> Simplifier --> FiscalPort
    FiscalPort --> SIAT --> SIN[(SIAT SOAP)]
    FiscalPort -.-> Sandbox
    UC --> River --> Worker
    Worker --> SIAT
    UC --> Storage

    subgraph DB [PostgreSQL]
      Tenants[(tenants / branches / pos)]
      Invoices[(invoices / items / events)]
      Creds[(cuis/cufd history<br/>certificates<br/>catalog_versions)]
    end
    UC --- DB
    SIAT --- Creds
```

**Principios clave (ver [`backend/PLAN.md`](./backend/PLAN.md)):**

* **Multi-tenancy por diseño** — toda query filtra por `tenant_id` resuelto desde `X-API-Key` (hash + scopes, revocable). Nunca se mezclan datos entre tenants.
* **SIAT como adaptador externo** — el dominio no importa `go-siat`. El puerto `FiscalService` (`internal/ports/fiscal.go`) es la única frontera; el adaptador (`internal/adapters/siat`) es reemplazable y hay un `sandbox` determinístico para tests/CI.
* **Emisión síncrona con contingencia oficial** — `POST /v1/invoices/emit` intenta emitir en el mismo request; ante timeout/caída genera `OFFLINE` con `codigoEmision=2`, firma local y `contingency_event_id` para envío posterior por paquete.
* **BD normalizada** — FK explícitas, `CHECK` constraints, `catalog_versions`/`catalog_items` versionados, `cufd_history`/`cuis_history`, `contingency_events`, `outbox` + River.

---

## 🚀 Características

### 🔌 Integración SIAT (vía `go-siat` v2.1.1)

* ✅ **51 perfiles sectoriales** (50 con builder + 1 sin builder - sector 33 con `archivo`/`hash`/`cuf` externo) — ver `GET /v1/invoices/sectores`
* ✅ **CUIS/CUFD automáticos** — renovación programada (~4h antes del vencimiento CUFD, diaria para CUIS) + validación antes de emitir
* ✅ **Firma XAdES-BES** con P12 legacy y moderno (AES-256/PBES2), selección de cert por cadena y normalización DER
* ✅ **CUF + XML + gzip + Base64 + hash** por `utils.CUF`/`CompressAndHash`
* ✅ **Sincronización de catálogos** versionados por tenant (productos SIN, actividades, leyendas, doc sectores, paramétricas)
* ✅ **Verificación de NIT**, **anulación**, **reversión**, **consulta de estado SIAT**
* ✅ **Paquetes y masiva** (`500` facturas/lote) + **compras**, **eventos significativos**, **documento de ajuste**
* ✅ **Contingencia automática** + certificados con alertas a 30/15/7 días y auto-transición a `EXPIRED`

### 🛠️ Backend

* **Go 1.25** + **Chi v5** + **clean architecture** (`domain` → `usecase` → `adapters`/`delivery`)
* **PostgreSQL 16** + **golang-migrate** + **GORM** + **Pgx v5**
* **River** (cola sobre PostgreSQL) — dispatcher outbox, workers con backoff, circuit breaker y rate limiting por tenant
* **Módulos HTTP autónomos** (`internal/delivery/http/modules/*`) bajo contrato `Module` + rutas versionadas `/v1`
* **Middlewares:** `TenantMiddleware` (resuelve `X-API-Key`), `LimitBody` (10MB), CORS, RequestID, Prometheus, `RateLimitIP`

### 🖥️ Frontend & Dashboard

* **Vite + React 19 + TypeScript 6 + React Router 7 + TanStack Query/Table**
* **Dashboard** como paquete reutilizable `@supay/dashboard` (`packages/dashboard`) — mismo núcleo para self-hosted y cloud
* Páginas: facturas (listar/crear/preview/emitir/anular), clientes, productos, empresa/sucursales/PDV, sectores, conexión SIAT, contingencia, lotes y compras
* UI con **shadcn + Tailwind 4 + Base UI**

### 🧪 Calidad y operación

* **Payload mínimo** — `POST /v1/invoices/preview|emit` resuelve `sku`→SIN, `customer` por identidad fiscal, infiere códigos y valida sectores sin tocar el SIAT
* **Errores que enseñan** — `code` + `field` + `sugerencias` + `acción` (`internal/delivery/http/errors.go`)
* **Observabilidad** — métricas Prometheus (`/metrics`), logs JSON con request ID y redacción de PII, dashboards Grafana provisionados (`backend/deploy/observability`)
* **Archivos fiscales** — XML y PDF en volumen local o bucket R2 privado, elegidos con `STORAGE_DRIVER`; descargas autenticadas hacen streaming y comprueban la empresa.
* **Tests** — unitarios + integración (PostgreSQL vía `TEST_DATABASE_URL`) + contrato sandbox SIAT + concurrencia/cola/rate limiting (`go test ./...`)

---

## 📁 Estructura del proyecto

```text
supay/
├── backend/                         # API Go (módulo github.com/brandsrx/supay)
│   ├── cmd/server/main.go           # bootstrap: config, DB, container, scheduler
│   ├── internal/
│   │   ├── app/                     # container, server, maintenance, reaper, storage
│   │   ├── config/                  # env → Config (SiatInfra, Storage, Queue, Maintenance)
│   │   ├── domain/                  # entidades puras (Invoice, Branch, Cufd, Catalog...)
│   │   ├── ports/                   # FiscalService y puertos del dominio
│   │   ├── adapters/siat/           # go-siat v2 (51 sectores, CUIS/CUFD, firma, paquetes)
│   │   ├── usecase/                 # lógica de aplicación (invoice_simplifier, emission...)
│   │   ├── delivery/http/           # router Chi, middleware, modules (company/branch/pos/invoice/siat...)
│   │   ├── repository/              # postgres (GORM) + storage/pdf
│   │   ├── crypto/ / pdf/ / emissionqueue/ / observability/
│   │   └── models/
│   ├── docs/api-v1.md               # contrato público /v1
│   ├── deploy/observability/        # prometheus.yml + dashboards Grafana
│   ├── Dockerfile                   # multi-stage Go 1.25
│   └── go.mod
│
├── frontend/                        # host Vite (consume @supay/dashboard)
│   └── src/pages/ + components/
│
├── packages/
│   ├── dashboard/                   # núcleo del dashboard (open source reutilizable)
│   └── ui/                          # primitives compartidos
│
├── docs/                            # docs generales del monorepo
├── docker-compose.yml               # profiles [selfhosted, cloud]
├── turbo.json + pnpm-workspace.yaml # orquestación frontend/packages
└── README.md
```

---

## ⚡ Inicio rápido

### Requisitos

* **Go 1.25+**, **Node 20+**, **pnpm 11.9+**, **PostgreSQL 16** (o Docker), **Docker** opcional

### 1️⃣ Clonar

```bash
git clone https://github.com/tu-org/supay.git
cd supay
```

### 2️⃣ Levantar PostgreSQL

El compose es **único** con dos perfiles. Para desarrollo local usa `selfhosted` (PDF en disco):

```bash
docker compose --profile selfhosted up -d
# cloud (R2 + DB gestionada):
# DEPLOYMENT_MODE=cloud docker compose --profile cloud up -d
```

### 3️⃣ Configurar entorno

```bash
cp backend/.env.example backend/.env   # si existe, o crea backend/.env
```

Variables mínimas (`backend/internal/config/config.go:93`):

```env
PORT=8081
DATABASE_URL=postgres://postgres:postgres@localhost:5432/supay?sslmode=disable
BACKEND_SECRET=un-secreto-largo-para-bootstrap-internal
ENCRYPTION_KEY=clave-32-bytes-base64-o-hex-para-AES-GCM
# SIAT infra (piloto por defecto)
SIAT_AMBIENTE=2
SIAT_CODIGO_SISTEMA=tu-codigo-sistema
SIAT_BASE_URL=https://pilotosiatservicios.impuestos.gob.bo/v2
# Storage / deploy
DEPLOYMENT_MODE=selfhosted   # selfhosted | cloud
STORAGE_DRIVER=local         # local | r2
STORAGE_LOCAL_PATH=./data/files
STORAGE_SIGNING_SECRET=un-secreto-aleatorio-de-al-menos-32-caracteres
STORAGE_PRESIGN_TTL=5m
# STORAGE_PUBLIC_URL=https://api.example.com (para enlaces locales absolutos)
# Si STORAGE_DRIVER=r2: R2_ACCOUNT_ID, R2_ACCESS_KEY_ID,
# R2_SECRET_ACCESS_KEY, R2_BUCKET; R2_ENDPOINT es opcional.
AUTO_MIGRATE=true
```

`GET /v1/invoices/{id}/xml` y `/pdf` requieren pertenecer a la empresa de la factura; una factura ajena devuelve 404. Los enlaces temporales locales apuntan a `/storage/download` y expiran por defecto en cinco minutos. R2 usa URLs prefirmadas de S3. No registre estas URLs en logs. El bucket R2 debe ser privado.

En Self-Hosted, `docker-compose.yml` monta el volumen `filedata` en `/app/data/files`. Respalde el volumen junto con PostgreSQL; por ejemplo, detenga la API y copie el contenido de `filedata` a su sistema de backups antes de reanudarla. Restaurar solo la base de datos deja metadatos sin archivos. Para probar el contrato S3 localmente: `docker compose --profile storage-test up -d minio minio-init` y después `MINIO_ENDPOINT=http://localhost:9000 MINIO_ACCESS_KEY=minioadmin MINIO_SECRET_KEY=minioadmin MINIO_BUCKET=supay-test go test ./internal/storage -run TestMinIO`. MinIO no reproduce todas las particularidades de R2. La prueba real solo se activa con `R2_INTEGRATION_TEST=1` y las variables `R2_*`.

Con Compose, edite `backend/.env` (que define `STORAGE_LOCAL_PATH=/app/data/files`); para R2 cambie allí `STORAGE_DRIVER=r2` y complete las credenciales. El servicio `api` no reemplaza esas variables.

> `ENCRYPTION_KEY` cifra tokens y P12 en reposo (AES-GCM). Sin ella la API arranca pero advierte y el multi-tenant seguro queda deshabilitado.

### 4️⃣ Migraciones

Con `AUTO_MIGRATE=true` se ejecutan al arrancar el server (`backend/cmd/server/main.go:17`). Manualmente:

```bash
# vía binario migrate o desde el container
go run ./cmd/server  # ejerce database.Migrate()
```

### 5️⃣ Backend

```bash
cd backend
go mod download
go run ./cmd/server
# → Servidor en http://localhost:8081  (health: /health , /v1/health, /metrics)
```

### 6️⃣ Frontend

```bash
pnpm install
pnpm dev              # turbo → frontend en http://localhost:5173
# o solo el host:
pnpm --filter frontend dev
```

### 7️⃣ Bootstrap del primer tenant

```bash
# Crear empresa + api key (protegido con BACKEND_SECRET)
curl -X POST http://localhost:8081/internal/companies \
  -H "X-Backend-Token: $BACKEND_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"nit":"123456789","business_name":"Mi Empresa","codigo_sistema":"...","ambiente":"PILOTO","codigo_modalidad":1}'

# Respuesta incluye X-API-Key (ej: sup_live_xxx) — úsala en todo lo siguiente:
#   curl -H "X-API-Key: sup_live_xxx" http://localhost:8081/v1/invoices
```

Flujo mínimo después del bootstrap: **crear sucursal → crear punto de venta → subir certificado P12 → sincronizar catálogos → solicitar CUIS/CUFD (`POST /v1/siat/...`) → emitir**.

---

## 📖 API v1 — contrato estable

Todas las rutas de negocio viven bajo `/v1` y requieren `X-API-Key`. `Idempotency-Key` (≤100 chars) es soportado en creaciones. Las rutas sin versión se mantienen temporalmente por compatibilidad.

Contrato completo en [`backend/docs/api-v1.md`](./backend/docs/api-v1.md) y detalle multisector en [`backend/docs/invoicing-sectors.md`](./backend/docs/invoicing-sectors.md).

### Payload mínimo (6 campos)

```json
POST /v1/invoices/preview | POST /v1/invoices | POST /v1/invoices/emit
{
  "point_of_sale_id": "5226da47-74da-4b27-a26d-892c62fa6d26",
  "customer": { "document_type": "ci", "document_number": "1234567", "name": "Ada Lovelace" },
  "items": [{ "sku": "PLAN-PRO", "quantity": 1, "price": 150 }],
  "invoice_type": "sale",
  "sector": "auto",
  "data": {}
}
```

* `point_of_sale_id`, `customer`, `items` obligatorios (en ajustes 24/29/47/48 se pueden omitir `items` para copiar líneas originales).
* `invoice_type` por defecto `sale`; `sector` por defecto `auto`; `data` y `items[].data` llevan los campos sectoriales validados contra `campos_datos_sector` del perfil.
* `GET /v1/invoices/sectores` expone los 51 perfiles con `soportado`, `emision_individual`/`emision_masiva` y el esquema de `data` (tipo, obligatoriedad, etiqueta). `soportado` = cobertura técnica del builder, no homologación.

### Endpoints principales

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/v1/invoices/preview` | Valida y resuelve sin persistir ni tocar SIAT |
| `POST` | `/v1/invoices` | Crea borrador (`201` / `200` idempotente) |
| `POST` | `/v1/invoices/emit` | Crea + emite síncrono (`ACCEPTED`/`OBSERVED` o `OFFLINE` con `contingency_event_id`) |
| `POST` | `/v1/invoices/{id}/emit` | Emite borrador existente |
| `GET` | `/v1/invoices/{id}` / `/xml` / `/siat-status` | Consulta, XML firmado y estado SIAT |
| `POST` | `/v1/invoices/{id}/annul` | Anulación |
| `GET` | `/v1/invoices/sectores` | Catálogo de sectores y campos |
| `POST` | `/v1/siat/masiva/{companyId}/{posId}` | Masiva (sector 30 solo por aquí) |
| `POST` | `/internal/companies` | Bootstrap tenant (requiere `X-Backend-Token`) |

Sectores, layouts (`nota_credito_debito` / `nota_fiscal_credito_debito` para 24), `reference_invoice_id` y `payment` se documentan en `api-v1.md`.

---

## 🔄 Flujo de emisión

```text
POST /v1/invoices/emit (payload mínimo)
        │
        ▼
InvoiceSimplifier → resuelve customer/producto, valida sector/data, calcula totales
        │
        ▼
Valida CUFD vigente → si falta, renueva antes de emitir
        │
        ▼
buildFacturaSDK → adapter sectorial → builder go-siat → SignXML (XAdES) → CUF
        │
        ├─► SIAT OK  →  ACCEPTED / OBSERVED (200 OK)
        │
        └─► timeout/caída → firma local codigoEmision=2 → OFFLINE + evento significativo
                            (reenvío posterior por paquete cuando vuelve la red)
```

---

## 🧪 Tests y validación

```bash
cd backend
go test ./... -coverprofile=/tmp/supay-cover.out && go tool cover -func=/tmp/supay-cover.out

# Integración con PostgreSQL (omitidos si no hay TEST_DATABASE_URL)
TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/supay_test?sslmode=disable go test ./...

# Frontend / dashboard
pnpm typecheck && pnpm lint && pnpm build
```

* Cobertura global actual ~41.9% (objetivo 70% — ver `PLAN.md` Fase 12).
* El adaptador SIAT tiene **sandbox** determinístico (`internal/adapters/siat/sandbox`) para CI sin credenciales reales.

---

## 📊 Estado del proyecto

Basado en [`backend/PLAN.md`](./backend/PLAN.md) (14 fases). Resumen honesto:

| Área | Estado | Notas |
|---|---|---|
| Multi-tenancy + API keys | 🟢 | `X-API-Key` por tenant, scopes, revocación |
| Dominio desacoplado de SIAT | 🟢 | `FiscalService` + `adapters/siat` + sandbox |
| BD normalizada + migraciones | 🟢 | FK/CHECK, `catalog_versions`, `cufd/cuis_history` |
| Emisión síncrona + contingencia | 🟢 | `POST /v1/invoices/emit` + fallback `OFFLINE` |
| Payload mínimo + `/v1` estable | 🟢 | `InvoiceSimplifier`, `preview`/`emit`, aliases |
| 51 sectores via `go-siat` | 🟢 | `GET /v1/invoices/sectores`, `data`/`items[].data`, sector 33 vía archivo externo |
| Renovación CUIS/CUFD + alertas cert | 🟢 | Scheduler liviano + webhook `CERTIFICATE_ALERT_WEBHOOK_URL` |
| River queue + rate limit + breaker | 🟢 | Outbox, workers, token bucket por tenant, métricas |
| Observabilidad | 🟢 | Prometheus `/metrics` + Grafana + logs JSON sin PII |
| Storage `none`/`local`/`r2` | 🟢 | `DEPLOYMENT_MODE` + `docker-compose` profiles |
| Dashboard React | 🟢 | `@supay/dashboard` + host `frontend` |
| Cobertura >70% / SDK TS | 🟡/🔴 | 41.9% hoy; SDK TypeScript pendiente (Fase 13) |
| Homologación SIN | 🔴 | Proyecto experimental — no homologado aún |

> ⚠️ **Importante:** Supay aún **no está homologado por el SIN**. Es software experimental en desarrollo activo.

---

## 🌱 Código abierto, con límites claros

El código es público y cualquier desarrollador boliviano puede:

* 📚 Aprender cómo funciona la integración con el SIAT
* 🏢 Integrar Supay dentro de su ERP, POS o inventario
* 🖥️ Desplegar su propia instancia **self-hosted**
* 🔌 Construir módulos y adaptadores
* 🤝 Contribuir al ecosistema boliviano

Lo único que la licencia **no permite** es tomar Supay (o un derivado) y ofrecerlo como **servicio/API de facturación que compita directamente con Supay**, sea hospedado por terceros o self-hosted por el cliente. Ver [📜 Licencia](#-licencia).

---

## 🤝 Contribuir

```bash
git checkout -b feature/nueva-funcionalidad
git commit -m "feat: agregar soporte CUFD"
git push origin feature/nueva-funcionalidad
# luego abre un Pull Request
```

> Al contribuir aceptas que tu aporte se distribuye bajo la [SupayAPI License v1.0](./LICENSE.md).

---

## 📚 Stack visual

<div align="center">

| Backend | Base de datos | Infra & Observabilidad | Frontend |
|---|---|---|---|
| ![Go](https://skillicons.dev/icons?i=go) ![Chi](https://img.shields.io/badge/Chi-v5-00ADD8) | ![Postgres](https://skillicons.dev/icons?i=postgres) | ![Docker](https://skillicons.dev/icons?i=docker) ![Prometheus](https://skillicons.dev/icons?i=prometheus) ![Grafana](https://skillicons.dev/icons?i=grafana) | ![React](https://skillicons.dev/icons?i=react) ![TypeScript](https://skillicons.dev/icons?i=ts) ![Vite](https://skillicons.dev/icons?i=vite) ![Tailwind](https://skillicons.dev/icons?i=tailwind) |

**Go 1.25 · Chi v5 · go-siat v2 · PostgreSQL 16 · River · GORM/Pgx · Prometheus/Grafana · React 19 · Vite · Tailwind 4**

</div>

---

## 📖 Documentación

* [`backend/docs/api-v1.md`](./backend/docs/api-v1.md) — contrato público `/v1`
* [`backend/docs/invoicing-sectors.md`](./backend/docs/invoicing-sectors.md) — sectores y ejemplos
* [`backend/docs/observability.md`](./backend/docs/observability.md) — métricas, logs y dashboards
* [`backend/docs/postman/siat_4_etapas.postman_collection.json`](./backend/docs/postman/siat_4_etapas.postman_collection.json) — colección Postman
* [`backend/PLAN.md`](./backend/PLAN.md) — plan maestro multi-tenant y roadmap por fases
* [`backend/ARCHITECURE.md`](./backend/ARCHITECURE.md) — análisis sectorial verificable (Supay V2)

---

## 📜 Licencia

Supay se distribuye bajo la **SupayAPI License v1.0**, inspirada en la Elastic License 2.0.

**Puedes:** usar, copiar, modificar, distribuir y usar comercialmente, incluso integrarlo en tu ERP/POS y vender ese producto (self-host incluido) y auto-alojarlo.

**No puedes:** ofrecer Supay como SaaS gestionado a terceros; distribuir Supay (o derivado) como tu propio producto/API de facturación competidor (hospedado o self-hosted por el cliente); ni remover avisos de copyright/licencia.

Texto completo en [`LICENSE.md`](./LICENSE.md).

```text
SupayAPI License v1.0 © 2026 Ramiro Brandon Mamani Quisbert
```

---

## 👹 ¿Por qué “Supay”?

En la cosmovisión andina, **Supay** es la fuerza del mundo subterráneo asociada a la transformación y el poder. El proyecto toma ese nombre porque busca **transformar una integración fiscal oscura y compleja en una experiencia de desarrollo moderna, clara y poderosa**.

---

<div align="center">

### 🇧🇴 Hecho en Bolivia para desarrolladores bolivianos

**Go · React · SIAT · Open Source**

⭐ **Si el proyecto te parece útil, dale una estrella.**

</div>

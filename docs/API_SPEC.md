# Supay API Specification

> **Base URL:** `http://localhost:8081` (default, env `PORT`)
> **Protocol:** HTTP / JSON (except certificate upload = `multipart/form-data`, PDF/XML download = binary)
> **Auth:** `X-API-Key` header on every request except `GET /health`
> **Stack:** Go + Chi + GORM + go-siat/v2, multi-tenant per `Company`, Bolivia SIAT (`UTC-4` La Paz)

This document is the single source of truth for a web frontend agent. All routes, headers, query params, request bodies and responses are derived from the backend source (`backend/internal/delivery/http/router.go`, handlers, use cases and domain models).

---

## 1. Global Conventions

### 1.1 Headers

| Header | Required | Description |
|---|---|---|
| `X-API-Key` | Yes (unless `API_KEY=""`) | Shared secret from env `API_KEY`. Missing/invalid → `401 UNAUTHORIZED` (`backend/internal/delivery/http/middleware.go:16`). |
| `X-Company-Id` | No | Optional tenant hint. Injected into `context` via `siat.WithCompanyID`. If omitted, `company_id` in body/query is used. No validation here; provider fails later if company missing. |
| `Authorization: Bearer <jwt>` | No | Future JWT with `company_id` claim. Currently parsed but not validated (`middleware.go:54`). Ignored if `X-Company-Id` present. |
| `Idempotency-Key` | No (invoices only) | `POST /invoices` header, max 100 chars (`invoice_handler.go:17`). Same `point_of_sale_id` + key returns existing invoice (replay, `200 OK`). Race returns `409` unique violation fallback. |
| `Content-Type` | Yes | `application/json` except `POST /companies/{id}/certificates` → `multipart/form-data`. |
| `Accept` | No | `application/json` except `GET /invoices/{id}/xml` → `application/xml`, `GET .../pdf` → `application/pdf` |

- Max body size: `10 MB` (`router.go:13` `maxBodyBytes = 10 << 20`), enforced via `http.MaxBytesReader`.
- Global timeout: `60s` (`middleware.Timeout`).
- Logging: `middleware.Logger` + `Recoverer`.

### 1.2 Error Envelope (single shape for all errors)

All non-2xx responses return (`errors.go:47`):

```json
{
  "error": {
    "code": "VALIDATION_ERROR | NOT_FOUND | CONFLICT | UNAUTHORIZED | SIAT_REJECTED | SIAT_UNAVAILABLE | INTERNAL",
    "message": "human readable",
    "invoice_id": "optional, only on emit/rejected flows",
    "details": [{ "code": 123, "message": "SIAT observation" }]
  }
}
```

**Taxonomy** (`errors.go:20-28`):

| Code | HTTP | When |
|---|---|---|
| `VALIDATION_ERROR` | `400` | Bad JSON, missing param, `BadRequestError` |
| `NOT_FOUND` | `404` | `NotFoundError` or `gorm.ErrRecordNotFound` |
| `CONFLICT` | `409` | `ConflictError` or `ErrCompanyNitConflict`, `ErrCustomerDocumentConflict`, `ErrBranchSucursalConflict`, etc. Missing CUFD, catalog readiness. |
| `UNAUTHORIZED` | `401` | `X-API-Key` mismatch |
| `SIAT_REJECTED` | `422` | `EmissionRejectedError` → `details[]` with SIAT `codigo`/`descripcion` |
| `SIAT_UNAVAILABLE` | `503` / `502` | `ErrSiatNoDisponible`, network/retryable, `goSiat.SiatError` |
| `INTERNAL` | `500` | Untyped error; message is generic `internal server error`, detail logged server-side only. |

### 1.3 List Envelope

Paginated lists use (`errors.go:174`):

```json
{
  "items": [],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

If `items` is `nil` → serialized as `[]`. `limit`/`offset` omitted when `0` (non-paginated lists like branches). Use `respondList`.

### 1.4 Date / Time

- Invoice `issue_date`: `time.Time` in `America/La_Paz` (`UTC-4`). Input accepts flexible formats (`usecase/invoice_usecase.go:168`): `RFC3339Nano`, `RFC3339`, `2006-01-02T15:04:05.000`, `2006-01-02T15:04:05`, `2006-01-02 15:04:05`, with `Asia?` → `siat.LaPaz`.
- Only allowed when `ALLOW_CUSTOM_ISSUE_DATE=true` (default `true` in `PILOTO` env, `false` prod). Otherwise `400`.
- `fechaHoraInicioEvento` / `fechaHoraFinEvento` → `YYYY-MM-DDTHH:mm:ss.SSS` (SIAT), parsed via `ParseFechaSiat`.

### 1.5 Environment Variables (infra)

| Var | Default | Notes |
|---|---|---|
| `PORT` | `8081` | HTTP listen |
| `API_KEY` | (required) | Empty disables auth |
| `SIAT_AMBIENTE` | `2` (piloto) | `1`=prod, `2`=piloto |
| `SIAT_MODALIDAD` | `1` (electronica) | `1`=electronica, `2`=computarizada; overridden per company/certificate |
| `SIAT_BASE_URL` | derived | `https://pilotosiatservicios.impuestos.gob.bo/v2` piloto |
| `ENCRYPTION_KEY` | (required prod) | AES-GCM 32 bytes base64, for P12/token encryption |
| `DEPLOYMENT_MODE` | `selfhosted` | `selfhosted`|`cloud` |
| `STORAGE_DRIVER` | `local`/`r2` | PDF/Cert storage |
| `ALLOW_CUSTOM_ISSUE_DATE` | piloto `true` | Dev offline simulation |

---

## 2. Entity Schemas (domain models)

### Company
```json
{
  "id": "uuid",
  "nit": "1020304050",
  "business_name": "Empresa SRL",
  "codigo_sistema": "ABC123",
  "ambiente": "PILOTO | PRODUCCION",
  "modalidad": 1,
  "municipio": "La Paz",
  "direccion": "Calle 123",
  "telefono": "2123456",
  "codigo_actividad": "620100",
  "pie_pagina": "Ley 453...",
  "usuario_siat": "SUPAY",
  "created_at": "2026-09-02T10:00:00-04:00",
  "updated_at": "..."
}
```

### Branch
```json
{
  "id": "uuid",
  "company_id": "uuid",
  "codigo_sucursal": 0,
  "name": "Casa Matriz",
  "address": "Av. ...",
  "active": true,
  "created_at": "..."
}
```

### PointOfSale
```json
{
  "id": "uuid",
  "company_id": "uuid",
  "codigo_sucursal": 0,
  "codigo_punto_venta": 1,
  "description": "POS 1",
  "cuis": "ABC...",
  "cuis_created_at": "...",
  "is_active": true,
  "siat_code": 1,
  "tipo_punto_venta": 1,
  "siat_transaccion": true,
  "created_at": "..."
}
```
`codigo_punto_venta` auto-increment `MAX+1` per company+sucursal under advisory lock if `0`.

### Customer (create-only, immutable after invoicing)
```json
{
  "id": "uuid",
  "company_id": "uuid",
  "document_type": "CI | CEX | PAS | NIT | OD",
  "document_number": "1234567",
  "complement": "1A",
  "email": "client@mail.com",
  "name": "Juan Perez",
  "codigo_cliente": "CI1234567",
  "created_at": "..."
}
```
Unique per `company_id + document_type + document_number`. After invoices exist → immutable (DB trigger).

### Product
```json
{
  "id": "uuid",
  "company_id": "uuid",
  "sku": "PROD-001",
  "name": "Producto A",
  "active": true,
  "mappings": [{
    "id": "uuid",
    "codigo_producto_sin": 101010,
    "codigo_actividad": "620100",
    "codigo_documento_sector": 1,
    "unidad_medida": 57,
    "is_default": true,
    "active": true
  }],
  "created_at": "...",
  "updated_at": "..."
}
```
Mappings must exist in synchronized SIAT catalogs and match `actividadesDocumentoSector`.

### Certificate (P12)
```json
{
  "id": "uuid",
  "company_id": "uuid",
  "name": "cert-abcd-20260902",
  "type": "P12",
  "status": "ACTIVE | EXPIRED | REVOKED | PENDING",
  "not_before": "...",
  "not_after": "...",
  "p12_storage_ref": "certs/<company>/<id>.p12.enc",
  "modalidad": 1,
  "ambiente": "PILOTO",
  "nit": "1020304050",
  "created_at": "..."
}
```
Encrypted at rest (AES-GCM). Never returns `EncryptedToken`/`EncryptedP12Password`.

### Invoice (DTO, `invoice_dto.go`)
```json
{
  "id": "uuid",
  "company_id": "uuid",
  "customer_id": "uuid",
  "point_of_sale_id": "uuid",
  "idempotency_key": "optional",
  "cufd_id": "uuid",
  "contingency_event_id": null,
  "ajusta_factura_id": null,
  "invoice_number": 1,
  "status": "PENDING|SENDING|SENT|ACCEPTED|REJECTED|OBSERVED|OFFLINE|CANCELLED",
  "cuf": "ABC...",
  "subtotal": 100.00,
  "discount": 0,
  "total": 100.00,
  "codigo_metodo_pago": 1,
  "codigo_moneda": 1,
  "tipo_cambio": 1,
  "codigo_documento_sector": 1,
  "codigo_tipo_factura": 1,
  "layout": "",
  "modalidad": 1,
  "nombre_estudiante": null,
  "periodo_facturado": null,
  "sector_data": {},
  "emission_type": "EN_LINEA",
  "issue_date": "2026-09-02T10:00:00-04:00",
  "siat_reception_code": null,
  "siat_mensajes": null,
  "customer": {
    "id": "uuid",
    "name": "Juan",
    "document_type": "CI",
    "document_number": "123",
    "complement": null
  },
  "items": [{
    "id": "uuid",
    "product_id": "uuid",
    "code": "PROD-001",
    "description": "Item A",
    "codigo_actividad": "620100",
    "codigo_producto_sin": "101010",
    "unit_code": 57,
    "quantity": 2,
    "unit_price": 50,
    "discount": 0,
    "subtotal": 100,
    "sector_data": {}
  }],
  "created_at": "...",
  "company": {},       // only if ?include=company
  "point_of_sale": {}, // only if ?include=point_of_sale
  "cufd_record": {},   // only if ?include=cufd
  "xml": "...",        // only if ?include=xml
  "xml_hash": "...",
  "archivo": "base64...", // only if ?include=archivo
  "hash_archivo": "sha256..."
}
```

---

## 3. Endpoint Reference

> All paths below are mounted under `router.go:45` group (API key + company injection). `GET /health` is outside group.

### 3.1 Health

**`GET /health`** — No auth
- **Response `200`:**
```json
{"status":"ok","message":"Supay API running"}
```

---

### 3.2 Setup (Orchestrator)

**`POST /setup`** — Auth required
- **Body:**
```json
{ "company_id": "uuid", "point_of_sale_id": "uuid" }
```
Both required. Idempotent: reuses existing `CUIS`, refreshes catalogs, ensures `CUFD`.
- **Response `200`:**
```json
{
  "cuis": { "codigo": "...", "fechaVigencia": "...", "transaccion": true },
  "cufd": { "id":"uuid", "cufd":"...", "codigoControl":"...", "validFrom":"...", "validTo":"..." },
  "operations": [{ "operation":"actividades","transaccion":true,"codigos":12,"status":"SUCCESS","rows_saved":12 }],
  "errors": [],
  "readiness": { "company_id":"...","point_of_sale_id":"...","ready":true, "missing":[] }
}
```
- **Errors:** `400` missing ids, `404` company/POS not found, `503` SIAT unavailable.

---

### 3.3 Companies

#### `POST /companies/`
- **Body `RegisterCompanyRequest` (`company_usecase.go:18`):**
```json
{
  "nit": "1020304050",
  "business_name": "Empresa SRL",
  "codigo_sistema": "ABC123",
  "ambiente": "PILOTO",
  "usuario_siat": "SUPAY",
  "municipio": "La Paz",
  "direccion": "Calle 123",
  "telefono": "2123456",
  "codigo_actividad": "620100",
  "pie_pagina": "Ley 453..."
}
```
`nit` + `business_name` required. `ambiente` defaults `PILOTO`, `usuario_siat` defaults `SUPAY`. Validates `PILOTO|PRODUCCION`.
- **Response `201`:** `Company` object
- **Errors:** `400` missing, `409` `ya existe una empresa registrada con este nit`, `500` on create

#### `GET /companies/?nit=...`
- **Query:** `nit` required
- **Response `200`:** `Company`
- **Errors:** `400` missing `nit`, `404` not found

#### `PATCH /companies/{id}`
- **Body `UpdateCompanyRequest` (all optional pointers):** same fields as create
- **Response `200`:** updated `Company`
- **Errors:** `400` invalid ambiente, `404` not found, `409` nit conflict

#### `DELETE /companies/{id}` (also `?id=` query compat)
- **Response `204`** no body
- **Errors:** `404` not found, `409` has dependencies (FK: POS, customers, invoices)

#### Certificates (nested under company)

**`POST /companies/{id}/certificates`** — `multipart/form-data` ONLY
- **Fields:**
  - `p12_file` (file, required, `.p12` or `.pfx`, ≤5 MB)
  - `token` or `token_delegado` (text, required) — SIAT delegated token, encrypted with `ENCRYPTION_KEY`
  - `p12_password` (text, optional)
  - `name` (text, optional, auto `cert-<shortId>-YYYYMMDD`)
  - `type` (text, default `P12`)
  - `modalidad` (text int `1`/`2`, optional)
  - `ambiente` (text `PILOTO`/`PRODUCCION`, optional)
- **Headers:** `Content-Type: multipart/form-data; boundary=...` + `X-API-Key`
- **Response `201`:**
```json
{"data": { "id":"uuid","company_id":"...","name":"...","type":"P12","status":"ACTIVE", ... }}
```
- **Errors:** `400` missing file/token, invalid extension, `409` missing `ENCRYPTION_KEY`, `500` encrypt/storage fail

**`GET /companies/{id}/certificates`** → `200 {"data": [Certificate]}`

**`GET /companies/{id}/certificates/active`** → `200 {"data": Certificate}` or `404`

**`DELETE /companies/{id}/certificates/{certId}`** → `204`; also deletes storage object.

---

### 3.4 Branches

#### `POST /branches/`
```json
{ "company_id": "uuid", "codigo_sucursal": 0, "name": "Casa Matriz", "address": "Av..." }
```
`company_id` + `name` required, `codigo_sucursal >=0`. Validates company exists, unique `company+sucursal`.
- **Response `201`:** `Branch`

#### `GET /branches/?company_id=uuid` → `200 {"items":[...],"total":n}` (non-paginated list)

#### `GET /branches/{id}` → `200 Branch` / `404`

#### `PUT /branches/{id}`
```json
{ "codigo_sucursal": 1, "name": "New", "address": "...", "active": true }
```
All optional.
- **Response `200`:** Branch

#### `DELETE /branches/{id}` → `204` / `404`

---

### 3.5 Points of Sale

#### `POST /point-of-sales/`
```json
{
  "company_id": "uuid",
  "codigo_sucursal": 0,
  "description": "Caja 1",
  "cuis": "optional",
  "is_active": true
}
```
`company_id`, `description` required. `codigo_punto_venta` auto `MAX+1` if not set.
- **Response `201`:** `PointOfSale`

#### `GET /point-of-sales/?company_id=uuid` → `200 {"items":[...]}`
#### `GET /point-of-sales/{id}` → `200 PointOfSale`
#### `PATCH /point-of-sales/{id}` — body `UpdatePointOfSaleRequest` (`codigo_sucursal`, `description`, `cuis`, `is_active` optional) → `200`
#### `DELETE /point-of-sales/{id}` → `204`

---

### 3.6 Customers

#### `POST /customers/`
```json
{
  "company_id": "uuid",
  "document_type": "CI",
  "document_number": "1234567",
  "complement": "1A",
  "name": "Juan Perez"
}
```
`company_id`, `document_type`, `document_number`, `name` required. `document_type in [CI,CEX,PAS,NIT,OD]`. Trims/uppercases.
- **Response `201`:** `Customer`
- **Errors:** `400` missing/invalid type, `404` company, `409` duplicate document

#### `GET /customers/?company_id=uuid` → `200 {"items":[...]}`
#### `GET /customers/{id}` → `200 Customer`

> Customers are resolved in invoice creation either by `customer_id` or inline `client_*` fields. If `client_document_number`/`client_name` not provided via `customer_id`, a new customer is auto-created/looked up via `GetByCompanyAndFiscalIdentity`.

---

### 3.7 Products

#### `POST /products/`
```json
{
  "company_id": "uuid",
  "sku": "PROD-001",
  "name": "Producto A",
  "mappings": [{
    "codigo_producto_sin": 101010,
    "codigo_actividad": "620100",
    "codigo_documento_sector": 1,
    "unidad_medida": 57,
    "is_default": true
  }]
}
```
`company_id`, `sku`, `name` required. Each mapping validated against SIAT synchronized catalogs (`sin_products`, `actividades`, `unidadMedida`, `actividadesDocumentoSector` compatibility). `codigo_producto_sin` etc >0.
- **Response `201`:** `Product` with `mappings`

#### `GET /products/?company_id=uuid` → `200 {"items":[...]}`
- **Errors:** `400` missing `company_id`

#### `POST /products/{id}/mappings?company_id=uuid`
```json
{
  "codigo_producto_sin": 101010,
  "codigo_actividad": "620100",
  "codigo_documento_sector": 1,
  "unidad_medida": 57,
  "is_default": false
}
```
Upserts mapping. Same validations as create.
- **Response `204`** no body
- **Errors:** `400` missing `company_id`, validation, sector incompatibility

---

### 3.8 Invoices

#### `POST /invoices/` — Create draft (optionally emit immediately)

**Headers:** `X-API-Key`, optional `Idempotency-Key: <100 chars>`, `Content-Type: application/json`, optional `X-Company-Id`

**Query:** `?include=xml,company,point_of_sale,cufd,archivo,all` (comma-separated, `all` enables all)

**Body `CreateInvoiceRequest` (`invoice_usecase.go:192`):**
```json
{
  "company_id": "uuid",
  "point_of_sale_id": "uuid",
  "customer_id": "uuid",
  "customer": { "document_type": "CI", "document_number": "123", "name": "Juan", "complement": "1A" },
  "receiver": { "document_type": 1, "document_number": "123", "complement": "1A", "name": "Juan", "email": "a@b.com" },
  "client_document_type": "CI",
  "client_document_number": "1234567",
  "client_name": "Juan Perez",
  "client_email": "juan@mail.com",
  "client_complement": "1A",
  "invoice_type": "sale | education | credit_note | debit_note",
  "codigo_metodo_pago": 1,
  "codigo_moneda": 1,
  "tipo_cambio": 1,
  "codigo_documento_sector": 1,
  "layout": "nota_credito_debito | nota_fiscal_credito_debito",
  "modalidad": 1,
  "codigo_tipo_factura": 1,
  "nombre_estudiante": "Juan",
  "periodo_facturado": "2026-08",
  "datos_sector": {},
  "referencia_factura_id": "uuid",
  "issue_date": "2026-09-02T10:00:00-04:00",
  "emit": false,
  "archivo": "base64...",
  "hash_archivo": "sha256...",
  "cuf": "optional sector 33",
  "items": [
    {
      "product_id": "uuid",
      "sku": "PROD-001",
      "code": "PROD-001",
      "description": "Item A",
      "codigo_actividad": "620100",
      "codigo_producto_sin": "101010",
      "unit_code": 57,
      "quantity": 2,
      "unit_price": 50,
      "discount": 0,
      "datos_sector": {}
    }
  ]
}
```

**Field rules:**
- `point_of_sale_id` required; if `company_id` empty → derived from POS. Validates POS belongs to company and `is_active`.
- Customer: priority `customer_id` > `client_document_*` inline > `customer` legacy > `receiver` legacy (int type → string). Uses `resolveCustomer` to find-or-create. `client_document_number` + `client_name` required if no `customer_id`.
- `items` non-empty; each `quantity>0`, `unitPrice>=0`, `discount>=0`, `subtotal = quantity*unitPrice - discount` rounded 2 decimals.
- Product mapping: if `product_id`/`sku` present, resolves via `productRepo` and freezes `codigo_actividad`, `codigo_producto_sin`, `unit_code`. Validates sector matches invoice `codigo_documento_sector`.
- `codigo_documento_sector`: if `0` → resolved via `invoice_type` + product mappings + `actividadesDocumentoSector` (fallback to company `codigo_actividad`). Fails if missing.
- `modalidad` defaults to company / config `SiatModalidad`; validated via `PerfilSector.ValidarModalidad`.
- `datos_sector` (header-level) + `items[].datos_sector` (detail-level) validated fail-fast against `SectorProfile.Campos` / `CamposDetalle` (type `string|int|float|fecha|json`, required flags, unknown keys rejected). See `GET /invoices/sectores` for schema.
- Ajuste sectors (`24,29,47,48`): `referencia_factura_id` required → must have `cuf` and `invoice_number`.
- If sector without builder (`33`): `archivo`, `hash_archivo`, `cuf` required.
- `issue_date` only when `ALLOW_CUSTOM_ISSUE_DATE=true`, else `400`.
- CUFD: auto-resolved lazy via `CredentialService.EnsureCufd` if configured; else requires active CUFD in DB.
- Catalog readiness: checks `syncState` (`actividades`, `productosServicios`, `actividadesDocumentoSector`, `unidadMedida`, `tipoMoneda`, `tipoMetodoPago`, `leyendasFactura` <24h) → `409` if missing, asks to `POST /siat/sincronizar`.

**Response:**
- `201 Created` (or `200 OK` if `Idempotency-Key` replay with existing)
- Body: `invoiceDTO` (see schema). If `emit:true` and status was `PENDING` → internally calls `Emit` (see next) and returns emitted invoice; on SIAT rejection → `422` with `invoice_id`.

**Errors:** `400` validation, `404` company/POS/customer, `409` inactive POS, missing CUFD/catalog, `422` SIAT rejected, `503` SIAT unavailable.

#### `GET /invoices/?point_of_sale_id=uuid&status=PENDING&from=2026-09-01T00:00:00-04:00&to=2026-09-02T23:59:59-04:00&limit=50&offset=0`
- `point_of_sale_id` required.
- `status` optional enum `PENDING|SENDING|...|CANCELLED` (`invoice.go:9`).
- `from`/`to` RFC3339, `from <= to`.
- `limit` default `50`, max `200`; `offset >=0`.
- **Response `200`:**
```json
{ "items": [Invoice], "total": 10, "limit": 50, "offset": 0 }
```
Lightweight items (no `xml`/`archivo`).

#### `GET /invoices/sectores?company_id=uuid`
- Catalog metadata for dynamic forms. No auth? Auth via group, so yes.
- **Response `200`:** `SectorDTO[]`
```json
[
  {
    "codigo": 1,
    "nombre": "Compra y Venta",
    "tipo_documento": 1,
    "operacion": "recepcion_factura",
    "fachada": "compra_venta",
    "layout": "",
    "modalidades": [1,2],
    "soportado": true,
    "tiene_builder": true,
    "requiere_archivo": false,
    "con_detalle": true,
    "detalle_unico": false,
    "monto_sujeto_iva_cero": false,
    "es_ajuste": false,
    "habilitado": true,
    "campos_datos_sector": [{ "json":"periodo_facturado","requerido":true,"tipo":"string","etiqueta":"Período facturado","ejemplo":"2026-08" }],
    "campos_datos_sector_detalle": []
  }
]
```
If `company_id` omitted → `habilitado` omitted. Else computed from `siat_actividad_doc_sector`.

#### `GET /invoices/{id}?include=...` → `200 InvoiceDTO`
#### `GET /invoices/{id}/xml` → `200 application/xml` with `Content-Disposition: attachment; filename="factura-{id}.xml"` — `409` if no xml yet.

#### `GET /invoices/{id}/pdf` (also `GET /siat/invoice/{invoiceId}/pdf`) — generates via `pdf.Service`
- **Response `200`:** `application/pdf` `attachment; filename="factura-{id}.pdf"`
- **Errors:** `404` not found, `500` generation error, `503` service not initialized

#### `POST /invoices/{id}/emit` → emits pending invoice to SIAT
- **Response `200`:** `InvoiceDTO` with `ACCEPTED`/`OBSERVED`/`REJECTED` etc.
- **Idempotency:** `ClaimForEmission` atomic `PENDING→SENDING`; stale `SENDING` reaper (5min interval, 10min stale) reverts to `PENDING`.
- **Errors:** `404`, `409` not pending, `422` `SIAT_REJECTED`, `503`.

#### `GET /invoices/{id}/siat-status` — verify SIAT reception code
- **Response `200`:** `InvoiceDTO` updated

#### `POST /invoices/{id}/annul`
```json
{ "codigo_motivo": 1 }
```
`codigo_motivo` SIAT annulment reason code.
- **Response `200`:** `InvoiceDTO` with `CANCELLED`
- Uses `ClaimStatus` atomic transition.

#### `POST /invoices/{id}/annul/revert` → `200 InvoiceDTO` (revert annulment)

---

### 3.9 SIAT Operations (all `POST`)

All under `/siat/...` need `companyId` + `pointOfSaleId` path params, validated via `LoadCompanyAndPointOfSale`. Require active `CUIS`/`CUFD` (error `409 CONFLICT` if missing). Resolve service per company via `SiatClientProvider` (multi-tenant cert).

#### `POST /siat/cuis/{companyId}/{pointOfSaleId}` → force new CUIS
- **Response `200`:**
```json
{ "success": true, "cuis": "ABC...", "fecha_vigencia": "2026-09-03 10:00:00" }
```

#### `POST /siat/cufd/{companyId}/{pointOfSaleId}` → force new CUFD
- **Response `200`:**
```json
{ "success": true, "data": { "cufd":"...", "fecha_vigencia":"2026-09-03 10:00:00", "codigo_control":"..." } }
```

#### `POST /siat/sincronizar/{companyId}/{pointOfSaleId}?operation=actividades`
- `operation` optional: if present → single catalog; else all `SincronizacionOperations` (actividades, productosServicios, actividadesDocumentoSector, unidadMedida, etc.)
- **Response `200`:**
```json
{
  "company": {...},
  "point_of_sale": {...},
  "operations": [{ "operation":"actividades","transaccion":true,"codigos":5,"status":"SUCCESS","rows_saved":5 }],
  "errors": []
}
```
- On single op failure → `500` with `FAILED` sync state. On all ops → `200` even with partial errors; `status` per op `SUCCESS|FAILED|EMPTY`.

#### `POST /siat/evento-significativo/{companyId}/{pointOfSaleId}`
```json
{
  "codigo_motivo_evento": 1,
  "descripcion": "Corte del servicio de internet",
  "cufd_evento": "optional_override",
  "fecha_hora_inicio_evento": "2026-09-02T10:00:00.000",
  "fecha_hora_fin_evento": "2026-09-02T12:00:00.000"
}
```
`codigo_motivo_evento` defaults `1` (internet). `descripcion` defaults `"Corte..."`. If both dates empty → auto `now-10m → now+1h50m` (holgada window, clamped to CUFD validity). If one empty → `400`.
- **Response `200`:**
```json
{ "success": true, "codigo_recepcion": "12345" }
```
Persists `ContingencyEvent` with `siat_event_code`.

#### `POST /siat/firma/{companyId}/{pointOfSaleId}`
```json
{ "xml": "<factura ...>...</factura>" }
```
- **Response `200`:** `{ "company":..., "point_of_sale":..., "response": { "xmlFirmado":"...", ... } }`

#### `POST /siat/paquete/{companyId}/{pointOfSaleId}`
Dual input shape (custom `UnmarshalJSON`):
```json
{
  "codigoEvento": 12345,
  "descripcion": "Paquete contingencia",
  "codigoEmision": 2,
  "archivo": "optional_base64",
  "hashArchivo": "optional_sha256",
  "facturas": ["invoiceId1", "invoiceId2"]
}
```
or
```json
{
  "codigo_evento": 12345,
  "codigo_emision": 2,
  "facturas": [ { "codigoAmbiente":1, "nit":"...", "items":[...] } ]
}
```
- `facturas` can be `[]string` IDs (contingency flow) or `[]SolicitudFactura` objects or single string. Max `500` per package (`siat.MaxFacturasPorPaquete`).
- If IDs → loads from DB, builds `SolicitudFactura` via `solicitudDesdeInvoice` (resolves leyenda, cliente, CUF, totals).
- `codigoEvento` if `0` → auto resolves latest persisted `ContingencyEvent` `siat_event_code`; if still `0` → `400`.
- Dates auto-aligned to event window in memory (clamped to CUFD).
- **Response `200`:** `{ "company":..., "point_of_sale":..., "response": { "codigoRecepcion":"...", "transaccion":true, ... } }`
- Persists `SentPackage` (`type=PAQUETE`).

#### `POST /siat/paquete/{companyId}/{pointOfSaleId}/validar`
```json
{ "codigo_recepcion": "123", "codigo_emision": 2, "codigo_documento_sector": 1, "codigo_tipo_factura": 1 }
```
- Docs `PaqueteValidacionInput` has synonyms `codigoRecepcion|codigo_recepcion` etc via custom unmarshal.
- **Response `200`:** same `PaqueteResultado`

#### `POST /siat/masiva/{companyId}/{pointOfSaleId}`
```json
{
  "codigoEmision": 3,
  "facturas": ["invoiceId1", "invoiceId2"]
}
```
Same dual shape as paquete but `codigoEmision` typically `3` (masiva). Max `500`.
- **Response `200`:** `SentPackage` type `MASIVA`

#### `POST /siat/masiva/{companyId}/{pointOfSaleId}/validar` — same as paquete validar

#### `POST /siat/compras/{companyId}/{pointOfSaleId}`
```json
{
  "descripcion": "Compras periodo",
  "tipoCompra": 1,
  "archivo": "base64 tar.gz",
  "hashArchivo": "sha256",
  "cantidadFacturas": 10,
  "gestion": 2026,
  "periodo": 8,
  "fechaEnvio": "2026-09-02T10:00:00-04:00"
}
```
`archivo` + `hashArchivo` required. `codigo_punto_venta=0` in compras service.
- **Response `200`:** `{ "company":..., "point_of_sale":..., "response": {...} }`

#### `POST /siat/documento-ajuste/{companyId}/{pointOfSaleId}` — deprecated, logs warn, same as paquete for notas
```json
{ "codigoEmision":1, ... }
```

---

### 3.10 Catalogs

#### `GET /companies/{id}/catalogs/readiness` → Catalog readiness per POS
```json
{ "ready": true, "missing": [], "states": [{ "operation":"actividades","status":"SUCCESS","syncedAt":"..." }] }
```
Requires `syncStateRepo` <24h for critical catalogs.

#### `GET /companies/{id}/catalogs/actividades-economicas?tipo_actividad=...`
#### `GET /companies/{id}/catalogs/documentos-sector?codigo_actividad=...`
#### `GET /companies/{id}/catalogs/leyendas-factura?codigo_actividad=...`
#### `GET /companies/{id}/catalogs/productos-sin?codigo_actividad=123&q=search&limit=50&offset=0`
- `q` alias `query`, `codigo_actividad` parsed via `ParseCodigoActividadInt64`
#### `GET /companies/{id}/catalogs/emision-bootstrap?codigo_actividad=...` → combined bootstrap data
#### `GET /companies/{id}/catalogs/{catalogSlug}` → parametric (e.g., `tipos-moneda`, `metodos-pago`) via `ListParametricCatalog`
#### `GET /companies/{id}/actividades-economicas` → legacy alias

#### `GET /catalogs/perfiles-documento-sector` → all `SectorProfile` metadata (non-company, static)
```json
{ "items": [{ "codigo_documento_sector":1, "nombre":"Compra y Venta","tipo_factura_documento":1,"layout":"","soportado":true,"campos_extra":[{ "clave":"periodo_facturado","tipo":"string","requerido":true }] }], "total":52 }
```
#### `GET /catalogs/perfiles-documento-sector/{codigo}` → single profile or `404`
#### Legacy `GET /catalogs/*` (siat_handler legacy compatibility):
- `GET /catalogs/activites-document-sectors?company_id=&query=&limit=&offset=` (typo kept)
- `GET /catalogs/products?company_id=&query=&limit=&offset=`
- `GET /catalogs/readiness?company_id=&point_of_sale_id=`
- `GET /catalogs/{companyId}` → all catalogs grouped
- `GET /catalogs/{companyId}/{tipo}` → single catalog type (e.g., `productosServicios`)

---

### 3.11 Frontend Agent Implementation Guide

**Recommended creation flow in UI:**

1. **Create Company** → form with `nit*`, `business_name*`, `codigo_sistema`, `ambiente`, `municipio`, `direccion`, `codigo_actividad` (select from `GET /companies/{id}/catalogs/actividades-economicas` after initial save)
2. **Upload Certificate** → after company created, `multipart` to `POST /companies/{id}/certificates`
3. **Create Branch** → `POST /branches/` with `codigo_sucursal=0`
4. **Create POS** → `POST /point-of-sales/` with `codigo_sucursal=0`, `description`
5. **Setup** → single button `POST /setup` (idempotent) — show `readiness` progress
6. **Sync if needed** → `POST /siat/sincronizar/{companyId}/{posId}` on failure
7. **Create Customers** → `POST /customers/` or inline in invoice
8. **Products + Mappings** → need synchronized catalogs first; use `GET /companies/{id}/catalogs/productos-sin` search; then `POST /products/` with validated mapping
9. **Create Invoice** → fetch `GET /invoices/sectores?company_id=` to build dynamic `datos_sector` forms (each `CampoSector` has `tipo`, `requerido`, `etiqueta`, `ejemplo`). Validate client-side before submit. Use `Idempotency-Key` header for retry safety.
10. **Emit** → `POST /invoices/{id}/emit` or `emit:true`; poll `GET /invoices/{id}` or `.../siat-status`
11. **Download** → `GET /invoices/{id}/xml` (inline view) vs `GET /invoices/{id}/pdf` (generate)
12. **Contingency UI** → event form → `POST /siat/evento-significativo` → show `codigo_recepcion` → paquete/masiva with invoice IDs multi-select (limit 500), visualize `SentPackage` status.

**State handling:**
- Use `X-API-Key` from env/config, never store in localStorage plain? App handles it.
- Keep `company_id` + `point_of_sale_id` in app state; pass via body `company_id` and query `point_of_sale_id` for invoices.
- Handle `422 SIAT_REJECTED` → show `error.details[]` per SIAT code.
- Handle `409` readiness → prompt `Sincronizar` CTA.

**Pagination:** All lists expect `limit`/`offset` parsing; default to `50`. Use `total` for pagination controls.

**Error UX:** Map `code` to i18n keys, not `message` (message may change). Show `invoice_id` when present.

---

## 4. Quick cURL Examples

```bash
# health
curl http://localhost:8081/health

# create company
curl -X POST http://localhost:8081/companies/ \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d '{"nit":"1020304050","business_name":"Demo SRL","codigo_actividad":"620100"}'

# upload cert
curl -X POST http://localhost:8081/companies/$COMPANY_ID/certificates \
  -H "X-API-Key: $API_KEY" -F p12_file=@firma.p12 -F token=$SIAT_TOKEN -F p12_password=secret

# setup
curl -X POST http://localhost:8081/setup \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d '{"company_id":"'$COMPANY_ID'","point_of_sale_id":"'$POS_ID'"}'

# list sectores for form
curl http://localhost:8081/invoices/sectores?company_id=$COMPANY_ID -H "X-API-Key: $API_KEY"

# create invoice (compra venta)
curl -X POST "http://localhost:8081/invoices/?include=xml" \
  -H "X-API-Key: $API_KEY" -H "Idempotency-Key: order-123" \
  -H "Content-Type: application/json" -H "X-Company-Id: $COMPANY_ID" \
  -d '{
    "point_of_sale_id":"'$POS_ID'",
    "client_document_type":"CI","client_document_number":"1234567","client_name":"Juan Perez",
    "codigo_documento_sector":1,
    "items":[{"code":"PROD-001","description":"Servicio","quantity":1,"unit_price":100}]
  }'

# emit
curl -X POST http://localhost:8081/invoices/$INVOICE_ID/emit -H "X-API-Key: $API_KEY"

# download xml
curl http://localhost:8081/invoices/$INVOICE_ID/xml -H "X-API-Key: $API_KEY" -o factura.xml

# download pdf
curl http://localhost:8081/invoices/$INVOICE_ID/pdf -H "X-API-Key: $API_KEY" -o factura.pdf
```

---

## 5. Sector Reference (abbreviated)

52 sectors, 6 facades. Key supported (`soportado=true`): `1` CompraVenta, `8` TasaCero, `11` Educativo, `24` NotaCreditoDebito (2 layouts), `29` NotaConciliacion, `46` EducativoZF, `47` NotaDescuentos, `48` NotaICE. Experimental `51-55` and `33` (no builder) require `archivo`/`hash`/`cuf`.

Per-sector `datos_sector` fields: use `GET /invoices/sectores` as truth. Examples:
- `2` Alquiler: `periodo_facturado` (string, required)
- `11/46` Educativo: `nombre_estudiante` (string, required), `periodo_facturado` (required)
- `24/47/48` Notas: `numero_autorizacion_cuf`, `fecha_emision_factura`, `monto_total_original`, `monto_total_devuelto`, `monto_efectivo_credito_debito` (all required)
- Bulk `facturas` max `500` per `paquete`/`masiva`.

Full catalog: `backend/internal/siat/sectores_catalogo.go:17`.

---

## 6. Notes for Agent

- No OpenAPI generated yet; this spec is authoritative. Derive TypeScript types from domain schemas above.
- Always trim string inputs (`TrimSpace`) client-side.
- POS `is_active` must be `true` to create invoices.
- Customer email optional but persisted as `codigo_cliente = UPPER(type)+number`.
- Product `sku` is user-facing code; `code` in invoice item defaults to `product.SKU` if empty.
- `archivo`/`hash_archivo` only for sectores without builder; otherwise auto-generated via go-siat XML + `SignXML` + `CompressAndHash`.

*Last verified against:* `router.go:31`, `invoice_handler.go:40`, `siat_handler.go:37`, `errors.go:20`, `invoice_usecase.go:192`, `sectores.go:114`, `domain/*.go`, `config.go:57` on 2026-09-02.

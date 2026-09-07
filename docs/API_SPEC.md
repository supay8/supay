# Supay API — Flujo de Facturación

> **Base URL:** `http://localhost:8081` (env `PORT`)
> **Prefijo estable:** `/v1` (todas las rutas de este doc). Responde `X-API-Version: v1` (`backend/internal/delivery/http/router.go:98`)
> **Protocolo:** `HTTP / JSON`

Este documento cubre el flujo mínimo para que un agente configure el punto de venta y emita facturas: **Sucursal → Punto de Venta → CUIS → CUFD → Sincronización de Catálogos → Facturación**. No incluye `Company`, `Customer`, `Product` o `Certificate` aislados — se asume `company_id` y `X-API-Key` ya existen.

---

## 1. Convenciones Globales

### Headers

| Header | Requerido | Descripción |
|---|---|---|
| `X-API-Key` | Sí | `sup_<prefix>_<random>`. Falta → `401 {"error":{"code":"UNAUTHORIZED","message":"no autorizado: falta el header X-API-Key"}}` (`backend/internal/delivery/http/middleware.go:27`). Inválida/inactiva → `401` `API key inválida o inactiva`. Inyecta `company_id` al contexto; no existe `X-Company-Id`. |
| `Idempotency-Key` | No | Solo `POST /v1/invoices` y `POST /v1/invoices/emit`. Máx 100 chars (`backend/internal/delivery/http/modules/invoice/handler.go:54`). Mismo `point_of_sale_id` + key → replay `200 OK`. Sin key → `201 Created`. |
| `Content-Type` | Sí | `application/json` |

CORS (`backend/internal/delivery/http/router.go:68`): `AllowedOrigins` `localhost:3000`, `127.0.0.1:3000`, `0.0.0.0:3000`, `localhost:5173`, `127.0.0.1:5173`; `AllowedMethods` `GET, POST, PUT, DELETE, OPTIONS`; `AllowCredentials: true`; `MaxAge: 300`. Body máx `10 MB` (`router.go:15`), timeout global `60s`.

### Envelope de Errores

Todo `!2xx` retorna (`backend/internal/delivery/http/errors.go:47`):

```json
{
  "error": {
    "code": "VALIDATION_ERROR | NOT_FOUND | CONFLICT | UNAUTHORIZED | SIAT_REJECTED | SIAT_UNAVAILABLE | INTERNAL",
    "message": "texto humano",
    "field": "items[0].sku (solo VALIDATION_ERROR si aplica)",
    "details": [{ "code": 123, "message": "observación SIAT" }],
    "invoice_id": "uuid (solo en flujos de emisión rechazada)"
  }
}
```

| `code` | HTTP | Cuándo |
|---|---|---|
| `VALIDATION_ERROR` | `400` | JSON inválido, campo faltante, tipo incorrecto, campo desconocido (`DisallowUnknownFields`) |
| `UNAUTHORIZED` | `401` | `X-API-Key` falta/inválida |
| `NOT_FOUND` | `404` | Empresa/POS/sucursal/producto no encontrado |
| `CONFLICT` | `409` | `codigo_sucursal` duplicado, POS inactivo, sin CUIS/CUFD, catálogos no listos |
| `SIAT_REJECTED` | `422` | SIAT rechaza emisión |
| `SIAT_UNAVAILABLE` | `502/503` | SIAT no disponible / `ErrSiatNoDisponible` |
| `INTERNAL` | `500` | Error no tipado |

Programar contra `code`, no contra `message`.

### List Envelope

```json
{ "items": [], "total": 42, "limit": 50, "offset": 0 }
```
`limit`/`offset` omitidos cuando `0` (listas no paginadas como branches/POS).

---

## 2. Sucursal

### `POST /v1/branches/`

- **Handler:** `backend/internal/delivery/http/modules/branch/handler.go:20` → `backend/internal/usecase/branch_usecase.go:20` `CreateBranchRequest`
- **Headers:** `X-API-Key`
- **Body:**
```json
{ "company_id": "uuid*", "codigo_sucursal": 0, "name": "Casa Matriz*", "address": "Av. ..." }
```
`company_id`, `name` obligatorios; `codigo_sucursal >=0` (default `0`). Valida empresa existe, único `company_id + codigo_sucursal` → `409 ErrBranchSucursalConflict`.

- **Response `201`:** `Branch` `{id, company_id, codigo_sucursal, name, address, active:true, created_at}`

### `GET /v1/branches/?company_id=uuid`

- **Handler:** `branch/handler.go:34`
- **Query:** `company_id` (opcional, lista todo si vacío)
- **Response `200`:** `{"items":[Branch], "total":n}` (no paginado, `limit`/`offset` siempre `0`)

### `GET /v1/branches/{id}` → `200 Branch` / `404`
### `PUT /v1/branches/{id}` — Body `{"codigo_sucursal?":int, "name?":string, "address?":string, "active?":bool}` (`branch_usecase.go:27`) → `200 Branch`
### `DELETE /v1/branches/{id}` → `204` / `404`

---

## 3. Punto de Venta

### `POST /v1/point-of-sales/`

- **Handler:** `backend/internal/delivery/http/modules/pos/handler.go:21` → `backend/internal/usecase/point_of_sale_usecase.go:20` `RegisterPointOfSaleRequest`
- **Headers:** `X-API-Key`
- **Body:**
```json
{ "company_id": "uuid*", "codigo_sucursal": 0, "description": "Caja 1*", "cuis": null, "is_active": true }
```
`company_id`, `description` obligatorios. `codigo_sucursal` referencia sucursal (default `0`). `cuis` opcional. `codigo_punto_venta` auto `MAX+1` por `company+sucursal` bajo advisory lock si no se envía.

- **Response `201`:** `PointOfSale` `{id, company_id, codigo_sucursal, codigo_punto_venta, description, cuis, cuis_created_at, is_active, siat_code, tipo_punto_venta, siat_transaccion, created_at}`

### `GET /v1/point-of-sales/?company_id=uuid` → `200 {"items":[PointOfSale]}`
### `GET /v1/point-of-sales/{id}` → `200 PointOfSale` / `404`
### `PATCH /v1/point-of-sales/{id}` — Body `{"codigo_sucursal?":int, "description?":string, "cuis?":string, "is_active?":bool}` (`point_of_sale_usecase.go:28`) → `200`
### `DELETE /v1/point-of-sales/{id}` → `204` / `404`

---

## 4. CUIS — Código Único de Inicio de Sistema

### `POST /v1/siat/cuis/{companyId}/{pointOfSaleId}`

- **Handler:** `backend/internal/delivery/http/modules/siat/handler.go:59` `solicitarCUIS` → `backend/internal/usecase/siat_usecase.go:324` `SolicitarCUIS`
- **Headers:** `X-API-Key`
- **Path params:** `companyId*`, `pointOfSaleId*` validados vía `LoadCompanyAndPointOfSale` (`siat_usecase.go:268`) → `400` si vacíos, `404` si no existen, `409` si POS no pertenece a empresa.
- **Body:** vacío
- **Response `200`:**
```json
{ "success": true, "cuis": "ABC...", "fecha_vigencia": "2026-09-03 10:00:00" }
```
Fuerza renovación CUIS. Requiere servicio SIAT configurado, si no → `503 ErrSiatNoDisponible`.

---

## 5. CUFD — Código Único de Facturación Diaria

### `POST /v1/siat/cufd/{companyId}/{pointOfSaleId}`

- **Handler:** `siat/handler.go:72` `solicitarCUFD` → `siat_usecase.go:345` `SolicitarCUFD`
- **Headers:** `X-API-Key`
- **Path params:** mismos que CUIS
- **Body:** vacío
- **Response `200`:**
```json
{ "success": true, "data": { "cufd": "...", "fecha_vigencia": "2026-09-03 10:00:00", "codigo_control": "..." } }
```
Requiere CUIS activo previo → `409` si no. Formato `fecha_vigencia` `YYYY-MM-DD HH:mm:ss` (`siat/handler.go:82`).

---

## 6. Sincronización de Catálogos

### `POST /v1/siat/sincronizar/{companyId}/{pointOfSaleId}?operation=`

- **Handler:** `siat/handler.go:237` `sincronizar` → `siat_usecase.go:1273` `Sincronizar`
- **Headers:** `X-API-Key`
- **Path params:** `companyId*`, `pointOfSaleId*`
- **Query `operation` opcional:** si presente → solo esa operación; si vacío → todas (`ports.FiscalSyncOperations`: `actividades`, `productosServicios`, `actividadesDocumentoSector`, `unidadMedida`, `tipoMoneda`, `tipoMetodoPago`, `leyendasFactura`, etc.). Valor desconocido → `400`.
- **Body:** vacío
- **Requiere:** CUIS activo → `409` si no.
- **Response:**
  - **Single `?operation=X`:**
    ```json
    { "company": {}, "point_of_sale": {}, "operations": [{ "operation":"actividades", "transaccion":true, "codigos":5, "status":"SUCCESS|FAILED|EMPTY", "rows_saved":5, "fechaHora":"2026-09-03T10:00:00Z" }], "errors": [] }
    ```
    Si falla → `403 Forbidden` (`siat/handler.go:250` `!resumen.Success && err != nil`) con `FAILED` en sync state.
  - **All (sin `operation`):** Siempre `200` incluso con errores parciales; `status` por op `SUCCESS|FAILED|EMPTY`, `errors:[{operation,error}]` poblado (`siat_usecase.go:1318`).
  - `SUCCESS` si `transaccion:true` y `rows_saved>0` (o no-crítico); `EMPTY` si crítico (`actividades`, `productosServicios`, `actividadesDocumentoSector`, `unidadMedida`, `tipoMoneda`, `tipoMetodoPago`, `leyendasFactura`) y `rows_saved==0` (`siat_usecase.go:1381`).

### `POST /v1/setup` — Orquestador idempotente

- **Handler:** `siat/handler.go:217` `setup` → `siat_usecase.go:376` `Setup`
- **Headers:** `X-API-Key`
- **Body:**
```json
{ "company_id": "uuid*", "point_of_sale_id": "uuid*" }
```
Ambos obligatorios → `400` si falta. Idempotente: reutiliza CUIS vigente, ejecuta sincronización completa, asegura CUFD vigente.

- **Response `200`:**
```json
{
  "cuis": { "codigo":"...", "fechaVigencia":"..." },
  "cufd": { "id":"uuid", "cufd":"...", "codigoControl":"...", "validFrom":"...", "validTo":"..." },
  "operations": [{ "operation":"actividades", "transaccion":true, "codigos":12, "status":"SUCCESS", "rows_saved":12 }],
  "errors": [],
  "readiness": { "ready":true, "missing":[] }
}
```
`cuis` omitido si ya existía. `cufd` siempre resuelto vía `EnsureCufd` lazy. `readiness` desde `CatalogReadiness`.

### `GET` Lectura de catálogos (requieren `X-API-Key`)

| Método | Path | Descripción |
|---|---|---|
| `GET` | `/v1/companies/{id}/catalogs/readiness?point_of_sale_id=` | `CatalogReadiness` `{ready:bool, missing:[], states:[{operation,status,syncedAt}]}`. Requiere `<24h` para críticos (`siat/handler.go:278`). Handler `catalog/handler.go:32` |
| `GET` | `/v1/companies/{id}/catalogs/actividades-economicas?tipo_actividad=` | Actividades SIAT |
| `GET` | `/v1/companies/{id}/catalogs/documentos-sector?codigo_actividad=` | Documentos-sector por actividad |
| `GET` | `/v1/companies/{id}/catalogs/leyendas-factura?codigo_actividad=` | Leyendas |
| `GET` | `/v1/companies/{id}/catalogs/productos-sin?codigo_actividad=&q=&limit=50&offset=0` | Productos SIN; `q` alias `query`; `codigo_actividad` parseado vía `ParseCodigoActividadInt64` (`catalog/handler.go:81`) |
| `GET` | `/v1/companies/{id}/catalogs/emision-bootstrap?codigo_actividad=` | Bootstrap combinado para emisión |
| `GET` | `/v1/companies/{id}/catalogs/{catalogSlug}` | Paramétrico (`tipos-moneda`, `metodos-pago`, etc.) vía `ListParametricCatalog` (`catalog/handler.go:112`) |
| `GET` | `/v1/catalogs/perfiles-documento-sector` | Metadata estática 52 sectores (`siat.PerfilesSector()`) → `{items:[{codigo_documento_sector,nombre,tipo_factura_documento,layout,soportado,campos_extra:[{clave,tipo,requerido,etiqueta,ejemplo}]}], total:52}` (`catalog/handler.go:122`) |
| `GET` | `/v1/catalogs/perfiles-documento-sector/{codigo}` | Un perfil o `404` |

---

## 7. Facturación

### Contrato mínimo (6 campos)

`POST /v1/invoices`, `POST /v1/invoices/preview`, `POST /v1/invoices/emit` comparten mismo payload (`backend/internal/usecase/invoice_simplifier.go:18` `MinimalInvoiceRequest`):

```json
{
  "point_of_sale_id": "pos_123*",
  "customer": { "id": "uuid" } | { "document_type":"ci", "document_number":"1234567*", "name":"Ada Lovelace*", "email":"ada@example.com", "complement":"1A" },
  "items": [{ "sku":"PLAN-PRO*", "quantity":1, "price":150, "discount":0, "data":{} }],
  "invoice_type": "sale",
  "sector": "auto",
  "data": {}
}
```

Solo `point_of_sale_id`, `customer`, `items` obligatorios. `invoice_type` default `sale` (`sale|venta|compraventa` → `sale`, `education|educación` → `education` `invoice_simplifier.go:324`). `sector` default `auto` (`auto|sale|education` o código numérico `1`, `11` etc. `invoice_simplifier.go:336`). `data` cabecera + `items[].data` detalle validados contra `PerfilSector` (`invoice_simplifier.go:196`).

Aliases normalizados (acentos/guiones/espacios ignorados `invoice_simplifier.go:354`): `ci|cédula|cex|extranjero|pasaporte|pas|nit|otro|od`, también `1..5`. `customer`: `id` → reutiliza; si no, `document_type` default `CI`, `document_number` obligatorio, si cliente no existe `name` obligatorio (`invoice_simplifier.go:274`). Cada `sku` debe existir en catálogo, `quantity>0`, `price>=0`, `discount>=0 && <=quantity*price` (`invoice_simplifier.go:156`). Catálogo readiness exigido (`ensureCatalogReadiness` `invoice_simplifier.go:123`) → `409` si falta, `CUIS/CUFD` resuelto lazy vía `CredentialService.EnsureCufd`.

Reglas estrictas: `DisallowUnknownFields` (`invoice/handler.go:41`) → campo no declarado `400 VALIDATION_ERROR`; solo un objeto JSON → `400` si no (`handler.go:45`).

### `POST /v1/invoices/preview` — Validar sin crear

- **Handler:** `backend/internal/delivery/http/modules/invoice/handler.go:86` `previewV1` → `invoice_simplifier.go:363` `PreviewSimplified`
- **Headers:** `X-API-Key`
- **Body:** 6 campos
- **Response `200`:** `InvoicePreview` `{point_of_sale_id, customer:{id?,document_type,document_number,name}, items:[{product_id,sku,description,quantity,unit_price,discount,subtotal,codigo_actividad,codigo_producto_sin,unidad_medida,codigo_documento_sector}], invoice_type, codigo_documento_sector, codigo_metodo_pago:1, codigo_moneda:1, tipo_cambio:1, subtotal, total}`
- **Errores:** `400` validación, `404` POS/producto, `409` POS inactivo/catálogo

### `POST /v1/invoices` — Crear borrador

- **Handler:** `handler.go:62` `createV1` → `invoice_simplifier.go:371` `CreateSimplified`
- **Headers:** `X-API-Key`, opcional `Idempotency-Key`
- **Body:** 6 campos
- **Response:** `201 Created` sin key, `200 OK` replay con key; `Location: /v1/invoices/{id}` (`handler.go:82`); Body `Invoice` DTO (ver §8)
- **Errores:** `400`, `404`, `409`

### `POST /v1/invoices/emit` — Crear y emitir en un paso

- **Handler:** `handler.go:100` `emitV1` → `invoice_simplifier.go:380` `EmitSimplified` (crea y si `PENDING` → `Emit`)
- **Headers:** `X-API-Key`, opcional `Idempotency-Key`
- **Body:** 6 campos
- **Response:** `200 OK` + `Location` con el resultado síncrono (`ACCEPTED|OBSERVED|OFFLINE`); Body `Invoice` DTO. Un timeout o caída de red genera la factura offline y la vincula a un evento de contingencia local, que el registro posterior ante SIAT completa con su código de recepción.
- **Errores:** `422 SIAT_REJECTED` con `details` SIAT, `503` SIAT no disponible

### `POST /v1/invoices/{id}/emit` — Emitir borrador existente

- **Handler:** `handler.go:247` `emit`
- **Headers:** `X-API-Key`
- **Response `200 OK`** con `Invoice` DTO emitido o `OFFLINE`; `409` si no `PENDING`

### `GET /v1/invoices/{id}?include=` — Polling

- **Handler:** `handler.go:169` `getByID`
- **Query `include`:** `xml,company,point_of_sale,cufd,archivo,all` (coma-separado)
- **Response `200`:** `Invoice` DTO; `404` si no existe

### `GET /v1/invoices/{id}/xml` — XML firmado

- **Handler:** `handler.go:317` `downloadXML`
- **Headers:** `X-API-Key`
- **Response `200`:** `application/xml` `attachment; filename="factura-{id}.xml"`; `409` si sin XML

### `GET /v1/invoices/{id}/pdf` (alias `GET /v1/siat/invoice/{invoiceId}/pdf`)

- **Handler:** `siat/handler.go:334` `downloadPDF` → `pdf.Service`
- **Headers:** `X-API-Key`
- **Response `200`:** `application/pdf` `attachment; filename="factura-{id}.pdf"`; `404`, `503` si servicio no inicializado

---

## 8. Invoice DTO (respuesta)

```json
{
  "id": "uuid",
  "company_id": "uuid",
  "customer_id": "uuid",
  "point_of_sale_id": "uuid",
  "invoice_number": 1,
  "status": "PENDING|SENDING|SENT|ACCEPTED|REJECTED|OBSERVED|OFFLINE|CANCELLED",
  "cuf": "ABC...",
  "subtotal": 100, "total": 100,
  "codigo_documento_sector": 1, "codigo_metodo_pago": 1, "codigo_moneda": 1, "tipo_cambio": 1,
  "modalidad": 1, "emission_type": "EN_LINEA",
  "issue_date": "2026-09-02T10:00:00-04:00",
  "siat_reception_code": null, "siat_mensajes": null,
  "customer": { "id":"uuid", "name":"Juan", "document_type":"CI", "document_number":"123", "complement":null },
  "items": [{ "id":"uuid", "product_id":"uuid", "code":"PROD-001", "description":"Item A", "codigo_actividad":"620100", "codigo_producto_sin":"101010", "unit_code":57, "quantity":2, "unit_price":50, "discount":0, "subtotal":100, "sector_data":{} }],
  "created_at": "..."
}
```

Campos `xml`, `company`, `point_of_sale`, `cufd_record`, `archivo`, `hash_archivo` solo con `?include=`.

---

*Verificado contra:* `backend/internal/delivery/http/router.go:97`, `backend/internal/delivery/http/modules/branch/handler.go:20`, `backend/internal/delivery/http/modules/pos/handler.go:21`, `backend/internal/delivery/http/modules/siat/handler.go:59`, `backend/internal/delivery/http/modules/catalog/handler.go:32`, `backend/internal/delivery/http/modules/invoice/handler.go:38`, `backend/internal/usecase/branch_usecase.go:20`, `backend/internal/usecase/point_of_sale_usecase.go:20`, `backend/internal/usecase/invoice_simplifier.go:18`, `backend/internal/usecase/siat_usecase.go:268` el 2026-09-05.

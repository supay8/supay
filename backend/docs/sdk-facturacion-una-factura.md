# SDK — Emitir una sola factura (compra-venta, sector 1)

Guía mínima para el primer test del SDK. Solo compra-venta estándar (`sector: "auto"` → sector 1).
Fuera de alcance: borradores separados, emisión por ID, anulación, masiva/paquetes, ajustes (24/29/47/48),
sectores 30/33/52, CUIS/CUFD/sincronización manuales, CRUD de companies/POS/customers/products, certificates, auth JWT.

Fuentes: `internal/delivery/http/modules/invoice/module.go` (`RegisterV1Routes`),
`internal/delivery/http/modules/invoice/handler.go` (`previewV1`, `emitV1`),
`internal/usecase/invoice_simplifier.go` (`MinimalInvoiceRequest`),
`internal/delivery/http/errors.go` (envelope), `docs/api-v1.md`, colección
`docs/postman/supay_v1_completo.postman_collection.json` (sección `Invoices-v1`).

## 0. Precondiciones (el SDK las asume, no las crea)

- `baseUrl`: `http://localhost:8080` (o piloto).
- `X-API-Key`: key válida ligada a la empresa. Es el único auth de este flujo.
- `point_of_sale_id` (UUID): punto de venta con CUIS/CUFD vigente en ambiente `PILOTO`.
- 1 producto con `sku` mapeado al sector 1 (ej. `PLAN-PRO`).
- Cliente: se resuelve/crea solo con el payload (no requiere alta previa).

## 1. Auth y headers base

```http
X-API-Key: <api_key>
Content-Type: application/json
Idempotency-Key: <opcional, max 100 chars>   # solo en POST /emit
```

- Sin `X-API-Key` → `401 {"error":{"code":"UNAUTHORIZED",...}}`.
- `Idempotency-Key` repetida no re-emite: devuelve la factura ya terminada.
- Todos los POST de facturas usan `DisallowUnknownFields`: cualquier campo no declarado → `400 VALIDATION_ERROR`.

## 2. Payload mínimo (compartido por preview y emit)

```json
{
  "point_of_sale_id": "5226da47-74da-4b27-a26d-892c62fa6d26",
  "customer": {
    "document_type": "ci",
    "document_number": "1234567",
    "name": "Ada Lovelace",
    "email": "ada@example.com"
  },
  "items": [
    {
      "sku": "PLAN-PRO",
      "description": "Plan profesional",
      "codigo_actividad": "101010",
      "codigo_producto_sin": 5113100,
      "unidad_medida": 58,
      "quantity": 1,
      "price": 150,
      "discount": 0
    }
  ],
  "invoice_type": "sale",
  "sector": "auto",
  "data": {}
}
```

> Verificado contra código (`internal/usecase/invoice_simplifier.go:Simplify` + `invoice_usecase.go:Create`):
> el ítem NO se resuelve por `sku` desde el catálogo — `description`, `codigo_producto_sin`
> y `unidad_medida` son obligatorios en el payload y se congelan como snapshot fiscal.
> `customer.id` está prohibido (400); enviar snapshot completo. Los ejemplos de
> `docs/api-v1.md` y Postman que envían solo `sku/quantity/price` devuelven
> `400 "items[0].description es obligatorio"`.

Reglas:

- Obligatorios: `point_of_sale_id`, `customer.document_number`, `customer.name`, `items` (≥1).
- Defaults: `invoice_type: "sale"`, `sector: "auto"`, `document_type: "CI"`. Aliases: `sale|venta|compraventa|compra_venta`, `auto|sale|compraventa|1`.
- `customer`: `document_number` + `name` obligatorios; si el cliente ya existe se reutiliza.
- `items[]`: obligatorios `sku`, `description`, `codigo_producto_sin` (>0), `unidad_medida` (>0),
  `quantity` (>0), `price` (≥0); opcionales `discount` (≤ quantity*price), `data`.
  `codigo_actividad` puede omitirse solo si la empresa tiene actividad principal (se hereda).
- `payment` omisible (= método 1, moneda 1, tipo cambio 1). `data: {}` para sector 1.
- NO enviar en este test: `layout`, `reference_invoice_id`, `total`, `items[].data` (son para ajustes/sectores especiales).

## 3. Paso 0 — Smoke test

```sh
curl -s $BASE/v1/health
# {"status":"ok","message":"Supay API running"}
```

## 4. Paso 1 — `POST /v1/invoices/preview` (validar sin crear)

Valida contrato local + catálogos. No crea cliente ni factura, no toca SIAT, no acepta `Idempotency-Key`.

```sh
curl -s -X POST $BASE/v1/invoices/preview \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d '{"point_of_sale_id":"5226da47-74da-4b27-a26d-892c62fa6d26",
       "customer":{"document_type":"ci","document_number":"1234567","name":"Ada Lovelace","email":"ada@example.com"},
       "items":[{"sku":"PLAN-PRO","description":"Plan profesional","codigo_actividad":"101010","codigo_producto_sin":5113100,"unidad_medida":58,"quantity":1,"price":150,"discount":0}],
       "invoice_type":"sale","sector":"auto","data":{}}'
```

Respuesta `200` (forma `InvoicePreview`):

```json
{
  "point_of_sale_id": "5226da47-74da-4b27-a26d-892c62fa6d26",
  "customer": {"document_number": "1234567", "document_type": "CI", "name": "Ada Lovelace"},
  "items": [{"sku": "PLAN-PRO", "description": "Plan profesional", "quantity": 1, "unit_price": 150, "discount": 0, "subtotal": 150, "codigo_documento_sector": 1, "codigo_producto_sin": 5113100, "unidad_medida": 58}],
  "invoice_type": "sale",
  "codigo_documento_sector": 1,
  "codigo_metodo_pago": 1,
  "codigo_moneda": 1,
  "tipo_cambio": 1,
  "subtotal": 150,
  "total": 150
}
```

El test SDK debe assertar `codigo_documento_sector == 1` y `total == 150` antes de emitir.

## 5. Paso 2 — `POST /v1/invoices/emit` (crear + emitir, ruta estrella)

Crea el borrador e intenta emitir al SIAT dentro del mismo request.

```sh
curl -s -X POST $BASE/v1/invoices/emit \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -H "Idempotency-Key: sdk-test-0001" \
  -d '{"point_of_sale_id":"5226da47-74da-4b27-a26d-892c62fa6d26",
       "customer":{"document_type":"ci","document_number":"1234567","name":"Ada Lovelace","email":"ada@example.com"},
       "items":[{"sku":"PLAN-PRO","description":"Plan profesional","codigo_actividad":"101010","codigo_producto_sin":5113100,"unidad_medida":58,"quantity":1,"price":150,"discount":0}],
       "invoice_type":"sale","sector":"auto","data":{}}'
```

Respuesta `200` + header `Location: /v1/invoices/{id}` (cuerpo `invoiceDTO` resumido):

```json
{
  "id": "b61d67ed-7962-4989-958f-8ccc01979d64",
  "company_id": "<uuid>",
  "point_of_sale_id": "5226da47-74da-4b27-a26d-892c62fa6d26",
  "invoice_number": 1,
  "status": "ACCEPTED",
  "cuf": "ABCDEF...",
  "subtotal": 150,
  "total": 150,
  "codigo_documento_sector": 1,
  "customer": {"id": "<uuid>", "name": "Ada Lovelace", "document_type": "CI", "document_number": "1234567"},
  "items": [{"code": "PLAN-PRO", "quantity": 1, "unit_price": 150, "subtotal": 150}],
  "siat_reception_code": "abc-123",
  "created_at": "2026-09-21T00:00:00Z"
}
```

Estados posibles a manejar en el SDK:

| `status` | Significado | Acción SDK |
|---|---|---|
| `ACCEPTED` | SIAT aceptó | guardar `id`, `cuf`, `invoice_number` |
| `OBSERVED` | SIAT observó (ver `siat_mensajes`) | loguear observaciones, no re-emitir |
| `OFFLINE` | timeout/caída SIAT; trae `xml` firmado + `contingency_event_id` | guardar XML local, reenviar luego por paquete |

Quitar `Idempotency-Key` → el SDK debe igual funcionar (solo cambia que no hay replay seguro).

## 6. Paso 3 — `GET /v1/invoices/{id}` (verificar)

```sh
curl -s $BASE/v1/invoices/$INVOICE_ID -H "X-API-Key: $API_KEY"
# 200, mismo invoiceDTO. ?include=xml añade "xml" y "xml_hash".
```

Asserts del test: `id == emitido`, `status in {ACCEPTED, OBSERVED, OFFLINE}`, `cuf != null`, `total == 150`.

## 7. Paso 4 — `GET /v1/invoices/{id}/xml` (comprobante)

```sh
curl -s $BASE/v1/invoices/$INVOICE_ID/xml -H "X-API-Key: $API_KEY" -o factura.xml
# 200 Content-Type: application/xml, attachment factura-{id}.xml
# Solo emitidas; pendiente → 409 CONFLICT.
```

## 8. Errores (programar contra `code`)

Envelope único:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "el sku es obligatorio",
    "field": "items[].sku",
    "sugerencias": ["Revise el tipo, formato y valor del campo indicado.", "Use POST /v1/invoices/preview para validar una factura antes de emitirla."],
    "accion": "Corrija el payload y vuelva a enviar la solicitud."
  }
}
```

| Caso | HTTP | `code` |
|---|---|---|
| campo faltante/tipo mal, `sector: "30"`, JSON con campos extra | 400 | `VALIDATION_ERROR` |
| sin key / key inválida | 401 | `UNAUTHORIZED` |
| factura de otra empresa | 404 | `NOT_FOUND` |
| SIAT rechaza documento (`details[]` con observaciones) | 422 | `SIAT_REJECTED` |
| SIAT caído/timeout en consulta directa | 503 | `SIAT_UNAVAILABLE` |

`field` cubre `point_of_sale_id`, `customer.*`, `items[].sku|quantity|price|discount`, `sector`, `invoice_type`, `payment`, `data`.

## 9. Checklist del primer test SDK

1. `GET /v1/health` → 200.
2. `POST /v1/invoices/preview` con payload §2 → 200, `codigo_documento_sector == 1`.
3. `POST /v1/invoices/emit` + `Idempotency-Key: sdk-test-0001` → 200, guardar `id` del `Location`/cuerpo.
4. `GET /v1/invoices/{id}` → `status` y `cuf` presentes.
5. `GET /v1/invoices/{id}/xml` → XML no vacío.
6. Reintentar `emit` con misma key → misma factura, sin duplicado.

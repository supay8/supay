# Supay API v1

`/v1` es el contrato público estable de Supay. Los cambios incompatibles se
publicarán en una nueva versión de ruta. Las rutas históricas sin versión se
mantienen temporalmente por compatibilidad, pero no forman parte del contrato
estable.

Todas las rutas de negocio requieren `X-API-Key`. Las operaciones de creación
aceptan `Idempotency-Key` (máximo 100 caracteres).

## Facturas: contrato mínimo

`POST /v1/invoices`, `POST /v1/invoices/preview` y
`POST /v1/invoices/emit` comparten el mismo payload de hasta seis campos de
primer nivel:

```json
{
  "point_of_sale_id": "pos_123",
  "customer": {
    "document_type": "ci",
    "document_number": "1234567",
    "name": "Ada Lovelace",
    "email": "ada@example.com"
  },
  "items": [
    {
      "sku": "PLAN-PRO",
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

Solo `point_of_sale_id`, `customer` e `items` son obligatorios. `invoice_type`
usa `sale` por defecto, `sector` usa `auto` y `data` contiene los campos
adicionales exigidos por sectores especializados. Cada ítem puede incluir su
propio objeto `data`.

El cliente puede enviarse por `customer.id`. Si se envía su identidad fiscal,
Supay reutiliza el cliente existente; `name` solo es obligatorio cuando debe
registrarse uno nuevo. Cada producto se identifica por `sku`: nombre, código
interno, actividad económica, producto SIN, unidad de medida y documento-sector
se obtienen del catálogo vigente del tenant.

Aliases aceptados:

- Documento: `ci`, `cédula`, `cex`, `extranjero`, `pasaporte`, `nit`, `otro`.
- Tipo de factura: `sale`, `venta`, `compraventa`, `education`, `educación`.
- Sector: `auto`, `sale`, `compraventa`, `education`, `educación`, o cualquier
  código numérico SIAT soportado.

Los campos no declarados se rechazan para evitar que un error tipográfico se
ignore silenciosamente.

## Endpoints

### `POST /v1/invoices/preview`

Valida y resuelve el payload sin crear cliente ni factura. Devuelve el cliente y
los productos encontrados, los códigos SIAT inferidos, subtotales y total.

### `POST /v1/invoices`

Crea un borrador. Devuelve `201 Created` sin `Idempotency-Key` y `200 OK` al
usar una clave idempotente. `Location` apunta a `/v1/invoices/{id}`.

### `POST /v1/invoices/emit`

Crea el borrador e intenta emitirlo al SIAT dentro del mismo request. Devuelve
`200 OK` con la factura `ACCEPTED`/`OBSERVED`; si el SIAT no responde por timeout
o caída de red, devuelve `200 OK` con la factura `OFFLINE`, su XML firmado y el
`contingency_event_id` local que permitirá enviarla posteriormente por paquete.
Una repetición idempotente no vuelve a emitir una factura ya terminada.
Al restablecerse la conexión, el registro del evento significativo completa ese
mismo evento local con su fecha de fin y el código de recepción del SIAT.

### Compatibilidad

Las operaciones existentes de consulta, XML, estado SIAT, anulación y emisión
por ID están disponibles con el prefijo `/v1`, por ejemplo:

- `GET /v1/invoices/{id}`
- `GET /v1/invoices/{id}/xml`
- `POST /v1/invoices/{id}/emit`
- `GET /v1/invoices/{id}/siat-status`
- `POST /v1/invoices/{id}/annul`

Los demás módulos registrados también están disponibles bajo `/v1`.

## Errores accionables

Las respuestas de error conservan un envelope estable. `field` se incluye
cuando Supay puede identificar el dato que debe corregirse:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "el sku es obligatorio",
    "field": "items[].sku",
    "sugerencias": [
      "Revise el tipo, formato y valor del campo indicado.",
      "Use POST /v1/invoices/preview para validar una factura antes de emitirla."
    ],
    "accion": "Corrija el payload y vuelva a enviar la solicitud."
  }
}
```

Los clientes deben programar contra `code`; `message`, `sugerencias` y
`accion` son texto explicativo y pueden evolucionar sin romper el contrato.

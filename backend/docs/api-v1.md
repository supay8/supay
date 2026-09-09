# Supay API v1

`/v1` es el contrato público estable de Supay. Los cambios incompatibles se
publicarán en una nueva versión de ruta. Las rutas históricas sin versión se
mantienen temporalmente por compatibilidad, pero no forman parte del contrato
estable.

Todas las rutas de negocio requieren `X-API-Key`. Las operaciones de creación
aceptan `Idempotency-Key` (máximo 100 caracteres).

## Facturas: contrato mínimo

`POST /v1/invoices`, `POST /v1/invoices/preview` y
`POST /v1/invoices/emit` comparten el mismo payload simplificado. Los seis
campos originales siguen funcionando; las opciones sectoriales son aditivas:

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

Opciones adicionales:

- `layout`: obligatorio para el sector 24; `nota_credito_debito` o
  `nota_fiscal_credito_debito`.
- `reference_invoice_id`: factura original para los ajustes 24, 29, 47 y 48.
  Debe pertenecer a la misma empresa y cliente y tener CUF y número de factura.
- `payment`: objeto con `method_code`, `currency_code` y `exchange_rate`, todos
  mayores a cero. Si se omite, se usan 1, 1 y 1.
- `total`: importe para el sector 30 cuando no se envían `items`. No se permite
  sobrescribir con él el total calculado de un documento con detalle.

En ajustes se pueden omitir `items` para copiar las líneas originales; en el
sector 29 los montos de conciliación por línea se deben proporcionar mediante
`items[].data`. Los demás sectores con detalle requieren al menos un ítem;
los prevalorados 23 y 36 requieren exactamente uno.

El cliente puede enviarse por `customer.id`. Si se envía su identidad fiscal,
Supay reutiliza el cliente existente; `name` solo es obligatorio cuando debe
registrarse uno nuevo. Cada producto se identifica por `sku`: nombre, código
interno, actividad económica, producto SIN, unidad de medida y documento-sector
se obtienen del catálogo vigente del tenant.

En documentos de ajuste, los códigos fiscales y la descripción se obtienen de
las líneas de la factura original. No se necesita un mapeo adicional del
producto al sector de la nota. Un SKU que no aparece en la factura referenciada
se rechaza. No duplique líneas para representar original/devolución: el backend
construye ambos grupos; las líneas repetidas del payload se conservan.

Aliases aceptados:

- Documento: `ci`, `cédula`, `cex`, `extranjero`, `pasaporte`, `nit`, `otro`.
- Tipo de factura: `sale`, `venta`, `compraventa`, `education`, `educación`,
  `credit_note`, `nota_credito`, `debit_note`, `nota_debito`. Las notas sin sector
  explícito seleccionan el 24 y requieren `layout`.
- Sector: `auto`, `sale`, `compraventa`, `education`, `educación`, o cualquier
  código numérico SIAT soportado.

Los campos no declarados se rechazan para evitar que un error tipográfico se
ignore silenciosamente.

## Endpoints

### `POST /v1/invoices/preview`

Valida y resuelve el payload sin crear cliente ni factura. Devuelve el cliente y
los productos encontrados, los códigos SIAT inferidos, subtotales y total.
Incluye `layout`, `reference_invoice_id` y los `data` de cabecera normalizados.
Los campos sectoriales faltantes o incompatibles producen `VALIDATION_ERROR`
(HTTP 400), sin ocultarlos como `INTERNAL`.

La vista previa verifica el contrato local y los catálogos disponibles. No
contacta al SIAT ni confirma la habilitación/homologación tributaria.

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

El sector 30 no admite emisión individual: use
`POST /v1/siat/masiva/{companyId}/{pointOfSaleId}` y su operación de validación.
Intentar emitirlo por `/v1/invoices/emit` o por ID devuelve una validación local.

### Sectores y campos disponibles

`GET /v1/invoices/sectores` devuelve los perfiles de la versión instalada de
`go-siat`, incluyendo `campos_datos_sector` para `data` y
`campos_datos_sector_detalle` para `items[].data`. Cada campo incluye tipo,
obligatoriedad y etiqueta. El esquema se completa con los setters del SDK; no
es necesario añadir manualmente cada campo a un DTO distinto por sector.

`soportado` significa cobertura técnica del constructor, no homologación.
`emision_individual` y `emision_masiva` indican los canales posibles.
Con `?company_id=UUID`, `habilitado` indica lo registrado en el catálogo
sincronizado de actividades/documentos de esa empresa.

Hay 50 códigos con constructor y 51 perfiles (dos layouts del sector 24).
El 33 sigue requiriendo XML externo mediante `/invoices` con `archivo`,
`hash_archivo` y `cuf`: el SDK instalado no tiene constructor para él.
El 52 solo admite modalidad electrónica. No se sustituyen sectores entre sí.

Ejemplos y límites: [Integración multisector](invoicing-sectors.md).

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

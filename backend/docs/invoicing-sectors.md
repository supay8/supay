# Facturación multisector con go-siat

La integración usa `github.com/ron86i/go-siat/v2 v2.1.1`. Fuentes revisadas:

- [Código y documentación del SDK](https://github.com/ron86i/go-siat/tree/v2.1.1).
- [Sectores, modalidades y fachadas](https://github.com/ron86i/go-siat/blob/v2.1.1/docs/es/explanation/sectores.md).
- [Emisión y documentos de ajuste](https://github.com/ron86i/go-siat/blob/v2.1.1/docs/es/how-to/envio-facturas.md).

## Elegir el perfil

Consulte `GET /v1/invoices/sectores?company_id=UUID`. Use su `codigo` en
`sector` (string) y su `layout` cuando corresponda. `data` y `items[].data`
aceptan exclusivamente las claves publicadas para ese perfil.

El registro cubre los 51 constructores del SDK (50 códigos, dos variantes del
24). Las pruebas contrastan las raíces XML con el catálogo del SDK, ejercitan
los campos opcionales y obligatorios, y verifican firma y transporte SOAP contra
un servidor local. Estas pruebas **no son homologación ante el SIAT**.
La obligatoriedad local combina las declaraciones sectoriales existentes con
los tipos no nulos de los setters del SDK; las reglas tributarias condicionales
y los valores de los catálogos todavía están sujetos a validación del SIAT.

| Caso | Tratamiento |
| --- | --- |
| 24 | Seleccionar `nota_credito_debito` o `nota_fiscal_credito_debito`. |
| 24, 29, 47, 48 | `reference_invoice_id` obligatorio; servicio DocumentoAjuste, tipo 3. |
| 29 | Se construyen `detalleOriginal` y `detalleConciliacion` con métodos separados del SDK. |
| 24, 47, 48 | Se conservan los importes originales y los devueltos en grupos distintos. |
| 23, 36 | Exactamente una línea. |
| 30 | Sin detalle XML, emisión masiva; tipo de documento 2. |
| 33 | Sin constructor; XML externo en la ruta completa `/invoices`. |
| 52 | Solo modalidad electrónica. |

## Sector 47: devolución parcial

El mismo cuerpo sirve para `POST /v1/invoices/preview`, crear el borrador con
`POST /v1/invoices` o crear y emitir con `POST /v1/invoices/emit`.
Los UUID deben existir en su entorno. El SKU debe ser el código guardado en la
factura original, aunque el producto actual ya no tenga un mapeo activo.

```json
{
  "point_of_sale_id": "5226da47-74da-4b27-a26d-892c62fa6d26",
  "customer": {
    "document_type": "CI",
    "document_number": "9971522",
    "name": "Ramiro",
    "email": "juan.perez@example.com"
  },
  "invoice_type": "credit_note",
  "sector": "47",
  "reference_invoice_id": "b61d67ed-7962-4989-958f-8ccc01979d64",
  "items": [
    { "sku": "PROD-001", "quantity": 1, "price": 1250, "discount": 0 }
  ],
  "data": {}
}
```

Supay completa CUF, fecha y total originales. Para una devolución de 1250,
el cálculo general existente completa 162.50 como crédito efectivo (13%);
se puede proporcionar `monto_efectivo_credito_debito` explícitamente cuando
corresponda otra base. En ICE deben enviarse los datos específicos del sector
48 y sus importes correctos; el porcentaje general no sustituye el cálculo ICE.
No envíe CUF, fecha o total originales que contradigan la factura referenciada.

## Sector 29: conciliación

Use `sector: "29"`, `reference_invoice_id` y los mismos datos de cliente e ítems.
Cada ítem conciliado exige en `data` los montos del detalle del SDK:

```json
{
  "sku": "PROD-001",
  "quantity": 1,
  "price": 900,
  "data": {
    "monto_original": 1000,
    "monto_final": 900,
    "monto_conciliado": 100
  }
}
```

Los campos de cabecera `monto_total_conciliado`, `credito_fiscal_iva` y
`debito_fiscal_iva` deben expresar la operación real. Use el catálogo del
endpoint para consultar la lista completa. Las líneas originales se cargan de
la factura referenciada; no se pierden por intentar usar un `AddDetalle`
genérico que este constructor no implementa.

## Sectores especializados

Ejemplos de claves ahora transportadas hasta los constructores:

- Exportación: `data.costos_gastos_nacionales` como objeto JSON;
  `items[].data.codigo_nandina`.
- Hoteles: `items[].data.detalle_huespedes` como JSON.
- Hidrocarburos IEHD: `data.monto_iehd` y `items[].data.porcentaje_iehd`.
- Compra/venta: `items[].data.numero_serie` y `numero_imei`.

Son ejemplos de campos, no payloads completos: obtenga los demás requeridos del
perfil. Un producto de una venta ordinaria necesita un mapeo activo compatible
con el sector elegido; un mapeo al sector 1 no habilita automáticamente el 19.

## Importes y pago

`payment` permite configurar método, moneda y tipo de cambio:

```json
{"method_code": 1, "currency_code": 2, "exchange_rate": 6.96}
```

Los precios y totales internos se expresan en bolivianos. El monto en la moneda
seleccionada se calcula como total dividido por el tipo de cambio. El descuento
de línea está en `items[].discount`; el descuento de cabecera está en
`data.descuento_adicional` y se aplica una vez al total. La gift card reduce la
base gravada predeterminada, no el total. Para bases sectoriales especiales,
`data.monto_total_sujeto_iva` está disponible si el constructor lo admite.
Estos importes deben ser no negativos.

El sector 30 permite un borrador sin `items` con `total` y sus datos de pasajero
y cabecera. Su envío se realiza por
`POST /v1/siat/masiva/{companyId}/{pointOfSaleId}`, usando el contrato de lotes;
no por la ruta de emisión individual.

## Verificación local

```sh
go test ./...
```

Las pruebas de repositorio PostgreSQL requieren `TEST_DATABASE_URL` apuntando
a una base de pruebas. No se envían documentos reales al SIAT desde las
pruebas multisector agregadas.

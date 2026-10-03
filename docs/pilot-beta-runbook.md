# Gate del primer piloto

Este runbook es el registro operativo de la Fase 4. Se ejecuta primero en SIAT
`PILOTO` y se repite sin cambios para el segundo y tercer tenant. No marque el
gate como aprobado sin guardar fecha, operador, empresa, POS, factura y resultado.

## 1. Bootstrap y preparación

Desde el dashboard:

1. Crear la empresa con ambiente `PILOTO` y guardar su ID.
2. Crear una API key de integración y guardarla en el gestor de secretos.
3. Crear la sucursal y el punto de venta; guardar el ID del POS.
4. En **Certificado**, cargar el `.p12`, confirmar que aparece activo y no
   registrar su contraseña en este documento.
5. En **Conexión SIAT**, solicitar CUIS, sincronizar todos los catálogos y
   solicitar CUFD. El checklist debe terminar sin pasos pendientes.
6. Confirmar en **Productos** que existe al menos un producto SIN de la
   actividad económica del piloto y que el formulario de factura lo resuelve
   para el sector 1.

Antes de emitir, compruebe mediante la API:

```bash
curl -fsS -H "X-API-Key: $SMOKE_API_KEY" \
  "$API_URL/v1/companies/$COMPANY_ID/catalogs/readiness?point_of_sale_id=$POINT_OF_SALE_ID" | jq
```

La respuesta debe indicar catálogos sincronizados y el POS debe tener CUIS/CUFD
vigentes. Las precondiciones fiscales completas están en
`backend/docs/sdk-facturacion-una-factura.md`.

## 2. Gate automatizado de API

Prepare `SMOKE_INVOICE_JSON` con el fixture real del piloto. El script emite,
lee la factura, verifica estado SIAT, descarga y valida XML/PDF, comprueba el
aislamiento con el tenant B cuando se proporciona su key y finalmente anula.

```bash
export API_URL="https://api.example.com"
export SMOKE_API_KEY="...tenant A..."
export TENANT_B_API_KEY="...tenant B..." # obligatorio para cerrar la matriz
export SMOKE_INVOICE_JSON="$(jq -c . /ruta/fixture-sector-1.json)"
export SMOKE_FORBIDDEN_PUBLIC_HOST="public.example-r2.dev" # si existe
scripts/pilot-smoke.sh
```

Para crear la empresa mediante el endpoint interno, omita `SMOKE_API_KEY` y
defina `BACKEND_SECRET` y `SMOKE_BOOTSTRAP_JSON`. El resto del setup (certificado,
POS y credenciales SIAT) sigue siendo manual porque usa datos fiscales reales.

## 3. Click-through del dashboard

Con la misma empresa ya preparada:

1. **Nueva factura**: elegir POS y producto sector 1, crear y emitir.
2. En la confirmación, descargar PDF.
3. En **Facturas**, abrir el detalle, pestaña **SIAT**, verificar estado y
   descargar XML firmado.
4. Anular con un motivo del catálogo y confirmar estado **Anulada**.

El contrato usado por este flujo está cubierto por
`frontend/tests/self-hosted-host.test.mjs`; el click-through real sigue siendo
obligatorio porque valida navegador, CORS, autenticación y descargas.

## 4. Evidencia y decisión

Registrar en el ticket de lanzamiento:

- fecha/hora y commit/tag desplegado;
- `company_id`, `point_of_sale_id` e `invoice_id` (sin secretos ni NIT completo);
- resultado del script y capturas de confirmación/estado anulado;
- enlace a las tres alertas cargadas en Prometheus/Alertmanager;
- responsable y decisión `GO` o `NO-GO`.

Un fallo de cualquier paso es `NO-GO`; no se repite sobre la misma factura
rechazada, se corrige y se crea una factura nueva según el riesgo aceptado.

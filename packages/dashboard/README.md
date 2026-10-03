# @supay/dashboard

Núcleo del dashboard de Supay.

## Alcance de la beta

La beta soporta únicamente el host **self-hosted**, creado con
`createSelfHostedHost({ baseUrl })`. Este host acepta `X-API-Key` para
integraciones o una sesión `Bearer` junto con `X-Company-ID`, y realiza las
descargas PDF/XML mediante `fetch` autenticado.

El modo `cloud` es sólo un punto de extensión de la interfaz. No forma parte de
la beta: requiere un `BetterAuthHost` con organización activa y renovación del
JWT, que se implementará después de la beta.

El flujo canónico de emisión es `POST /v1/invoices/preview` seguido de
`POST /v1/invoices/emit` con `Idempotency-Key`. El payload debe incluir el
snapshot completo del receptor y de los datos fiscales de cada ítem; consulte
`backend/docs/sdk-facturacion-una-factura.md`.

# Alcance congelado de la beta

Este documento fija el contrato operativo de la beta abierta. Cambiar este
alcance requiere una decisión explícita de producto; no debe ocurrir como efecto
colateral de un cambio técnico.

## Flujo soportado el día 1

El camino crítico de una integración es:

1. Crear el tenant con `POST /internal/companies` usando `BACKEND_SECRET`.
2. Conservar la API key devuelta y enviarla como `X-API-Key`.
3. Crear una factura con `POST /v1/invoices` o crearla y emitirla de forma
   síncrona con `POST /v1/invoices/emit`.
4. Consultar la factura con `GET /v1/invoices/{id}` y su estado fiscal con
   `GET /v1/invoices/{id}/siat-status`.
5. Descargar sus artefactos con `GET /v1/invoices/{id}/xml` y
   `GET /v1/invoices/{id}/pdf`.
6. Anularla con `POST /v1/invoices/{id}/annul`.

`GET /v1/invoices/sectores` publica todos los perfiles que compila la versión
instalada. El campo `estado` siempre vale `soportado` o `experimental`;
`soportado` se mantiene además como booleano por compatibilidad. Esta marca
describe cobertura técnica, no homologación ni habilitación tributaria de un
tenant. Los fallos SIAT de perfiles experimentales son feedback de producto.

## Matriz de despliegue

### Self-hosted

- `DEPLOYMENT_MODE=selfhosted` es el valor predeterminado.
- `STORAGE_DRIVER=local` es el valor predeterminado.
- Los secretos pueden seguir viniendo de `.env`.
- `AUTO_MIGRATE=true` continúa siendo válido para una única réplica.
- La imagen debe seguir pudiendo ejecutar web y trabajos en un solo contenedor.

### SaaS beta

- Se despliega un único servicio con web y River juntos.
- Usa `DEPLOYMENT_MODE=cloud` y almacenamiento privado Cloudflare R2.
- Las restricciones exclusivas del SaaS solo pueden activarse con
  `DEPLOYMENT_MODE=cloud`; nunca deben endurecer globalmente el modo self-hosted.

## Fuera del alcance de la beta

- Worker dedicado con CPU siempre asignada.
- Emisión asíncrona pública con respuesta `202`.
- Observabilidad OTel completa.
- Migración decimal dual-write completa.
- Aislamiento tenant total fuera del módulo de facturas.
- Despliegue canary 90/10.
- Operación de instalaciones self-hosted de terceros.

Estos puntos pertenecen a v1.1 y no bloquean la apertura de la beta.

**Resumen**
- **Qué:** Implementación inicial del motor de facturación SIAT dentro del proyecto Supay.
- **Objetivo:** Proveer flujo básico de generación de XML de factura, cálculo de hash, persistencia y envío SOAP mínimo a SIAT (operación `recepcionFactura`) usando el token delegado en la cabecera.
- **Estado:** Funcionalidad básica implementada; falta firma XMLDSig (requerida para modalidad Electrónica) y ajuste total al XSD SIAT.

**Qué se implementó**
- **Generación de XML:** Generador básico a partir de `Invoice` y sus `Items`, con `XML` indentado y cálculo SHA-256.
- **Persistencia:** XML y hash almacenados en la entidad `invoices` y eventos en `invoice_events`.
- **Envío a SIAT:** Envío SOAP mínimo (sin firma) a la operación `recepcionFactura` usando el cliente SOAP que añade la cabecera `apikey: TokenApi {SIAT_TOKEN_DELEGADO}`.
- **Rutas HTTP:** Endpoint para disparar la emisión por ID de factura.

**Archivos añadidos/modificados**
- **Resumen investigación:** [SIAT_WEB_RESEARCH.md](SIAT_WEB_RESEARCH.md)
- **Seed:** [cmd/seed/main.go](cmd/seed/main.go)
- **Servidor:** [cmd/server/main.go](cmd/server/main.go) (registro de ruta /siat/emit/{invoiceId})
- **SIAT client y servicios:** [internal/siat/client.go](internal/siat/client.go), [internal/siat/cuis_service.go](internal/siat/cuis_service.go), [internal/siat/cufd_service.go](internal/siat/cufd_service.go)
- **Emission service (nuevo):** [internal/siat/emission_service.go](internal/siat/emission_service.go)
- **Handlers:** [internal/delivery/http/siat_handler.go](internal/delivery/http/siat_handler.go) (añadido EmitInvoice)
- **Invoice repo (nuevo):** [internal/repository/postgres/invoice_repo.go](internal/repository/postgres/invoice_repo.go)
- **Modelos y migraciones:** [internal/models/model.go](internal/models/model.go) (ya existente — usado por la implementación)
- **Configuración:** [internal/config/config.go](internal/config/config.go) (carga de SIAT_TOKEN_DELEGADO como apikey: TokenApi {token})

**Cómo probar localmente**
1. Configurar .env con las variables SIAT necesarias (ya tienes SIAT_TOKEN_DELEGADO, SIAT_NIT, SIAT_CODIGO_SISTEMA, etc.).
2. Ejecutar migraciones y seed para crear company + point of sale:
```bash
cd /home/brandon/Documents/projects/supay/backend
go run cmd/seed/main.go
```
3. Iniciar servidor:
```bash
go run cmd/server/main.go
```
4. Crear una factura en la base (usa tu UI o inserta con SQL). Luego disparar emisión:
```bash
curl -X POST http://localhost:8080/siat/emit/{invoiceId}
```
Respuesta: JSON con invoice_id, status y xml_hash. Eventos y respuesta SIAT quedan registrados en la BD.

**Limitaciones y próximos pasos**
- Firma XMLDSig: implementar firmador (aceptar PKCS#12 o PEM) y aplicar firma al XML antes de envío. Esto es obligatorio para la modalidad Electrónica.
- Ajustar XML al XSD SIAT: hoy se genera una estructura simplificada; hay que mapear y validar contra los XSD oficiales.
- Manejo avanzado de respuestas: parseo de CUF, códigos, observaciones, reintentos y reglas para marcar ACCEPTED / REJECTED.
- Contingencia: armado de paquetes (gzip + SHA256), registro de eventos significativos, envío/validación de paquetes.
- Tests: añadir pruebas unitarias e integración (mocks y/o pruebas contra pilotosiatservicios).

**Cómo proporcionar el certificado**
- Opción A (recomendada para implementar firma ahora): añadir variables de entorno en .env con la ruta y contraseña del P12/PEM y avisarme para implementar el firmador y pruebas contra pilotosiatservicios.
- Opción B: implementar la interfaz de firma y documentar cómo subir el certificado y la configuración. (Actual: la implementación permite envío sin firma para pruebas locales pero no es válida para homologación).

**Contacto / notas**
- Fecha de la implementación: 2026-08-04
- Si quieres que implemente la firma y el envío firmado ahora, indica si vas a proporcionar el .p12/.pem y confirmá que usaremos el entorno PILOTO (pilotosiatservicios.impuestos.gob.bo).

Archivo generado automáticamente por la tarea de implementación SIAT en el repositorio Supay.

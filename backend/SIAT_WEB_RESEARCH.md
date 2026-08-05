Resumen de investigación web sobre SIAT (SIN) — 2026-08-04

Este archivo recopila las páginas oficiales consultadas, puntos clave operativos y próximos pasos recomendados para integrar SIAT en Supay.

Páginas oficiales consultadas
- https://siatinfo.impuestos.gob.bo/index.php/sistema-facturacion — Marco general y anexo técnico.
- https://siatinfo.impuestos.gob.bo/index.php/informacion/modalidades-facturacion/facturacion-electronica — Modalidad Electrónica: firma, CUIS, CUFD, flujo.
- https://siatinfo.impuestos.gob.bo/index.php/informacion/codigos-de-autorizacion — Definición CUIS/CUFD/CUF/CAFC.
- https://siatinfo.impuestos.gob.bo/index.php/facturacion-en-linea/implementacion-servicios-facturacion/codigos/solicitud-cufd — Parámetros del servicio `solicitudCufd`.
- https://siatinfo.impuestos.gob.bo/index.php/facturacion-en-linea/emision-y-envio-de-facturas/contingencia-y-eventos-significativos — Reglas y límites de contingencia (paquetes, plazos, eventos).
- https://siatinfo.impuestos.gob.bo/index.php/facturacion-en-linea/casos-especiales/manuales-contingencia — Procedimiento de transcripción y armado/envío de paquetes (gzip + SHA256, hasta 500 facturas por paquete).
- https://siatinfo.impuestos.gob.bo/index.php/facturacion-en-linea/factura-electronica?id=750 — Información sobre versiones y WSDL públicos (piloto):
  - https://pilotosiatservicios.impuestos.gob.bo/v1/ServicioFacturacionComputarizada?wsdl
  - https://pilotosiatservicios.impuestos.gob.bo/v2/ServicioFacturacionComputarizada?wsdl
- https://siatanexo.impuestos.gob.bo/index.php/informacion-gral/requisitos-sfvl — Requisitos, certificados (ADSIB, DIGICERT) y autorización.

Puntos clave operativos (resumen)
- CUIS: vigencia 365 días. Relaciona NIT, sistema, sucursal y punto de venta. Requerido antes de operar.
- CUFD: vigencia ~24 horas. Debe obtenerse diariamente (servicio `solicitudCufd`) y antes de enviar paquetes en contingencia.
- CUF: ID único de factura generado en la emisión.
- Contingencia: emitir offline o con facturas manuales CAFC; transcribir y enviar paquetes comprimidos (gzip) con hash SHA-256; paquetes hasta 500 facturas y del mismo documento sector; registrar evento significativo dentro de 48 horas.
- Firma digital: XMLDSig para la modalidad Electrónica en Línea; almacenar certificados de forma segura y rotarlos.
- Catálogos: sincronización periódica (actividades, unidades, métodos de pago, monedas, tipos de documento, etc.).
- Servicios SOAP/WSDL: SIAT expone servicios WSDL versionados; la integración típica usa SOAP (consumo de WSDL) para operaciones de códigos, emisión y recepción de paquetes.

WSDL y endpoints (ejemplos)
- Piloto WSDL v1: https://pilotosiatservicios.impuestos.gob.bo/v1/ServicioFacturacionComputarizada?wsdl
- Piloto WSDL v2: https://pilotosiatservicios.impuestos.gob.bo/v2/ServicioFacturacionComputarizada?wsdl
(En producción los endpoints oficiales serán similares bajo dominios `impuestos.gob.bo` y requieren autenticación/tokens.)

Recomendación inmediata para Supay (pasos técnicos)
1. Implementar módulo `siat` (Go) con submódulos: codes (CUIS/CUFD), emisión (XML+firma), paquetes/contingencia, catálogos, administración de certificados.
2. Crear cliente SOAP reutilizable que consuma los WSDL de piloto y producción (probar contra `pilotosiatservicios`).
3. Implementar generador de XML conforme a XSD oficiales y un firmador XMLDSig (soporte para certificados PKCS#12 o PEM + clave privada segura).
4. Implementar sincronización de catálogos y caché con versionado.
5. Implementar flujo de contingencia (armado de paquetes, gzip, SHA256, registro de evento significativo y envío/validación de paquetes).
6. Añadir pruebas de integración contra los WSDL de `pilotosiatservicios` y documentación para homologación.

Próximos pasos que puedo hacer ahora
- Generar un `dossier` detallado (2–4 páginas) con ejemplos de payloads (CUIS/CUFD, ejemplo XML de factura) y enlaces XSD/WSDL.
- O generar un ejemplo de cliente Go mínimamente funcional que solicite CUIS/CUFD contra el WSDL de piloto.

¿Cuál prefieres? Responde: `dossier` o `cliente-go`.

Fuente: páginas públicas SIAT/SIN consultadas (enlaces arriba), fecha de captura: 2026-08-04.

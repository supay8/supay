# Arquitectura del backend

## Dirección de dependencias

- `internal/domain`: entidades, transiciones y políticas fiscales puras. Los perfiles de `domain/fiscal` describen los 52 layouts y validan los campos de cabecera y detalle sin importar el SDK ni GORM. Las consultas devuelven copias de la metadata.
- `internal/ports`: contratos de entrada y salida. `InvoiceEmitter` y `InvoiceEmissionProcessor` son puertos de entrada para HTTP y workers. Los repositorios, almacenamiento, cifrado y capacidades fiscales son puertos de salida.
- `internal/usecase`: composición de reglas y coordinación de efectos mediante puertos. Solo importa otros paquetes del núcleo; no conoce adaptadores, modelos ORM ni configuración del despliegue.
- `internal/adapters/siat`, `internal/repository/postgres`, `internal/storage`, `internal/auth`: implementaciones de los puertos. PostgreSQL normaliza los errores de ausencia a `domain.ErrNotFound`.
- `internal/delivery`: transportes de entrada que llaman casos de uso y puertos. No importa adaptadores SIAT.
- `internal/app`: composición y propiedad de recursos. Es el único lugar que selecciona las implementaciones para cada despliegue.

`FiscalService` compone capacidades en el límite del proveedor multiempresa. Los consumidores individuales piden contratos menores: emisión consume `FiscalSingle`, credenciales consume `FiscalCredentials`; serialización, firma, empaquetado y sincronización tienen puertos propios.

## Emisión individual

1. Validar datos sectoriales antes de pedir credenciales o reservar la factura.
2. Resolver CUIS/CUFD y reservar atómicamente `PENDING → SENDING`.
3. Serializar y validar el documento con llamadas compiladas al SDK.
4. Firmar cuando corresponde a modalidad electrónica y comprimir los bytes exactos.
5. Guardar el XML inmutable y su metadata; después guardar CUF, CUFD y archivo preparado en PostgreSQL.
6. Despachar al SIAT usando ese mismo archivo, sin reconstruir ni firmar otra vez.
7. Persistir la respuesta mediante una transición explícita y generar PDF/notificación.

`single.Preparer` recibe `DocumentSerializer`, `FiscalSigner` y `LotPacker`. Cada fallo detiene las etapas siguientes. La proyección de artefactos y la decisión del estado resultante son funciones sin I/O. El coordinador hace explícitas las escrituras y el envío.

Un reintento verifica el digest del archivo preparado y reutiliza su XML firmado y CUFD original. Si la reserva SQL falla después de crear el objeto, se compensa únicamente el archivo creado por ese intento. Un fallo de preparación local nunca activa la contingencia por conectividad. Un fallo de transporte puede activar la contingencia existente; se retira la preparación en línea antes de guardar el CUF offline, evitando colisiones de metadata. Fallos de compensación se propagan para conciliación.

La reserva `SENDING` y la máquina de estados conservan la protección contra emisiones concurrentes. `domain.CanTransition` consulta una tabla privada de aristas y motivos; los repositorios siguen validando cada transición. Documentos aceptados, anulados y rechazados no vuelven arbitrariamente a borrador.

Los adaptadores reales implementan `FiscalEmissionPipeline`. Se conserva la vía `Emit` para consumidores embebidos y dobles de pruebas que implementan únicamente el contrato individual anterior; el sandbox no realiza transporte SOAP.

## Capacidades SIAT

- `adapters/siat/sync`: CUIS, CUFD, identidad del SDK y sincronización de catálogos.
- `adapters/siat/single`: preparación individual por etapas.
- `adapters/siat/batch`: núcleo único de gzip/tar/base64 y SHA-256 del archivo comprimido. La emisión por paquetes y masiva comparte la preparación de documentos; XML ya persistido se empaqueta sin modificarlo.
- El paquete raíz `adapters/siat` conserva las fachadas de transporte y compatibilidad del SDK, la selección por sector y el puente hacia los puertos. No es parte del núcleo de negocio.

El puente de arranque `Config.SIAT → siat.Service → FiscalAdapter` tiene **sunset el 2026-12-31**. Solo conserva compatibilidad para consumidores embebidos que construyen `Config` directamente; `config.Load` ya ignora credenciales globales del entorno. Antes de retirarlo, esos consumidores deben registrar tokens/certificados por empresa y resolverlos con `FiscalServiceProvider`. En esa fecha se retirarán el bloque legacy de `app/siat.go` y `Config.SIAT` después de verificar la migración de los consumidores. No hay un corte automático por reloj.

La coordinación de lotes vive en `siat_batches.go`; selección, preparación, documentos y validación tienen archivos propios. Las pruebas de emisión están separadas por documentos, credenciales, catálogos, contingencias, estados y creación; los fakes y fixtures compartidos se mantienen aparte.

`builder_reflex.go` fue eliminado. `builder_calls_generated.go` llama setters tipados y compilados; la reflexión para descubrir sus firmas existe exclusivamente en la herramienta de desarrollo `internal/buildergen`. Los valores sectoriales desconocidos, requeridos ausentes, enteros fraccionarios y contenedores JSON incompatibles fallan con errores de validación antes de SOAP. HTTP los traduce a 400.

Al actualizar go-siat:

```sh
go generate ./internal/adapters/siat
go test ./internal/adapters/siat/... ./internal/domain/...
```

La lista de constructores y el esquema fiscal explícito deben revisarse junto al SDK. Las pruebas de paridad recorren todos los perfiles y sus campos, incluyendo opcionales y variantes de layout. CI comprueba que los archivos generados sean reproducibles.

## Arranque y recursos

`NewCloudApp` y `NewSelfHostedApp` construyen la aplicación completa y devuelven `(*App, error)`. `NewApp` selecciona la factory una sola vez. Cloud usa JWKS y membresías Better Auth; self-hosted usa JWT local y sus rutas de autenticación. La lógica de negocio no pregunta por el entorno.

La composición está distribuida en `postgres.go`, `siat.go`, `http.go`, `services.go`, `email.go` y `queue.go`. No hay getters que creen servicios de forma diferida. `App.Close` cierra recursos propios en orden inverso, también cuando el arranque falla. `WithDatabase` permite inyectar una conexión cuya propiedad conserva quien la entrega.

`database.OpenForMode` recibe el modo elegido por la factory para configurar el pool; no lo vuelve a inferir de `DEPLOYMENT_MODE`. Los constructores internos propagan errores. La terminación del proceso queda en `cmd/*`. Este cambio no añade migraciones SQL.

## Outbox de notificaciones

Las transiciones a `ACCEPTED`/`OBSERVED`, tanto individuales como de lotes, escriben el outbox `invoice_email_notifications` en la misma transacción que el estado fiscal y la auditoría. La preferencia del tenant y el email del snapshot determinan la elegibilidad; el índice único de entregas automáticas evita duplicados. Un fallo posterior revierte también la entrega pendiente.

Los repositorios no reciben `INVOICE_EMAIL_ENABLED` ni tienen setters de configuración. Ese flag controla la composición del dispatcher/worker. Con el dispatcher desactivado, las entregas elegibles quedan pendientes y se procesan al reactivarlo; así una pausa operativa no pierde notificaciones. La publicación en Cloud Tasks y SMTP ocurre después del commit.

## Gobierno y verificación

Desde `backend/`:

```sh
go test -race ./...
go vet ./...
golangci-lint run
```

`.golangci.yml` utiliza depguard v2 para impedir las dependencias hacia infraestructura desde el núcleo y hacia adaptadores desde delivery. `internal/architecture` también verifica la dirección de imports, la ubicación de contratos y la ausencia de `log.Fatal*`/`os.Exit` internos con `go test`.

El workflow `.github/workflows/backend-ci.yml` ejecuta estas comprobaciones y la generación reproducible. Levanta PostgreSQL 16 y define `TEST_DATABASE_URL` para ejecutar las pruebas transaccionales de repositorio y River, además de las pruebas con fakes y servidores HTTP locales. `testutil.LockPostgres` protege la base compartida entre paquetes mediante un advisory lock de sesión hasta terminar la limpieza de cada prueba.

Para ejecutarlas localmente, `TEST_DATABASE_URL` debe apuntar a una base desechable: el helper aplica migraciones y trunca tablas. R2 y SIAT real siguen necesitando sus variables y recursos externos.

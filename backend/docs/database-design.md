# Diseño de persistencia y ciclo de vida

La migración `000021_database_lifecycle` amplía el esquema existente. Go y
golang-migrate siguen siendo los propietarios del DDL; Next.js ejecuta Better
Auth en Supay Cloud con permisos DML sobre `auth.*`. No usar AutoMigrate de GORM
ni el CLI de Better Auth para modificar este esquema.

## Autenticación y tenants

En Cloud, `auth.users` representa la identidad y `auth.members` la pertenencia.
Una organización se enlaza 1:1 con el tenant fiscal mediante
`tenants.auth_organization_id`. El backend valida el JWT de Better Auth y
consulta la membresía actual: eliminar una membresía revoca acceso aunque el
JWT todavía sea válido. La implementación de Better Auth pertenece al Next.js
de Supay Cloud, fuera de este repositorio.

En self-hosted, `public.users` y `public.user_tenants` son canónicos. El JWT local
solo autoriza tenants sin organización Cloud. Una fila local que apunte a un
tenant Cloud no concede acceso ni aparece en su lista de empresas.

`tenant_memberships` es una vista derivada para consultas administrativas;
`(provider, user_id)` identifica al principal. No sincronizar membresías entre
proveedores, copiar hashes ni vincular identidades por coincidencia de email.
La vista conserva el rol de Better Auth como texto, incluyendo roles personalizados.
Las políticas de rol siguen siendo responsabilidad del flujo de autorización;
la vista y los repositorios de membresía verifican pertenencia, no permisos por acción.

Ambos proveedores normalizan email con `lower(btrim(email))` al insertar o
actualizar. Cloud tiene además un índice único normalizado. No cambiar IDs al
normalizar ni fusionar usuarios existentes.

## Colas e historial

Outbox conserva cada evento publicado. Solo puede existir una emisión
`invoice.emit.requested` pendiente/procesando por tenant, tipo de agregado y
factura. El enqueue bloquea la factura y reutiliza el evento pendiente;
después de publicar, una nueva solicitud crea otro ID. Los errores de despacho
reintentan el mismo evento y conservan su contador. Otros tipos de evento
pueden repetirse y se identifican por su UUID. El dispatcher sigue
usando leases y `SKIP LOCKED`.

`sent_package_invoices` tiene PK `(invoice_id, sent_package_id)`. Un trigger
bloquea la factura e impide agregarla a otro paquete mientras exista una
pertenencia a un paquete no rechazado, incluyendo uno aceptado o con resultado
desconocido. Las pertenencias anteriores se conservan y no se pueden editar.
Los repositorios ignoran paquetes rechazados al comprobar reservas.

Esto permite representar intentos sucesivos, pero **no autoriza por sí solo
reenviar una factura rechazada al SIAT**. El flujo actual sigue exigiendo
facturas PENDING/OFFLINE elegibles, conserva CUF y recepción y no resetea
estados fiscales automáticamente. Un futuro flujo de conciliación/reenvío debe
validar el rechazo y la identidad fiscal antes de crear otro paquete.

Email deduplica la entrega automática por `(tenant_id, invoice_id, recipient)`
mediante un índice parcial `WHERE is_automatic`. Repetir la transición a ACCEPTED
no produce otro email automático. `QueueDelivery` permite destinatarios adicionales
y reenvíos explícitos sin claves auxiliares: cada llamada crea un UUID de notificación.
Los reintentos de esa entrega usan el mismo UUID. Exige factura ACCEPTED, tenant
activo y preferencia habilitada. El método está disponible en el repositorio;
todavía no existe un endpoint/UI de reenvío. El llamador comprueba autorización.

## Receptores y documentos

Los datos fiscales de `customers` son inmutables: tenant, documento,
complemento, nombre, código e identidad histórica. Solo `email` e `is_active`
pueden actualizarse. Cambiar un nombre/documento crea otra fila.
`List` muestra el receptor activo más reciente por tenant, tipo, número y
complemento, con desempate por ID; el histórico se mantiene en la tabla.
La factura conserva su propio snapshot de receptor, incluido su email original.

`invoice_files.tenant_id` reemplaza el nombre `company_id` y conserva su FK
compuesta con invoices. La API mantiene el vocabulario company por
compatibilidad. El backfill de XML anterior a 000017 utiliza explícitamente
el repositorio legacy con `company_id`; no requiere aplicar 000021 antes de
rescatar los XML.

`invoice_documents` ya tenía un índice único parcial para el documento actual.
Al insertar una versión actual, el nuevo trigger bloquea la factura, exige
avanzar la versión y desmarca la anterior en la misma transacción. La metadata
histórica es inmutable; solo `is_current` puede cambiar. Una inserción fallida
revierte también la desmarcación. Los bytes continúan en object storage.

## Operación, retención y borrado

El administrador puede ejecutar, con un rol que tenga EXECUTE y UPDATE:

```sql
BEGIN;
SELECT public.archive_tenant('UUID_DEL_TENANT', 'Motivo de cierre');
COMMIT;
```

La función usa privilegios del invocador y revoca EXECUTE a PUBLIC. El
administrador concede acceso explícito a un rol operativo si lo necesita.
Desactiva el tenant, conserva fecha/motivo y revoca sus API keys en la misma
transacción. Las consultas de autorización JWT, Better Auth y API keys exigen
tenant activo. Las tareas fiscales pendientes de conciliación siguen su curso;
archivar no significa cancelar una operación que pudo llegar al SIAT.
No borra usuarios Cloud compartidos con otras organizaciones.

Archivar el acceso **no exporta objetos ni elimina datos personales**. El
traslado a almacenamiento frío, el inventario de objetos/backups y la eventual
purga requieren un trabajo operativo aparte. La retención fiscal no se puede
inferir del modo PILOTO ni de la fecha de cierre. Para una purga de producción,
definir elegibilidad de cada categoría, verificar obligaciones de retención,
exportar/verificar documentos, registrar autorización y ejecutar una migración
administrativa específica sobre el conjunto aprobado. No ofrecer un bypass
genérico con `session_replication_role` ni una función SECURITY DEFINER que
permita al runtime saltarse todas las restricciones.

Los datos de prueba deben vivir en bases separadas y descartables; eliminar
esa base elimina su conjunto completo sin introducir excepciones al historial
fiscal de producción. Auth mantiene sus cascadas propias para retirar
identidades/sesiones sin borrar las facturas.

## Índices y concurrencia

000020 retiró `certificates.point_of_sale_id`, por lo que el índice con
COALESCE ya no existía. La regla vigente es un certificado ACTIVE por tenant,
protegida por un índice único parcial, sin UUID centinela.

`api_keys.scopes` pasa de CSV a `text[]`, y GORM usa `pq.StringArray`. No se
cambia la política de autorización de scopes con esta conversión.
Los triggers mantienen `updated_at` en todas las tablas existentes que tienen
esa columna, incluyendo auth. Una escritura SQL directa ya no puede envejecer
artificialmente un registro para el reaper.

La sincronización de catálogos ya adquiría un `pg_advisory_xact_lock` por
`(tenant, tipo)` antes de calcular MAX(version)+1. Se conserva ese mecanismo,
la restricción única y la escritura de versión/ítems en la misma transacción;
una prueba concurrente verifica ocho sincronizaciones sin colisiones. Las
escrituras directas de catálogos deben respetar el mismo protocolo.

Los BRIN sobre `created_at` de invoices, invoice_events y outbox facilitan
consultas temporales sin cambiar las PK UUID ni las FKs. Medir planes y tamaño
antes de particionar. Un particionado por fecha requiere diseñar nuevamente
unicidad, relaciones e idempotencia; no se aplica como cambio automático.

## Despliegue y rollback

1. Respaldar y probar la actualización en una copia representativa.
2. Resolver emails Cloud duplicados al normalizar y certificados ACTIVE
   múltiples por tenant. La migración aborta, no fusiona cuentas ni revoca
   certificados silenciosamente. Las colisiones locales con espacios también
   abortan por el índice existente al normalizar.
3. Ejecutar las migraciones con el rol propietario, con los escritores
   detenidos. El renombrado de invoice_files y la conversión de scopes requieren
   desplegar este backend junto con el esquema; los binarios antiguos no son
   compatibles. Las operaciones DDL/índices pueden bloquear tablas grandes.
4. Arrancar backend y Next.js; comprobar permisos DML del rol de Cloud.

El down rechaza historiales con múltiples eventos, pertenencias o entregas
que el esquema anterior no puede representar, y metadata de archivo que se perdería. No borra filas para forzar rollback. La
normalización de emails y los cambios de contacto ya efectuados no se revierten.

Validación realizada con PostgreSQL 16 aislado: suite de repositorios,
concurrencia de outbox/catálogos, FK multi-tenant de archivos, snapshots,
sustitución de documentos, reenvíos, separación de autoridades y archivado.
También se probó 000001–000020 con datos existentes → 000021 → down → up y
el rechazo del down al existir un segundo intento de outbox.

Referencias oficiales consultadas: [organizaciones de Better Auth](https://better-auth.com/docs/plugins/organization),
[índices parciales de PostgreSQL](https://www.postgresql.org/docs/current/indexes-partial.html)
y [bloqueos transaccionales](https://www.postgresql.org/docs/current/explicit-locking.html).

## Retiro de claves auxiliares (000022)

`000022_remove_idempotency_keys` elimina las columnas de claves de invoices,
outbox y emails. El runtime, dashboard, OpenAPI, Postman y scripts ya no generan,
leen ni envían claves auxiliares. La unicidad de CUF y los correlativos por POS
siguen protegidos por PostgreSQL. Los locks, estados fiscales y IDs de trabajo
siguen evitando despachos simultáneos sobre una misma factura.

Crear un borrador todavía no implica tener un CUF. Repetir POST /invoices o
POST /invoices/emit crea otra factura; un payload idéntico no prueba que sea la
misma venta. Conservar ID, consultar el estado y reenviar/conciliar la factura
existente evita pedir un nuevo correlativo en un reintento operativo. Los paquetes
OFFLINE conservan el XML firmado del documento existente.

Aplicar 000022 junto con este backend/dashboard, con escritores detenidos.
El down restaura columnas opcionales pero no recupera las claves eliminadas:
no son parte del documento fiscal. Conserva CUFs, eventos y entregas; a los
emails manuales les asigna el UUID de notificación en el contrato anterior.
Las referencias a claves en 000001/000021 son historia de migraciones, no un
contrato de API vigente.

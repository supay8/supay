# Plan Maestro Definitivo: Supay API como SaaS Multi-Tenant de Facturación

> **Producto:** Plataforma en la nube donde cada cliente (tenant) se registra con su API key, configura sus datos fiscales y emite sus propias facturas ante el SIAT. Supay no factura; habilita a otros a facturar.
>
> **Objetivo arquitectónico:** Sistema multi-tenant, base de datos normalizada, dominio limpio, SIAT aislado como adaptador externo, emisión asíncrona con colas, idempotencia y catálogos versionados.

---

## 1. Principios Arquitectónicos

### P1 — Multi-tenancy por diseño, no como afterthought

- Cada tabla de negocio lleva `tenant_id` (o `company_id`).
- Toda query filtra por tenant.
- El tenant se resuelve desde la API key del request.
- Nunca se mezclan datos de dos tenants.

### P2 — Base de datos normalizada y relacional

- 3FN en datos maestros.
- Sin redundancia innecesaria.
- Claves foráneas explícitas con `ON DELETE RESTRICT`.
- CHECK constraints para enums y estados.
- Índices por tenant y por flujo de trabajo.

### P3 — SIAT como adaptador externo

- El dominio propio no conoce `go-siat`.
- Puerto `FiscalService` abstrae todas las operaciones fiscales.
- El adaptador SIAT es reemplazable y testeable con mocks.

### P4 — Estados explícitos y máquina de estados

- Toda factura tiene un estado bien definido.
- Solo la máquina de estados autoriza transiciones.
- La BD refuerza las transiciones con constraints.

### P5 — Emisión asíncrona y confiable

- El request HTTP solo crea/elige la factura y encola un job.
- Worker con reintentos, backoff y circuit breaker.
- Outbox pattern para atomicidad.

### P6 — Cada fase entrega valor

- Ninguna fase es "refactor por refactor".
- Cada una mejora performance, seguridad, DX o confiabilidad.

---

## 2. Modelo de Datos Objetivo

### 2.1. Diagrama de entidades

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│    tenants      │────▶│  tenant_configs  │     │   api_keys      │
│  (companías)    │     │ (límites, flags) │◄────│ (auth por key)  │
└────────┬────────┘     └──────────────────┘     └─────────────────┘
         │
         │ 1:N
         ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│     branches    │────▶│ points_of_sale   │◄────│   certificates  │
│  (sucursales)   │     │ (puntos de venta)│     │  (certs SIAT)   │
└─────────────────┘     └────────┬─────────┘     └─────────────────┘
                                 │
                                 │ 1:N
                                 ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  cufd_history   │     │  cuis_history    │     │contingency_events│
│  (CUFD vigentes)│     │  (CUIS vigentes) │     │  (contingencias) │
└─────────────────┘     └──────────────────┘     └─────────────────┘
         │
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                         invoices                                │
│  tenant_id | pos_id | customer_id | cufd_id | invoice_number    │
│  status | emission_type | issue_date | subtotal | total | cuf   │
└─────────────────────────────────────────────────────────────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌─────────┐ ┌─────────────┐ ┌─────────────┐
│ invoice │ │ invoice_    │ │  invoice_   │
│ _items  │ │ _events     │ │ _documents  │
└─────────┘ └─────────────┘ └─────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Catalogos SIAT versionados                   │
│  catalog_versions ──▶ catalog_items (por tenant y tipo)         │
│  sin_products | activities | legends | doc_sectors | parametrics │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│     outbox      │────▶│   job_queue      │     │  sent_packages  │
│  (eventos)      │     │  (workers)       │     │  (paquetes SIAT)│
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

### 2.2. Tablas principales

#### `tenants` (antes `companies`)

```sql
CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nit varchar(20) NOT NULL,
    business_name varchar(150) NOT NULL,
    municipio varchar(100) NOT NULL DEFAULT '',
    direccion text NOT NULL DEFAULT '',
    telefono varchar(50) NOT NULL DEFAULT '',
    usuario_siat varchar(50) NOT NULL DEFAULT 'SUPAY',
    codigo_actividad varchar(20),
    pie_pagina text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(nit)
);
-- Nota: ambiente, codigo_sistema y modalidad viven en tenant_configs para tener
-- una única fuente de verdad operativa por tenant.
CREATE INDEX idx_tenants_nit ON tenants(nit);
```

#### `tenant_configs`

Configuración operativa y límites por tenant. Separada de `tenants` para no bloquear la tabla maestra en actualizaciones frecuentes.

```sql
CREATE TABLE tenant_configs (
    tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    ambiente varchar(20) NOT NULL DEFAULT 'PILOTO' CHECK (ambiente IN ('PILOTO','PRODUCCION')),
    codigo_sistema varchar(50) NOT NULL,
    codigo_modalidad int NOT NULL DEFAULT 1 CHECK (codigo_modalidad IN (1,2)),
    token_siat text,                       -- token de autorización SIAT (cifrado)
    api_token text,                        -- token auxiliar si aplica (cifrado)
    max_invoices_monthly int NOT NULL DEFAULT 1000,
    max_points_of_sale int NOT NULL DEFAULT 5,
    max_api_keys int NOT NULL DEFAULT 10,
    settings jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

#### `api_keys`

```sql
CREATE TABLE api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_hash varchar(255) NOT NULL,        -- hash de la API key
    key_prefix varchar(20) NOT NULL,       -- prefijo visible (sup_...)
    name varchar(100) NOT NULL DEFAULT 'default',
    scopes text[] NOT NULL DEFAULT '{}',
    is_active boolean NOT NULL DEFAULT true,
    last_used_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(key_hash)
);
CREATE INDEX idx_api_keys_tenant ON api_keys(tenant_id);
```

#### `branches`

```sql
CREATE TABLE branches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    codigo_sucursal int NOT NULL,
    name varchar(150) NOT NULL,
    address text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, codigo_sucursal)
);
```

#### `points_of_sale`

```sql
CREATE TABLE points_of_sale (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid REFERENCES branches(id) ON DELETE SET NULL,
    codigo_sucursal int NOT NULL,
    codigo_punto_venta int NOT NULL,
    description varchar(150) NOT NULL,
    tipo_punto_venta int,
    is_active boolean NOT NULL DEFAULT true,
    siat_registered boolean NOT NULL DEFAULT false,
    siat_registered_at timestamptz,
    siat_response jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, codigo_sucursal, codigo_punto_venta)
);
CREATE INDEX idx_points_of_sale_tenant ON points_of_sale(tenant_id);
```

#### `certificates`

Certificados digitales (P12/PFX) del tenant, cifrados en reposo. Un tenant puede tener varios certificados, pero solo uno activo por punto de venta (o a nivel tenant si el mismo certificado cubre todos los POS).

```sql
CREATE TABLE certificates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid REFERENCES points_of_sale(id) ON DELETE SET NULL,
    alias varchar(150) NOT NULL,
    encrypted_blob bytea NOT NULL,         -- archivo P12 cifrado con AES-GCM
    encrypted_password text,               -- contraseña del P12 cifrada
    serial_number varchar(100),
    issuer varchar(255),
    subject varchar(255),
    not_before timestamptz NOT NULL,
    not_after timestamptz NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','EXPIRED','REVOKED')),
    is_active boolean NOT NULL DEFAULT true,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_certificate_dates CHECK (not_after > not_before)
);
CREATE INDEX idx_certificates_tenant ON certificates(tenant_id);
CREATE INDEX idx_certificates_active ON certificates(tenant_id, point_of_sale_id, status)
    WHERE status = 'ACTIVE';

-- Solo un certificado ACTIVE por punto de venta. Si point_of_sale_id es NULL,
-- se considera certificado global del tenant (también permitido uno solo).
CREATE UNIQUE INDEX idx_certificates_one_active_per_pos
    ON certificates(tenant_id, COALESCE(point_of_sale_id, '00000000-0000-0000-0000-000000000000'))
    WHERE status = 'ACTIVE';
```

#### `cufd_history`

```sql
CREATE TABLE cufd_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid NOT NULL REFERENCES points_of_sale(id) ON DELETE CASCADE,
    cufd text NOT NULL,
    direccion text NOT NULL,
    codigo_control varchar(100) NOT NULL,
    codigo_qr text,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, point_of_sale_id, cufd)
);
CREATE INDEX idx_cufd_history_active ON cufd_history(tenant_id, point_of_sale_id, valid_from, valid_to)
    WHERE is_active = true;
```

#### `cuis_history`

```sql
CREATE TABLE cuis_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid NOT NULL REFERENCES points_of_sale(id) ON DELETE CASCADE,
    cuis varchar(100) NOT NULL,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_cuis_history_active ON cuis_history(tenant_id, point_of_sale_id, valid_from, valid_to)
    WHERE is_active = true;
```

#### `contingency_events`

Registro de eventos significativos ante el SIAT para emisión en modo offline/contingencia.

```sql
CREATE TABLE contingency_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid NOT NULL REFERENCES points_of_sale(id) ON DELETE CASCADE,
    reason varchar(50) NOT NULL CHECK (reason IN (
        'FALTA_ENERGIA_ELECTRICA',
        'FALLA_CONEXION_INTERNET',
        'FALLA_SERVIDOR_SIN',
        'FALLA_SISTEMA_FACTURACION',
        'OTRO'
    )),
    description text,
    start_date timestamptz NOT NULL,
    end_date timestamptz NOT NULL,
    siat_event_code varchar(50),
    siat_reception_code varchar(100),
    cufd_id uuid NOT NULL REFERENCES cufd_history(id) ON DELETE RESTRICT,
    is_synced boolean NOT NULL DEFAULT false,
    synced_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_contingency_dates CHECK (end_date > start_date)
);
CREATE INDEX idx_contingency_events_pos ON contingency_events(tenant_id, point_of_sale_id, start_date);
CREATE INDEX idx_contingency_events_pending ON contingency_events(tenant_id, is_synced)
    WHERE is_synced = false;
```

#### `customers`

```sql
CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_type varchar(20) NOT NULL CHECK (document_type IN ('CI','CEX','PAS','NIT','OD')),
    document_number varchar(30) NOT NULL,
    complement varchar(10),
    name varchar(150) NOT NULL,
    email varchar(150),
    codigo_cliente varchar(50) NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, document_type, document_number, COALESCE(complement,''))
);
CREATE INDEX idx_customers_tenant ON customers(tenant_id);
```

#### `sin_products`

Catálogo de productos y servicios homologados por el SIAT, sincronizado por tenant.

```sql
CREATE TABLE sin_products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    codigo_actividad varchar(20) NOT NULL,
    codigo_producto_sin bigint NOT NULL,
    descripcion text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, codigo_actividad, codigo_producto_sin)
);
CREATE INDEX idx_sin_products_tenant ON sin_products(tenant_id);
```

#### `products` y `product_mappings`

Productos propios del tenant y su mapeo a códigos SIAT.

```sql
CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sku varchar(100) NOT NULL,
    name varchar(200) NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, sku)
);
CREATE INDEX idx_products_tenant ON products(tenant_id);

CREATE TABLE product_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sin_product_id uuid REFERENCES sin_products(id) ON DELETE SET NULL,
    codigo_producto_sin bigint NOT NULL,
    codigo_actividad varchar(20) NOT NULL,
    codigo_documento_sector int NOT NULL,
    unidad_medida int NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    synced_at timestamptz NOT NULL,
    UNIQUE(product_id, codigo_documento_sector, codigo_actividad)
);
CREATE INDEX idx_product_mappings_product ON product_mappings(product_id);
-- Garantiza un solo mapeo "default" por producto.
CREATE UNIQUE INDEX idx_product_mappings_one_default
    ON product_mappings(product_id)
    WHERE is_default = true;
```

#### Catálogos SIAT versionados

```sql
CREATE TABLE catalog_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    tipo varchar(50) NOT NULL,           -- 'tipo_metodo_pago', 'actividades', etc.
    version int NOT NULL,                -- secuencial por tenant+tipo
    synced_at timestamptz NOT NULL,
    source varchar(50) NOT NULL DEFAULT 'SIAT',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, tipo, version)
);

CREATE TABLE catalog_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id uuid NOT NULL REFERENCES catalog_versions(id) ON DELETE CASCADE,
    codigo varchar(50) NOT NULL,
    descripcion text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}',
    UNIQUE(version_id, codigo)
);
CREATE INDEX idx_catalog_items_version ON catalog_items(version_id);
```

#### `invoices`

```sql
CREATE TABLE invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid NOT NULL REFERENCES points_of_sale(id) ON DELETE RESTRICT,
    customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    cufd_id uuid NOT NULL REFERENCES cufd_history(id) ON DELETE RESTRICT,
    contingency_event_id uuid REFERENCES contingency_events(id) ON DELETE SET NULL,
    catalog_version_id uuid REFERENCES catalog_versions(id) ON DELETE SET NULL,
    idempotency_key varchar(100),
    invoice_number int NOT NULL,
    cuf varchar(150) UNIQUE,
    emission_type varchar(30) NOT NULL DEFAULT 'EN_LINEA' CHECK (emission_type IN ('EN_LINEA','OFFLINE','CONTINGENCIA')),
    codigo_metodo_pago int NOT NULL DEFAULT 1,
    codigo_moneda int NOT NULL DEFAULT 1,
    tipo_cambio decimal(18,5) NOT NULL DEFAULT 1,
    codigo_documento_sector int NOT NULL DEFAULT 1,
    layout varchar(80),
    modalidad int NOT NULL DEFAULT 1 CHECK (modalidad IN (1,2)),
    codigo_tipo_factura int NOT NULL DEFAULT 1,
    archivo text,
    hash_archivo varchar(100),
    sector_data jsonb,
    ajusta_factura_id uuid REFERENCES invoices(id) ON DELETE SET NULL,
    issue_date timestamptz NOT NULL,
    subtotal decimal(18,2) NOT NULL,
    discount decimal(18,2) NOT NULL DEFAULT 0,
    total decimal(18,2) NOT NULL,
    xml text,
    xml_hash varchar(100),
    siat_reception_code varchar(100),
    siat_mensajes text,
    status varchar(30) NOT NULL DEFAULT 'PENDING' CHECK (status IN (
        'DRAFT','PENDING','SUBMITTING','ACCEPTED','REJECTED','OBSERVED',
        'CANCELLED','OFFLINE'
    )),
    motivo_anulacion int,
    fecha_anulacion timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, point_of_sale_id, invoice_number)
);
CREATE INDEX idx_invoices_tenant ON invoices(tenant_id);
CREATE INDEX idx_invoices_pos_status ON invoices(point_of_sale_id, status);
CREATE INDEX idx_invoices_issue_date ON invoices(tenant_id, issue_date);
-- Índice único parcial: solo aplica cuando el cliente envió idempotency_key.
CREATE UNIQUE INDEX idx_invoices_idempotency
    ON invoices(tenant_id, point_of_sale_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

#### `invoice_items`

```sql
CREATE TABLE invoice_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id) ON DELETE SET NULL,
    code varchar(50) NOT NULL,
    description text NOT NULL,
    codigo_actividad varchar(20),
    codigo_producto_sin varchar(20),
    unit_code int,
    quantity decimal(18,3) NOT NULL,
    unit_price decimal(18,2) NOT NULL,
    discount decimal(18,2) NOT NULL DEFAULT 0,
    subtotal decimal(18,2) NOT NULL,
    sector_data jsonb
);
CREATE INDEX idx_invoice_items_invoice ON invoice_items(invoice_id);
```

#### `invoice_events`

```sql
CREATE TABLE invoice_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    type varchar(50) NOT NULL,
    message text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_invoice_events_invoice ON invoice_events(invoice_id);
```

#### `outbox`

```sql
CREATE TABLE outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    aggregate_type varchar(50) NOT NULL,   -- 'invoice'
    aggregate_id uuid NOT NULL,            -- invoice_id
    event_type varchar(50) NOT NULL,       -- 'invoice.submit'
    payload jsonb NOT NULL,
    headers jsonb NOT NULL DEFAULT '{}',
    attempts int NOT NULL DEFAULT 0,
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_outbox_pending ON outbox(processed_at, created_at)
    WHERE processed_at IS NULL;
```

---

## 3. Análisis de Problemas Actuales

### 3.1. Base de datos

| Problema | Severidad | Por qué es malo |
|---|---|---|
| `float64` para montos fiscales | Alta | Pérdida de precisión decimal en cálculos fiscales. |
| Sin CHECK constraints | Alta | La BD acepta cualquier string como estado/ambiente. |
| FK implícitas (solo GORM) | Alta | No hay integridad referencial a nivel de BD. |
| `Catalog` sin índice único | Media | Duplicados posibles por `(company_id, tipo, codigo)`. |
| `Catalog.Replace` borra e inserta | Alta | Si falla tras el DELETE, el catálogo queda vacío. |
| `ProductMapping` redundante | Media | Guarda `SinProductId` + `CodigoProductoSin` + `CodigoActividad`. |
| CUIS como campos sueltos en POS | Media | Sin historial ni vigencia explícita. |
| `Invoice.IdempotencyKey` nullable + unique permite múltiples NULL | Media | Varias facturas sin key pueden coexistir; no es bug grave, pero es confuso. |
| Numeración con advisory lock + MAX() | Alta | Serializa por punto de venta; cuello de botella. |
| Migraciones con `log.Fatalf` | Alta | Error en migración mata el proceso sin rollback controlado. |
| Tests de postgres no compilan | Crítica | No hay seguridad en la capa de persistencia. |

### 3.2. Multi-tenancy

| Problema | Severidad | Por qué es malo |
|---|---|---|
| API key única global | Alta | No hay keys por tenant; cualquiera con la key accede a todo. |
| `InjectCompanyID` lee header `X-Company-Id` | Alta | El cliente puede cambiar de tenant libremente. |
| `customer_repo.List("")` devuelve todo | Crítica | Brecha de seguridad real. |
| `invoice_repo.GetByID` no filtra por tenant | Alta | Con un ID puedes leer facturas de otro tenant. |
| Logs de PII en customer_repo | Alta | Datos fiscales de clientes en logs. |
| Sin tabla `api_keys` | Alta | No hay rotación, scopes ni revocación. |

### 3.3. Arquitectura y SIAT

| Problema | Severidad | Por qué es malo |
|---|---|---|
| Usecases construyen `siat.Solicitud*` | Crítica | El SIAT penetró el dominio. |
| `extras ...any` en constructores | Alta | Tipado roto, tests difíciles. |
| God classes (`InvoiceUsecase`, `SiatUsecase`) | Alta | Decenas de responsabilidades mezcladas. |
| No hay máquina de estados | Alta | Estados inconsistentes posibles. |
| Emisión síncrona en request HTTP | Alta | Timeout, sin reintentos, colapsa bajo carga. |
| PDF con goroutine suelta | Media | Sin control de concurrencia. |
| Sin outbox | Alta | No hay atomicidad entre BD y envío SIAT. |

---

## 4. Fases de Ejecución

Cada fase mejora el sistema de forma tangible y acumulativa.

### Fase 0a — Reparar tests de PostgreSQL (cimiento)

**Mejora:** confianza para refactorizar.

- [ ] Reparar `internal/repository/postgres/invoice_repo_test.go`.
  - Eliminar referencias a `models.Cuis`.
  - Corregir `&f.customer.ID` → `f.customer.ID`.
- [ ] Agregar tests básicos en `internal/storage`.
- [ ] Asegurar `go test ./...` verde.
- [ ] Configurar CI con `TEST_DATABASE_URL`.

**Entregable:** suite verde y confiable.

---

### Fase 0b — API keys por tenant (paralelo a legacy)

**Mejora:** seguridad multi-tenant sin breaking change day 1.

- [ ] Crear tabla `api_keys`.
- [ ] Crear middleware que resuelva `tenant_id` desde `X-API-Key`.
- [ ] Mantener `X-Company-Id` como fallback deprecado (con log de advertencia).
- [ ] Filtrar siempre por tenant en repositorios (`GetByID`, `List`, etc.).
- [ ] Agregar tests de seguridad: un tenant no puede ver datos de otro.

**Entregable:** autenticación por API key funcional; legacy aún compatible.

---

### Fase 0c — Remover header legacy y limpiar PII

**Mejora:** hardening de seguridad.

- [ ] Eliminar header `X-Company-Id` como mecanismo de tenant.
- [ ] Quitar logs de PII en `customer_repo.go` y otros repositorios.
- [ ] Auditar todos los `log.Println`/`slog` que impriman datos sensibles.
- [ ] Validar que ningún endpoint exponga datos sin filtrar por tenant.

**Entregable:** único mecanismo de tenant es API key; sin filtrado de PII.

---

### Fase 1 — Container y limpieza de `main.go`

**Mejora:** escalabilidad del código, velocidad de desarrollo.

- [ ] Crear `internal/app/container.go` con lazy factories.
- [ ] Extraer wiring desde `cmd/server/main.go`.
- [ ] Eliminar `database.DB` global.
- [ ] Reorganizar imports.

**Entregable:** `main.go` < 50 líneas; agregar un dominio nuevo toca 1 archivo.

---

### Fase 2 — Módulos autónomos de rutas

**Mejora:** bajo acoplamiento, alta cohesión.

- [ ] Definir contrato `Module`.
- [ ] Migrar dominios a `internal/delivery/http/modules/<dominio>`.
- [ ] Eliminar struct `Handlers`.
- [ ] Mantener rutas legacy.

**Entregable:** router coordinador puro.

---

### Fase 3 — Constructores explícitos

**Mejora:** tipado, testabilidad, claridad.

- [ ] Eliminar `extras ...any`.
- [ ] Handlers dependen de interfaces.
- [ ] Actualizar tests.

**Entregable:** código compilado sin `interface{}` en DI.

---

### Fase 4 — Puerto `FiscalService` + SIAT sandbox

**Mejora:** el dominio deja de depender del SDK; tests determinísticos.

- [ ] Crear `internal/ports/fiscal.go`.
- [ ] Mover `internal/siat` a `internal/adapters/siat`.
- [ ] Crear `FiscalDocument`, `FiscalResult`, etc. en dominio.
- [ ] Mover todo el mapping al adaptador.
- [ ] Crear `adapters/siat/fake` o `adapters/siat/sandbox`: implementación de `FiscalService` que devuelve respuestas determinísticas sin llamar al SIAT real.
- [ ] Configurar el sandbox como default en CI y desarrollo local.

**Entregable:** `application` no importa `go-siat`; tests de contrato corren sin SIAT real.

---

### Fase 5 — Base de datos normalizada (Parte 1: estructura)

**Mejora:** integridad, performance, multi-tenancy.

- [ ] Migrar a `golang-migrate`.
- [ ] Crear tablas `tenants`, `api_keys`, `branches`, `points_of_sale`, `certificates`.
- [ ] Crear tablas `cufd_history`, `cuis_history`, `contingency_events`.
- [ ] Normalizar catálogos a `catalog_versions` + `catalog_items`.
- [ ] Migrar datos existentes.

**Entregable:** esquema relacional sólido con FK y CHECK.

---

### Fase 6 — Base de datos normalizada (Parte 2: facturas y eventos)

**Mejora:** trazabilidad, estados consistentes, precisión decimal.

- [ ] Cambiar montos a `decimal`.
- [ ] Renombrar `companies` → `tenants` y agregar `tenant_id` a todas las tablas.
- [ ] Crear `invoice_events`.
- [ ] Tabla `invoice_documents` para XML/archivo/historial.
- [ ] Reemplazar numeración MAX()+lock por tabla de secuencias.

**Entregable:** facturas con trazabilidad completa y numeración escalable.

---

### Fase 7 — Máquina de estados

**Mejora:** estados consistentes, menos bugs.

- [ ] Definir estados y transiciones.
- [ ] Implementar `InvoiceStateMachine`.
- [ ] Validar transiciones a nivel de aplicación (ej. `CANCELLED` no puede volver a `ACCEPTED`).
- [ ] Reforzar transiciones críticas con trigger opcional en BD si el equipo lo considera necesario.
- [ ] Reemplazar `ClaimStatus`/`ClaimForEmission` por máquina.

> Nota: el enum de `status` se valida con el `CHECK` de columna en `invoices`. La validación de *transiciones* entre estados vive en la aplicación, no en un CHECK duplicado.

**Entregable:** transiciones imposibles de realizar por error.

---

### Fase 8 — Renovación automática de CUFD/CUIS + alertas de certificados

**Mejora:** el tenant nunca se queda sin credenciales vigentes, y la cola de emisión se prueba con CUFDs reales. Además, se alerta antes de que venza el certificado P12 (que no se renueva automáticamente).

- [ ] Crear job programado que revise CUFD próximos a vencer (~4h antes del vencimiento).
- [ ] Crear job programado que renueve CUIS cuando esté próximo a vencer.
- [ ] Antes de encolar una factura, validar CUFD vigente; si no, renovar primero.
- [ ] Alertas/métricas cuando la renovación falle.
- [ ] Job diario que revise `certificates.not_after` a 30, 15 y 7 días del vencimiento.
- [ ] Notificar al tenant vía webhook o email en cada umbral.
- [ ] Auto-transicionar certificados vencidos a `status = 'EXPIRED'` y alertar internamente si un certificado `ACTIVE` ya pasó `not_after`.
- [ ] Tests de escenario: "factura en cola espera CUFD"; "certificado a 7 días de vencer dispara alerta".

> **Nota técnica:** los jobs de esta fase usan un scheduler liviano propio (cron/ticker) para no depender de River, que se elige en Fase 9. Si finalmente se usa River para todo, Fase 8 y Fase 9 se planifican juntas, aunque se documenten por separado.

**Entregable:** credenciales SIAT siempre vigentes; certificados P12 monitoreados; fallback controlado si falla renovación.

---

### Fase 9 — Outbox + cola de emisión + rate limiting

**Mejora:** confiabilidad, reintentos, escalabilidad, fairness entre tenants.

- [ ] Crear tabla `outbox`.
- [ ] Elegir cola (recomendado **River** sobre PostgreSQL).
- [ ] Modificar `Emit` para encolar en outbox.
- [ ] Implementar worker con backoff y circuit breaker.
- [ ] Implementar rate limiting por tenant hacia el SIAT (token bucket por NIT/tenant).
- [ ] Mover PDF a worker.

**Entregable:** emisión asíncrona; cliente recibe 202 Accepted; un tenant no monopoliza el SIAT.

---

### Fase 10 — Payloads mínimos + versionado API `/v1/`

**Mejora:** integración en minutos, contrato público estable.

- [ ] Versionar todas las rutas bajo `/v1/`.
- [ ] Crear `InvoiceRequestSimplifier`.
- [ ] Inferir campos SIAT desde catálogos.
- [ ] Aliases legibles.
- [ ] Autocompletado de productos/clientes.
- [ ] Endpoint `POST /v1/invoices/preview`.
- [ ] Crear `POST /v1/invoices/emit`.
- [ ] Documentar contrato público como estable.

**Entregable:** payload mínimo de 6 campos; API versionada.

---

### Fase 11 — Observabilidad y errores que enseñan

**Mejora:** soporte reducido, experiencia fluida.

- [ ] Error mapper con `code`, `field`, `sugerencias`, `accion`.
- [ ] Métricas Prometheus.
- [ ] Dashboards básicos.
- [ ] Logs estructurados sin PII.

**Entregable:** errores autodescriptivos + métricas.

---

### Fase 12 — Tests de calidad

**Mejora:** seguridad para evolucionar.

- [ ] Cobertura HTTP > 70%.
- [ ] Cobertura application > 70%.
- [ ] Tests de integración con PostgreSQL confiables.
- [ ] Tests de contrato del adaptador SIAT (contra sandbox).
- [ ] Tests de concurrencia y cola.
- [ ] Tests de rate limiting entre tenants.

**Entregable:** suite robusta.

---

### Fase 13 — SDK y documentación

**Mejora:** adopción del producto.

- [ ] SDK TypeScript.
- [ ] Documentación de onboarding.
- [ ] Guías de migración.
- [ ] Ejemplos de integración.

**Entregable:** SDK alpha publicado.

---

## 5. Criterios de Aceptación

### Por fase

- **Fase 0a:** `go test ./...` verde.
- **Fase 0b:** autenticación por API key funcional; legacy aún compatible.
- **Fase 0c:** único mecanismo de tenant es API key; sin filtrado de PII.
- **Fase 1:** `main.go` < 50 líneas.
- **Fase 2:** agregar endpoint a dominio existente toca 1 archivo.
- **Fase 3:** ningún constructor usa `...any`.
- **Fase 4:** `application` no importa `go-siat`; sandbox usable en CI.
- **Fase 5:** migraciones versionadas; CHECK y FK en BD.
- **Fase 6:** montos en `decimal`; facturas con eventos.
- **Fase 7:** máquina de estados con tests de propiedad.
- **Fase 8:** CUFD/CUIS se renuevan automáticamente; certificados P12 alertan antes de vencer; alerta si falla renovación.
- **Fase 9:** emisión devuelve 202; worker con reintentos; rate limit por tenant.
- **Fase 10:** POST /v1/invoices con ≤ 6 campos.
- **Fase 11:** métricas de emisión disponibles.
- **Fase 12:** cobertura global > 70%.
- **Fase 13:** SDK alpha funcional.

---

## 6. Notas sobre Multi-tenancy SaaS

### API keys por tenant

Cada tenant puede tener múltiples API keys:

```
X-API-Key: sup_live_xxxxxxxx
```

El middleware:
1. Hashea la key.
2. Busca en `api_keys`.
3. Valida que esté activa y no expirada.
4. Inyecta `tenant_id` en el contexto.
5. Todos los repositorios filtran por `tenant_id`.

### Onboarding de un nuevo tenant

```http
POST /tenants
{
  "nit": "123456789",
  "business_name": "Mi Empresa",
  "codigo_sistema": "...",
  "ambiente": "PILOTO",
  "codigo_modalidad": 1
}

# Respuesta incluye API key de administrador
```

El endpoint crea `tenants` (identidad) y `tenant_configs` (configuración operativa) en la misma transacción. Fuente de verdad:
- `tenants`: NIT, razón social, datos de contacto.
- `tenant_configs`: ambiente, código de sistema, modalidad, tokens, límites.

Luego el tenant:
1. Crea sucursal.
2. Crea punto de venta.
3. Sube certificado.
4. Sincroniza catálogos.
5. Solicita CUIS/CUFD.
6. Empieza a facturar.

### Límites y cuotas (futuro)

```sql
CREATE TABLE tenant_plans (
    tenant_id uuid PRIMARY KEY REFERENCES tenants(id),
    plan varchar(50) NOT NULL,
    max_invoices_monthly int,
    max_points_of_sale int,
    max_api_keys int
);
```

### Seguridad

- Cifrado AES-GCM de tokens y P12 (ya existe).
- API keys hasheadas con bcrypt/argon2.
- Rotación de keys.
- Scopes por key (read, write, admin).
- Auditoría de eventos.

---

## 7. Riesgos y Mitigaciones

| Riesgo | Mitigación |
|---|---|
| Migración masiva de BD | Fases 5 y 6 separadas; backups; migraciones reversibles. |
| Cambiar modelo de tenant | Fase 0 primero; feature flag para nuevo auth. |
| Perder compatibilidad SIAT | Mantener tests de payload XML durante todo el proceso. |
| Performance de cola | Monitorear; escalar workers; PostgreSQL puede soportar River. |
| Resistencia al cambio | Fases pequeñas; valor visible en cada una. |

---

## 8. Próximos Pasos

1. Aprobar este plan definitivo.
2. **Fase 0a:** reparar tests de PostgreSQL (`go test ./...` verde).
3. **Fase 0b:** crear tabla `api_keys` y middleware que resuelva tenant desde `X-API-Key` (manteniendo compatibilidad legacy).
4. **Fase 0c:** eliminar header `X-Company-Id` y limpiar logs de PII.
5. **Fases 1-3:** container, módulos autónomos, constructores explícitos (pueden ir en paralelo).
6. No avanzar a Fase 4 sin que las anteriores estén verdes.

---

*Plan maestro definitivo para Supay API — SaaS multi-tenant de facturación electrónica SIAT Bolivia.*

-- Supay · Esquema de referencia (PostgreSQL 15+)
-- Las tablas se crean y mantienen con GORM AutoMigrate; este archivo es solo
-- referencia documental de la estructura y sus índices.
--
-- Tablas relacionadas con el flujo Sucursales + Puntos de Venta + CUIS + CUFD.

CREATE TABLE IF NOT EXISTS branches (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL REFERENCES companies(id),
    codigo_sucursal INTEGER NOT NULL,
    name            VARCHAR(150) NOT NULL,
    address         TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_company_sucursal UNIQUE (company_id, codigo_sucursal),
    CONSTRAINT ck_sucursal_non_negative CHECK (codigo_sucursal >= 0)
);

-- Punto de venta local. Status: CREATING -> OPERATIVE | ERROR
CREATE TABLE IF NOT EXISTS point_of_sales (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id         UUID NOT NULL REFERENCES companies(id),
    branch_id          UUID REFERENCES branches(id),
    codigo_sucursal    INTEGER NOT NULL DEFAULT 0,
    codigo_punto_venta INTEGER NOT NULL,
    description        VARCHAR(150) NOT NULL,
    cuis               VARCHAR(100),
    cuis_created_at    TIMESTAMPTZ,
    is_active          BOOLEAN NOT NULL DEFAULT TRUE,
    siat_code          INTEGER,
    status             VARCHAR(50) NOT NULL DEFAULT 'CREATING',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- datos del registro oficial ante el SIAT (registroPuntoVenta)
    tipo_punto_venta   INTEGER,
    siat_transaccion   BOOLEAN NOT NULL DEFAULT FALSE,
    siat_registered_at TIMESTAMPTZ,
    siat_response      JSONB,
    siat_error         TEXT,
    CONSTRAINT uq_company_sucursal_pv UNIQUE (company_id, codigo_sucursal, codigo_punto_venta)
);

CREATE TABLE IF NOT EXISTS tipo_punto_ventas (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID NOT NULL REFERENCES companies(id),
    codigo_clasificador INTEGER NOT NULL,
    descripcion         VARCHAR(200) NOT NULL,
    synced_at           TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_company_tipo_pv UNIQUE (company_id, codigo_clasificador)
);

CREATE TABLE IF NOT EXISTS cuis (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    point_of_sale_id UUID NOT NULL REFERENCES point_of_sales(id),
    cuis             VARCHAR(200) NOT NULL,
    valid_from       TIMESTAMPTZ NOT NULL,
    valid_to         TIMESTAMPTZ NOT NULL,
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cuis_pos ON cuis (point_of_sale_id);

-- Catálogos sincronizados del SIAT (FacturacionSincronizacion). Cada fila es un
-- elemento codigo+descripcion de un catálogo; `tipo` identifica la operación de
-- origen (p.ej. tipoMoneda, tipoMetodoPago, unidadMedida, actividades, ...).
-- Se reemplazan por completo (delete + insert) en cada sincronización, por lo que
-- no requieren constraint único.
CREATE TABLE IF NOT EXISTS catalogs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   UUID NOT NULL REFERENCES companies(id),
    tipo         VARCHAR(50) NOT NULL,
    codigo       INTEGER NOT NULL,
    descripcion  TEXT NOT NULL,
    synced_at    TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_catalog_company_tipo ON catalogs (company_id, tipo);

-- Catálogo de actividades económicas (sincronizarActividades). codigo_caeb es
-- VARCHAR porque el SIAT lo transmite como texto; tipo_actividad distingue
-- actividades principales ("P") de secundarias.
CREATE TABLE IF NOT EXISTS siat_actividades (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id     UUID NOT NULL REFERENCES companies(id),
    codigo_caeb    VARCHAR(20) NOT NULL,
    descripcion    TEXT NOT NULL,
    tipo_actividad VARCHAR(10) NOT NULL DEFAULT '',
    synced_at      TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_company_caeb UNIQUE (company_id, codigo_caeb)
);

-- Leyendas de factura por actividad económica
-- (sincronizarListaLeyendasFactura).
CREATE TABLE IF NOT EXISTS siat_leyendas_factura (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID NOT NULL REFERENCES companies(id),
    codigo_actividad    VARCHAR(20) NOT NULL,
    descripcion_leyenda TEXT NOT NULL,
    synced_at           TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_company_leyenda_act ON siat_leyendas_factura (company_id, codigo_actividad);

-- Relación actividad ↔ documento-sector
-- (sincronizarListaActividadesDocumentoSector). Resuelve el documento-sector a
-- usar para emitir según la actividad del contribuyente.
CREATE TABLE IF NOT EXISTS siat_actividades_doc_sector (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id              UUID NOT NULL REFERENCES companies(id),
    codigo_actividad        VARCHAR(20) NOT NULL,
    codigo_documento_sector INTEGER NOT NULL,
    tipo_documento_sector   VARCHAR(20) NOT NULL DEFAULT '',
    synced_at               TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_company_act_sector UNIQUE (company_id, codigo_actividad, codigo_documento_sector)
);

CREATE TABLE IF NOT EXISTS cufds (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    point_of_sale_id UUID NOT NULL REFERENCES point_of_sales(id),
    cufd             TEXT NOT NULL,
    direccion        TEXT NOT NULL,
    codigo_control   VARCHAR(100) NOT NULL,
    valid_from       TIMESTAMPTZ NOT NULL,
    valid_to         TIMESTAMPTZ NOT NULL,
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cufd_pos_valid ON cufds (point_of_sale_id, valid_from);

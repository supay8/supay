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

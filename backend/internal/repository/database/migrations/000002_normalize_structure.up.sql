-- Structural normalization (phase 5). Business tables intentionally retain
-- company_id until phase 6; only the structural/catalog tables move now.
ALTER TABLE companies RENAME TO tenants;

CREATE TABLE tenant_configs (
    tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    ambiente varchar(20) NOT NULL DEFAULT 'PILOTO',
    codigo_sistema varchar(100) NOT NULL,
    codigo_modalidad integer NOT NULL DEFAULT 1,
    token_siat text,
    api_token text,
    max_invoices_monthly integer NOT NULL DEFAULT 1000,
    max_points_of_sale integer NOT NULL DEFAULT 5,
    max_api_keys integer NOT NULL DEFAULT 10,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_tenant_configs_ambiente CHECK (ambiente IN ('PILOTO', 'PRODUCCION')),
    CONSTRAINT chk_tenant_configs_modalidad CHECK (codigo_modalidad IN (1, 2)),
    CONSTRAINT chk_tenant_configs_limits CHECK (
        max_invoices_monthly > 0 AND max_points_of_sale > 0 AND max_api_keys > 0
    )
);

INSERT INTO tenant_configs (tenant_id, ambiente, codigo_sistema, codigo_modalidad, created_at, updated_at)
SELECT id, ambiente, codigo_sistema, modalidad, created_at, updated_at
FROM tenants
ON CONFLICT (tenant_id) DO NOTHING;

ALTER TABLE tenants
    DROP COLUMN ambiente,
    DROP COLUMN codigo_sistema,
    DROP COLUMN modalidad;

ALTER TABLE api_keys RENAME COLUMN company_id TO tenant_id;
ALTER TABLE branches RENAME COLUMN company_id TO tenant_id;
ALTER TABLE branches RENAME COLUMN active TO is_active;
ALTER TABLE point_of_sales RENAME TO points_of_sale;
ALTER TABLE points_of_sale RENAME COLUMN company_id TO tenant_id;
ALTER TABLE certificates RENAME COLUMN company_id TO tenant_id;
ALTER TABLE catalog_sync_states RENAME COLUMN company_id TO tenant_id;
ALTER TABLE sin_products RENAME COLUMN company_id TO tenant_id;
ALTER TABLE sin_products RENAME COLUMN active TO is_active;

ALTER TABLE cufds RENAME TO cufd_history;
ALTER TABLE cufd_history RENAME COLUMN active TO is_active;
ALTER TABLE cufd_history ADD COLUMN tenant_id uuid;
UPDATE cufd_history c
SET tenant_id = p.tenant_id
FROM points_of_sale p
WHERE p.id = c.point_of_sale_id;
ALTER TABLE cufd_history ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE cufd_history
    ADD CONSTRAINT fk_cufd_history_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT chk_cufd_history_dates CHECK (valid_to > valid_from),
    ADD CONSTRAINT uq_cufd_history_tenant_pos_cufd UNIQUE (tenant_id, point_of_sale_id, cufd);

CREATE TABLE cuis_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid NOT NULL REFERENCES points_of_sale(id) ON DELETE CASCADE,
    cuis varchar(100) NOT NULL,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_cuis_history_dates CHECK (valid_to IS NULL OR valid_to > valid_from)
);

INSERT INTO cuis_history (tenant_id, point_of_sale_id, cuis, valid_from, is_active, created_at)
SELECT tenant_id, id, cuis, COALESCE(cuis_created_at, created_at), true, COALESCE(cuis_created_at, created_at)
FROM points_of_sale
WHERE cuis IS NOT NULL AND cuis <> '';

CREATE OR REPLACE FUNCTION record_point_of_sale_cuis() RETURNS trigger AS $$
BEGIN
    IF NEW.cuis IS NOT NULL AND NEW.cuis <> '' AND NEW.cuis IS DISTINCT FROM OLD.cuis THEN
        UPDATE cuis_history
        SET is_active = false, valid_to = COALESCE(NEW.cuis_created_at, now())
        WHERE point_of_sale_id = NEW.id AND is_active = true;

        INSERT INTO cuis_history (tenant_id, point_of_sale_id, cuis, valid_from)
        VALUES (NEW.tenant_id, NEW.id, NEW.cuis, COALESCE(NEW.cuis_created_at, now()));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_point_of_sale_cuis_history
AFTER UPDATE OF cuis ON points_of_sale
FOR EACH ROW EXECUTE FUNCTION record_point_of_sale_cuis();

ALTER TABLE contingency_events ADD COLUMN tenant_id uuid;
ALTER TABLE contingency_events ADD COLUMN siat_reception_code varchar(100);
ALTER TABLE contingency_events ADD COLUMN cufd_id uuid;
ALTER TABLE contingency_events ADD COLUMN synced_at timestamptz;
UPDATE contingency_events e
SET tenant_id = p.tenant_id
FROM points_of_sale p
WHERE p.id = e.point_of_sale_id;
UPDATE contingency_events e
SET cufd_id = (
    SELECT c.id
    FROM cufd_history c
    WHERE c.point_of_sale_id = e.point_of_sale_id
      AND c.valid_from <= e.start_date
      AND c.valid_to >= COALESCE(e.end_date, e.start_date)
    ORDER BY c.is_active DESC, c.created_at DESC
    LIMIT 1
)
WHERE e.cufd_id IS NULL;
ALTER TABLE contingency_events ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE contingency_events
    ADD CONSTRAINT fk_contingency_events_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_contingency_events_cufd FOREIGN KEY (cufd_id) REFERENCES cufd_history(id) ON DELETE RESTRICT,
    ADD CONSTRAINT chk_contingency_reason CHECK (reason IN (
        'FALTA_ENERGIA_ELECTRICA', 'FALLA_CONEXION_INTERNET',
        'FALLA_SERVIDOR_SIN', 'FALLA_SISTEMA_FACTURACION', 'OTRO'
    )),
    ADD CONSTRAINT chk_contingency_dates CHECK (end_date IS NULL OR end_date > start_date);

CREATE TABLE catalog_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    tipo varchar(50) NOT NULL,
    version integer NOT NULL,
    synced_at timestamptz NOT NULL,
    source varchar(50) NOT NULL DEFAULT 'SIAT',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_catalog_version_positive CHECK (version > 0),
    CONSTRAINT uq_catalog_versions_tenant_type_version UNIQUE (tenant_id, tipo, version)
);

CREATE TABLE catalog_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id uuid NOT NULL REFERENCES catalog_versions(id) ON DELETE CASCADE,
    codigo varchar(100) NOT NULL,
    descripcion text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT uq_catalog_items_version_code UNIQUE (version_id, codigo)
);

-- productosServicios had two stores. Consolidate it into the dedicated table.
INSERT INTO sin_products (
    id, tenant_id, codigo_actividad, codigo_producto_sin, descripcion,
    is_active, synced_at, created_at, updated_at
)
SELECT DISTINCT ON (company_id, codigo)
    gen_random_uuid(), company_id, 0, codigo, descripcion, true,
    synced_at, created_at, now()
FROM catalogs
WHERE tipo = 'productosServicios'
ORDER BY company_id, codigo, created_at DESC
ON CONFLICT (tenant_id, codigo_actividad, codigo_producto_sin) DO UPDATE
SET descripcion = EXCLUDED.descripcion,
    synced_at = EXCLUDED.synced_at,
    is_active = true,
    updated_at = now();

INSERT INTO catalog_versions (tenant_id, tipo, version, synced_at)
SELECT company_id, tipo, 1, max(synced_at)
FROM catalogs
WHERE tipo <> 'productosServicios'
GROUP BY company_id, tipo;

INSERT INTO catalog_items (version_id, codigo, descripcion)
SELECT DISTINCT ON (c.company_id, c.tipo, c.codigo)
    v.id, c.codigo::text, c.descripcion
FROM catalogs c
JOIN catalog_versions v ON v.tenant_id = c.company_id AND v.tipo = c.tipo AND v.version = 1
WHERE c.tipo <> 'productosServicios'
ORDER BY c.company_id, c.tipo, c.codigo, c.created_at DESC
ON CONFLICT (version_id, codigo) DO UPDATE SET descripcion = EXCLUDED.descripcion;

INSERT INTO catalog_versions (tenant_id, tipo, version, synced_at)
SELECT company_id, 'tipoPuntoVenta', 1, max(synced_at)
FROM tipo_punto_ventas GROUP BY company_id;
INSERT INTO catalog_items (version_id, codigo, descripcion)
SELECT v.id, p.codigo_clasificador::text, p.descripcion
FROM tipo_punto_ventas p
JOIN catalog_versions v ON v.tenant_id = p.company_id AND v.tipo = 'tipoPuntoVenta' AND v.version = 1
ON CONFLICT (version_id, codigo) DO UPDATE SET descripcion = EXCLUDED.descripcion;

INSERT INTO catalog_versions (tenant_id, tipo, version, synced_at)
SELECT company_id, 'actividades', 1, max(synced_at)
FROM siat_actividades GROUP BY company_id;
INSERT INTO catalog_items (version_id, codigo, descripcion, metadata)
SELECT v.id, a.codigo_caeb, a.descripcion, jsonb_build_object('tipo_actividad', a.tipo_actividad)
FROM siat_actividades a
JOIN catalog_versions v ON v.tenant_id = a.company_id AND v.tipo = 'actividades' AND v.version = 1
ON CONFLICT (version_id, codigo) DO UPDATE
SET descripcion = EXCLUDED.descripcion, metadata = EXCLUDED.metadata;

INSERT INTO catalog_versions (tenant_id, tipo, version, synced_at)
SELECT company_id, 'leyendasFactura', 1, max(synced_at)
FROM siat_leyendas_factura GROUP BY company_id;
INSERT INTO catalog_items (version_id, codigo, descripcion, metadata)
SELECT v.id,
       l.codigo_actividad || ':' || md5(l.descripcion_leyenda),
       l.descripcion_leyenda,
       jsonb_build_object('codigo_actividad', l.codigo_actividad)
FROM siat_leyendas_factura l
JOIN catalog_versions v ON v.tenant_id = l.company_id AND v.tipo = 'leyendasFactura' AND v.version = 1
ON CONFLICT (version_id, codigo) DO UPDATE
SET descripcion = EXCLUDED.descripcion, metadata = EXCLUDED.metadata;

INSERT INTO catalog_versions (tenant_id, tipo, version, synced_at)
SELECT company_id, 'actividadesDocumentoSector', 1, max(synced_at)
FROM siat_actividades_doc_sector GROUP BY company_id;
INSERT INTO catalog_items (version_id, codigo, descripcion, metadata)
SELECT v.id,
       d.codigo_actividad || ':' || d.codigo_documento_sector::text,
       d.tipo_documento_sector,
       jsonb_build_object(
           'codigo_actividad', d.codigo_actividad,
           'codigo_documento_sector', d.codigo_documento_sector,
           'tipo_documento_sector', d.tipo_documento_sector
       )
FROM siat_actividades_doc_sector d
JOIN catalog_versions v ON v.tenant_id = d.company_id
 AND v.tipo = 'actividadesDocumentoSector' AND v.version = 1
ON CONFLICT (version_id, codigo) DO UPDATE
SET descripcion = EXCLUDED.descripcion, metadata = EXCLUDED.metadata;

DROP TABLE catalogs;
DROP TABLE tipo_punto_ventas;
DROP TABLE siat_actividades;
DROP TABLE siat_leyendas_factura;
DROP TABLE siat_actividades_doc_sector;

DROP INDEX IF EXISTS idx_company_sucursal;
DROP INDEX IF EXISTS idx_company_sucursal_pv;
DROP INDEX IF EXISTS idx_api_keys_company;
DROP INDEX IF EXISTS idx_cert_company;
DROP INDEX IF EXISTS idx_company_sin_product;
DROP INDEX IF EXISTS idx_cufd_pos_valid;
DROP INDEX IF EXISTS idx_contingency_pos_start;

CREATE UNIQUE INDEX idx_branches_tenant_code ON branches(tenant_id, codigo_sucursal);
CREATE UNIQUE INDEX idx_points_of_sale_tenant_codes ON points_of_sale(tenant_id, codigo_sucursal, codigo_punto_venta);
CREATE INDEX idx_api_keys_tenant ON api_keys(tenant_id);
CREATE INDEX idx_points_of_sale_tenant ON points_of_sale(tenant_id);
CREATE INDEX idx_certificates_tenant ON certificates(tenant_id);
CREATE INDEX idx_certificates_active ON certificates(tenant_id, status) WHERE status = 'ACTIVE';
CREATE INDEX idx_cufd_history_active
    ON cufd_history(tenant_id, point_of_sale_id, valid_from, valid_to) WHERE is_active = true;
CREATE UNIQUE INDEX idx_cuis_history_one_active
    ON cuis_history(tenant_id, point_of_sale_id) WHERE is_active = true;
CREATE INDEX idx_cuis_history_active
    ON cuis_history(tenant_id, point_of_sale_id, valid_from, valid_to) WHERE is_active = true;
CREATE INDEX idx_contingency_events_pos
    ON contingency_events(tenant_id, point_of_sale_id, start_date);
CREATE INDEX idx_contingency_events_pending
    ON contingency_events(tenant_id, is_synced) WHERE is_synced = false;
CREATE INDEX idx_catalog_versions_current
    ON catalog_versions(tenant_id, tipo, version DESC);
CREATE INDEX idx_catalog_items_version ON catalog_items(version_id);
CREATE UNIQUE INDEX idx_sin_products_tenant_code
    ON sin_products(tenant_id, codigo_actividad, codigo_producto_sin);

ALTER TABLE tenants
    ADD CONSTRAINT chk_tenants_nit_not_blank CHECK (btrim(nit) <> '');
ALTER TABLE points_of_sale
    ADD CONSTRAINT chk_points_of_sale_codes CHECK (codigo_sucursal >= 0 AND codigo_punto_venta >= 0);
ALTER TABLE certificates
    ADD COLUMN point_of_sale_id uuid REFERENCES points_of_sale(id) ON DELETE SET NULL,
    ADD COLUMN encrypted_blob bytea,
    ADD COLUMN encrypted_password text,
    ADD COLUMN serial_number varchar(100),
    ADD COLUMN is_active boolean NOT NULL DEFAULT true,
    ADD COLUMN uploaded_at timestamptz NOT NULL DEFAULT now(),
    ADD CONSTRAINT chk_certificate_status CHECK (status IN ('ACTIVE', 'EXPIRED', 'REVOKED', 'PENDING')),
    ADD CONSTRAINT chk_certificate_dates CHECK (not_after > not_before);

-- Replace GORM's default NO ACTION foreign keys on phase-5 tables with the
-- explicit lifecycle rules from the target schema.
DO $$
DECLARE fk record;
BEGIN
    FOR fk IN
        SELECT conrelid::regclass AS table_name, conname
        FROM pg_constraint
        WHERE contype = 'f'
          AND conrelid IN (
              'api_keys'::regclass, 'branches'::regclass, 'points_of_sale'::regclass,
              'certificates'::regclass, 'cufd_history'::regclass,
              'contingency_events'::regclass, 'catalog_sync_states'::regclass,
              'sin_products'::regclass
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.table_name, fk.conname);
    END LOOP;
END $$;

ALTER TABLE api_keys ADD CONSTRAINT fk_api_keys_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE branches ADD CONSTRAINT fk_branches_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE points_of_sale
    ADD CONSTRAINT fk_points_of_sale_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_points_of_sale_branch FOREIGN KEY (branch_id) REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE certificates
    ADD CONSTRAINT fk_certificates_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_certificates_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE SET NULL;
ALTER TABLE cufd_history
    ADD CONSTRAINT fk_cufd_history_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_cufd_history_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE;
ALTER TABLE contingency_events
    ADD CONSTRAINT fk_contingency_events_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_contingency_events_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_contingency_events_cufd FOREIGN KEY (cufd_id) REFERENCES cufd_history(id) ON DELETE RESTRICT;
ALTER TABLE catalog_sync_states
    ADD CONSTRAINT fk_catalog_sync_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_catalog_sync_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE;
ALTER TABLE sin_products ADD CONSTRAINT fk_sin_products_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id, point_of_sale_id ORDER BY created_at DESC, id) AS row_number
    FROM certificates WHERE status = 'ACTIVE'
)
UPDATE certificates c
SET status = 'EXPIRED', is_active = false
FROM ranked r
WHERE c.id = r.id AND r.row_number > 1;
CREATE UNIQUE INDEX idx_certificates_one_active_per_pos
    ON certificates(tenant_id, COALESCE(point_of_sale_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE status = 'ACTIVE';

-- Existing invoice FKs automatically follow the cufds -> cufd_history rename.
CREATE OR REPLACE FUNCTION enforce_customer_immutability() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM invoices WHERE customer_id = OLD.id) THEN
        IF TG_OP = 'DELETE' THEN
            RAISE EXCEPTION 'el cliente % tiene facturas asociadas; no se puede eliminar', OLD.id;
        END IF;
        IF OLD.company_id IS DISTINCT FROM NEW.company_id
            OR OLD.document_type IS DISTINCT FROM NEW.document_type
            OR OLD.document_number IS DISTINCT FROM NEW.document_number
            OR OLD.complement IS DISTINCT FROM NEW.complement
            OR OLD.name IS DISTINCT FROM NEW.name THEN
            RAISE EXCEPTION 'el cliente % tiene facturas asociadas; sus datos fiscales son inmutables', OLD.id;
        END IF;
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_customers_immutability ON customers;
CREATE TRIGGER trg_customers_immutability
BEFORE UPDATE OR DELETE ON customers
FOR EACH ROW EXECUTE FUNCTION enforce_customer_immutability();

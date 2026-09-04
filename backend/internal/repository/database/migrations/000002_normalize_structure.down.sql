-- Roll back the structural names while preserving the newest catalog snapshot.
CREATE TABLE catalogs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES tenants(id),
    tipo varchar(50) NOT NULL, codigo integer NOT NULL, descripcion text NOT NULL,
    synced_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO catalogs (company_id, tipo, codigo, descripcion, synced_at)
SELECT v.tenant_id, v.tipo, i.codigo::integer, i.descripcion, v.synced_at
FROM catalog_versions v JOIN catalog_items i ON i.version_id = v.id
WHERE v.version = (SELECT max(v2.version) FROM catalog_versions v2 WHERE v2.tenant_id = v.tenant_id AND v2.tipo = v.tipo)
  AND v.tipo NOT IN ('tipoPuntoVenta', 'actividades', 'leyendasFactura', 'actividadesDocumentoSector')
  AND i.codigo ~ '^[0-9]+$';

CREATE TABLE tipo_punto_ventas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES tenants(id),
    codigo_clasificador integer NOT NULL, descripcion varchar(200) NOT NULL,
    synced_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO tipo_punto_ventas (company_id, codigo_clasificador, descripcion, synced_at)
SELECT v.tenant_id, i.codigo::integer, i.descripcion, v.synced_at
FROM catalog_versions v JOIN catalog_items i ON i.version_id = v.id
WHERE v.tipo = 'tipoPuntoVenta'
  AND v.version = (SELECT max(v2.version) FROM catalog_versions v2 WHERE v2.tenant_id = v.tenant_id AND v2.tipo = v.tipo);

CREATE TABLE siat_actividades (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES tenants(id),
    codigo_caeb varchar(20) NOT NULL, descripcion text NOT NULL,
    tipo_actividad varchar(10) NOT NULL DEFAULT '', synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO siat_actividades (company_id, codigo_caeb, descripcion, tipo_actividad, synced_at)
SELECT v.tenant_id, i.codigo, i.descripcion, COALESCE(i.metadata->>'tipo_actividad', ''), v.synced_at
FROM catalog_versions v JOIN catalog_items i ON i.version_id = v.id
WHERE v.tipo = 'actividades'
  AND v.version = (SELECT max(v2.version) FROM catalog_versions v2 WHERE v2.tenant_id = v.tenant_id AND v2.tipo = v.tipo);

CREATE TABLE siat_leyendas_factura (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES tenants(id),
    codigo_actividad varchar(20) NOT NULL, descripcion_leyenda text NOT NULL,
    synced_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO siat_leyendas_factura (company_id, codigo_actividad, descripcion_leyenda, synced_at)
SELECT v.tenant_id, i.metadata->>'codigo_actividad', i.descripcion, v.synced_at
FROM catalog_versions v JOIN catalog_items i ON i.version_id = v.id
WHERE v.tipo = 'leyendasFactura'
  AND v.version = (SELECT max(v2.version) FROM catalog_versions v2 WHERE v2.tenant_id = v.tenant_id AND v2.tipo = v.tipo);

CREATE TABLE siat_actividades_doc_sector (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES tenants(id),
    codigo_actividad varchar(20) NOT NULL, codigo_documento_sector integer NOT NULL,
    tipo_documento_sector varchar(20) NOT NULL DEFAULT '', synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO siat_actividades_doc_sector (
    company_id, codigo_actividad, codigo_documento_sector, tipo_documento_sector, synced_at
)
SELECT v.tenant_id, i.metadata->>'codigo_actividad',
       (i.metadata->>'codigo_documento_sector')::integer,
       COALESCE(i.metadata->>'tipo_documento_sector', ''), v.synced_at
FROM catalog_versions v JOIN catalog_items i ON i.version_id = v.id
WHERE v.tipo = 'actividadesDocumentoSector'
  AND v.version = (SELECT max(v2.version) FROM catalog_versions v2 WHERE v2.tenant_id = v.tenant_id AND v2.tipo = v.tipo);

DROP TRIGGER IF EXISTS trg_point_of_sale_cuis_history ON points_of_sale;
DROP FUNCTION IF EXISTS record_point_of_sale_cuis();
DROP TABLE catalog_items;
DROP TABLE catalog_versions;
DROP TABLE cuis_history;

DROP INDEX IF EXISTS idx_branches_tenant_code;
DROP INDEX IF EXISTS idx_points_of_sale_tenant_codes;
DROP INDEX IF EXISTS idx_api_keys_tenant;
DROP INDEX IF EXISTS idx_points_of_sale_tenant;
DROP INDEX IF EXISTS idx_certificates_tenant;
DROP INDEX IF EXISTS idx_certificates_active;
DROP INDEX IF EXISTS idx_certificates_one_active_per_pos;
DROP INDEX IF EXISTS idx_cufd_history_active;
DROP INDEX IF EXISTS idx_contingency_events_pos;
DROP INDEX IF EXISTS idx_contingency_events_pending;
DROP INDEX IF EXISTS idx_sin_products_tenant_code;

ALTER TABLE tenants DROP CONSTRAINT IF EXISTS chk_tenants_nit_not_blank;
ALTER TABLE points_of_sale DROP CONSTRAINT IF EXISTS chk_points_of_sale_codes;
ALTER TABLE certificates
    DROP CONSTRAINT IF EXISTS chk_certificate_status,
    DROP CONSTRAINT IF EXISTS chk_certificate_dates;
ALTER TABLE contingency_events
    DROP CONSTRAINT IF EXISTS chk_contingency_reason,
    DROP CONSTRAINT IF EXISTS chk_contingency_dates;
ALTER TABLE cufd_history DROP CONSTRAINT IF EXISTS chk_cufd_history_dates;

ALTER TABLE certificates
    DROP COLUMN point_of_sale_id, DROP COLUMN encrypted_blob,
    DROP COLUMN encrypted_password, DROP COLUMN serial_number,
    DROP COLUMN is_active, DROP COLUMN uploaded_at;
ALTER TABLE contingency_events
    DROP COLUMN tenant_id, DROP COLUMN siat_reception_code,
    DROP COLUMN cufd_id, DROP COLUMN synced_at;
ALTER TABLE cufd_history DROP COLUMN tenant_id;
ALTER TABLE cufd_history RENAME COLUMN is_active TO active;
ALTER TABLE cufd_history RENAME TO cufds;
ALTER TABLE sin_products RENAME COLUMN is_active TO active;
ALTER TABLE sin_products RENAME COLUMN tenant_id TO company_id;
ALTER TABLE catalog_sync_states RENAME COLUMN tenant_id TO company_id;
ALTER TABLE certificates RENAME COLUMN tenant_id TO company_id;
ALTER TABLE points_of_sale RENAME COLUMN tenant_id TO company_id;
ALTER TABLE points_of_sale RENAME TO point_of_sales;
ALTER TABLE branches RENAME COLUMN is_active TO active;
ALTER TABLE branches RENAME COLUMN tenant_id TO company_id;
ALTER TABLE api_keys RENAME COLUMN tenant_id TO company_id;

ALTER TABLE tenants
    ADD COLUMN codigo_sistema varchar(100) NOT NULL DEFAULT '',
    ADD COLUMN ambiente varchar(20) NOT NULL DEFAULT 'PILOTO',
    ADD COLUMN modalidad integer NOT NULL DEFAULT 1;
UPDATE tenants t SET
    codigo_sistema = c.codigo_sistema,
    ambiente = c.ambiente,
    modalidad = c.codigo_modalidad
FROM tenant_configs c WHERE c.tenant_id = t.id;
DROP TABLE tenant_configs;
ALTER TABLE tenants RENAME TO companies;

CREATE UNIQUE INDEX IF NOT EXISTS idx_company_sucursal ON branches(company_id, codigo_sucursal);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_sucursal_pv ON point_of_sales(company_id, codigo_sucursal, codigo_punto_venta);
CREATE INDEX IF NOT EXISTS idx_api_keys_company ON api_keys(company_id);
CREATE INDEX IF NOT EXISTS idx_cert_company ON certificates(company_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_sin_product ON sin_products(company_id, codigo_actividad, codigo_producto_sin);
CREATE INDEX IF NOT EXISTS idx_cufd_pos_valid ON cufds(point_of_sale_id, valid_from);
CREATE INDEX IF NOT EXISTS idx_contingency_pos_start ON contingency_events(point_of_sale_id, start_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_tipo_pv ON tipo_punto_ventas(company_id, codigo_clasificador);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_caeb ON siat_actividades(company_id, codigo_caeb);
CREATE INDEX IF NOT EXISTS idx_company_leyenda_act ON siat_leyendas_factura(company_id, codigo_actividad);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_act_sector ON siat_actividades_doc_sector(company_id, codigo_actividad, codigo_documento_sector);

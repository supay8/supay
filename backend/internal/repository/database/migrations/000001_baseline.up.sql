-- Baseline adoptable: CREATE IF NOT EXISTS lets databases previously managed
-- by GORM enter golang-migrate without losing their current data.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS companies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nit varchar(20) NOT NULL UNIQUE,
    business_name varchar(150) NOT NULL,
    codigo_sistema varchar(100) NOT NULL,
    ambiente varchar(20) NOT NULL DEFAULT 'PILOTO',
    modalidad integer NOT NULL DEFAULT 1,
    municipio varchar(100) NOT NULL DEFAULT '',
    direccion text NOT NULL DEFAULT '',
    telefono varchar(50) NOT NULL DEFAULT '',
    codigo_actividad varchar(20),
    pie_pagina text NOT NULL DEFAULT '',
    usuario_siat varchar(50) NOT NULL DEFAULT 'SUPAY',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    key_hash varchar(255) NOT NULL UNIQUE,
    key_prefix varchar(20) NOT NULL,
    name varchar(100) NOT NULL DEFAULT 'default',
    scopes varchar(255) NOT NULL DEFAULT 'read,write',
    is_active boolean NOT NULL DEFAULT true,
    last_used_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS branches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    codigo_sucursal integer NOT NULL,
    name varchar(150) NOT NULL,
    address text,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS point_of_sales (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    branch_id uuid REFERENCES branches(id),
    codigo_sucursal integer NOT NULL DEFAULT 0,
    codigo_punto_venta integer NOT NULL,
    description varchar(150) NOT NULL,
    cuis varchar(100),
    cuis_created_at timestamptz,
    is_active boolean NOT NULL DEFAULT true,
    siat_code integer,
    status varchar(50) NOT NULL DEFAULT 'CREATING',
    tipo_punto_venta integer,
    siat_transaccion boolean NOT NULL DEFAULT false,
    siat_registered_at timestamptz,
    siat_response jsonb,
    siat_error text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tipo_punto_ventas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    codigo_clasificador integer NOT NULL,
    descripcion varchar(200) NOT NULL,
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS catalogs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    tipo varchar(50) NOT NULL,
    codigo integer NOT NULL,
    descripcion text NOT NULL,
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sin_products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    codigo_actividad bigint NOT NULL DEFAULT 0,
    codigo_producto_sin bigint NOT NULL,
    descripcion text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS siat_actividades (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    codigo_caeb varchar(20) NOT NULL,
    descripcion text NOT NULL,
    tipo_actividad varchar(10) NOT NULL DEFAULT '',
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS siat_leyendas_factura (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    codigo_actividad varchar(20) NOT NULL,
    descripcion_leyenda text NOT NULL,
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS siat_actividades_doc_sector (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    codigo_actividad varchar(20) NOT NULL,
    codigo_documento_sector integer NOT NULL,
    tipo_documento_sector varchar(20) NOT NULL DEFAULT '',
    synced_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS catalog_sync_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    point_of_sale_id uuid NOT NULL REFERENCES point_of_sales(id),
    operation varchar(80) NOT NULL,
    status varchar(20) NOT NULL,
    rows_saved integer NOT NULL DEFAULT 0,
    synced_at timestamptz,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    sku varchar(100) NOT NULL,
    name varchar(200) NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS product_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products(id),
    sin_product_id uuid REFERENCES sin_products(id),
    codigo_producto_sin bigint NOT NULL,
    codigo_actividad varchar(20) NOT NULL,
    codigo_documento_sector integer NOT NULL,
    unidad_medida integer NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    synced_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS cufds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    point_of_sale_id uuid NOT NULL REFERENCES point_of_sales(id),
    cufd text NOT NULL,
    direccion text NOT NULL,
    codigo_control varchar(100) NOT NULL,
    codigo_qr text,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS contingency_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    point_of_sale_id uuid NOT NULL REFERENCES point_of_sales(id),
    reason varchar(50) NOT NULL,
    description text,
    start_date timestamptz NOT NULL,
    end_date timestamptz,
    siat_event_code varchar(50),
    is_synced boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    document_type varchar(20) NOT NULL,
    document_number varchar(30) NOT NULL,
    complement varchar(10),
    name varchar(150) NOT NULL,
    email varchar(150),
    codigo_cliente varchar(50) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    customer_id uuid REFERENCES customers(id),
    point_of_sale_id uuid NOT NULL REFERENCES point_of_sales(id),
    idempotency_key varchar(100),
    cufd_id uuid NOT NULL REFERENCES cufds(id),
    contingency_event_id uuid REFERENCES contingency_events(id),
    invoice_number integer NOT NULL,
    cuf varchar(150) UNIQUE,
    emission_type varchar(30) NOT NULL DEFAULT 'EN_LINEA',
    codigo_metodo_pago integer NOT NULL DEFAULT 1,
    codigo_moneda integer NOT NULL DEFAULT 1,
    tipo_cambio decimal(18,5) NOT NULL DEFAULT 1,
    codigo_documento_sector integer NOT NULL DEFAULT 1,
    layout varchar(80),
    modalidad integer NOT NULL DEFAULT 1,
    codigo_tipo_factura integer NOT NULL DEFAULT 1,
    archivo text,
    hash_archivo varchar(100),
    nombre_estudiante varchar(150),
    periodo_facturado varchar(30),
    sector_data jsonb,
    ajusta_factura_id uuid REFERENCES invoices(id),
    issue_date timestamptz NOT NULL,
    subtotal decimal(18,2) NOT NULL,
    discount decimal(18,2) NOT NULL DEFAULT 0,
    total decimal(18,2) NOT NULL,
    xml text,
    xml_hash varchar(100),
    siat_reception_code varchar(100),
    siat_mensajes text,
    status varchar(30) NOT NULL DEFAULT 'PENDING',
    motivo_anulacion integer,
    fecha_anulacion timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoice_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices(id),
    product_id uuid REFERENCES products(id),
    code varchar(50) NOT NULL,
    description text NOT NULL,
    codigo_actividad varchar(20),
    codigo_producto_sin varchar(20),
    unit_code integer,
    quantity decimal(18,3) NOT NULL,
    unit_price decimal(18,2) NOT NULL,
    discount decimal(18,2) NOT NULL DEFAULT 0,
    subtotal decimal(18,2) NOT NULL,
    sector_data jsonb
);

CREATE TABLE IF NOT EXISTS invoice_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices(id),
    type varchar(50) NOT NULL,
    message text NOT NULL,
    payload jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sent_packages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    point_of_sale_id uuid NOT NULL REFERENCES point_of_sales(id),
    type varchar(20) NOT NULL,
    codigo_recepcion varchar(100) NOT NULL UNIQUE,
    hash_archivo varchar(100) NOT NULL,
    cantidad_facturas integer NOT NULL,
    codigo_documento_sector integer NOT NULL,
    codigo_tipo_factura integer NOT NULL,
    codigo_emision integer NOT NULL,
    codigo_evento bigint,
    contingency_event_id uuid REFERENCES contingency_events(id),
    status varchar(30) NOT NULL DEFAULT 'SENT',
    mensajes text,
    xml_hash varchar(100) NOT NULL,
    sent_at timestamptz NOT NULL,
    validated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS certificates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    name varchar(150) NOT NULL,
    type varchar(10) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    not_before timestamptz NOT NULL,
    not_after timestamptz NOT NULL,
    issuer text,
    subject text,
    thumbprint varchar(100),
    siat_user_code varchar(50),
    config_path text,
    renewed_from uuid,
    encrypted_token text NOT NULL DEFAULT '',
    encrypted_p12_password text NOT NULL DEFAULT '',
    p12_storage_ref text NOT NULL DEFAULT '',
    modalidad integer,
    ambiente varchar(20),
    nit varchar(20) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS data_migrations (
    name varchar(200) PRIMARY KEY,
    run_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_company_sucursal ON branches(company_id, codigo_sucursal);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_sucursal_pv ON point_of_sales(company_id, codigo_sucursal, codigo_punto_venta);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_tipo_pv ON tipo_punto_ventas(company_id, codigo_clasificador);
CREATE INDEX IF NOT EXISTS idx_catalog_company_tipo ON catalogs(company_id, tipo);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_sin_product ON sin_products(company_id, codigo_actividad, codigo_producto_sin);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_caeb ON siat_actividades(company_id, codigo_caeb);
CREATE INDEX IF NOT EXISTS idx_company_leyenda_act ON siat_leyendas_factura(company_id, codigo_actividad);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_act_sector ON siat_actividades_doc_sector(company_id, codigo_actividad, codigo_documento_sector);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_state ON catalog_sync_states(company_id, point_of_sale_id, operation);
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_product_sku ON products(company_id, sku);
CREATE INDEX IF NOT EXISTS idx_cufd_pos_valid ON cufds(point_of_sale_id, valid_from);
CREATE INDEX IF NOT EXISTS idx_contingency_pos_start ON contingency_events(point_of_sale_id, start_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pos_invoice_num ON invoices(point_of_sale_id, invoice_number);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoice_idem_key ON invoices(point_of_sale_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_company ON api_keys(company_id);
CREATE INDEX IF NOT EXISTS idx_cert_company ON certificates(company_id);

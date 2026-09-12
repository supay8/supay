CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    sku varchar(100) NOT NULL,
    name varchar(200) NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_products_identity CHECK (btrim(sku) <> '' AND btrim(name) <> '')
);
CREATE UNIQUE INDEX idx_products_tenant_sku ON products(tenant_id, sku);
CREATE UNIQUE INDEX uq_products_tenant_id ON products(tenant_id, id);

CREATE TABLE product_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    product_id uuid NOT NULL,
    sin_product_id uuid,
    codigo_producto_sin bigint NOT NULL,
    codigo_actividad varchar(20) NOT NULL,
    codigo_documento_sector integer NOT NULL,
    unidad_medida integer NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    synced_at timestamptz NOT NULL,
    CONSTRAINT fk_product_mappings_product_tenant
        FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_product_mappings_sin_product_tenant
        FOREIGN KEY (tenant_id, sin_product_id) REFERENCES sin_products(tenant_id, id) ON DELETE RESTRICT
);

ALTER TABLE invoice_items ADD COLUMN product_id uuid;
ALTER TABLE invoice_items DROP CONSTRAINT IF EXISTS chk_invoice_items_fiscal_snapshot;
ALTER TABLE invoice_items ADD CONSTRAINT fk_invoice_items_product_tenant
    FOREIGN KEY (tenant_id, product_id) REFERENCES products(tenant_id, id) ON DELETE RESTRICT;

DROP TRIGGER IF EXISTS trg_no_mutate_invoice_items ON invoice_items;
DROP TRIGGER IF EXISTS trg_no_update_invoice_items ON invoice_items;
DROP FUNCTION IF EXISTS prevent_invoice_item_mutation();
DROP TRIGGER IF EXISTS trg_invoice_receiver_snapshot_immutability ON invoices;
DROP FUNCTION IF EXISTS enforce_invoice_receiver_snapshot_immutability();
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS chk_invoices_receiver_snapshot;

DROP INDEX IF EXISTS idx_customers_tenant_document_history;
DROP INDEX IF EXISTS idx_customers_tenant_name_history;
CREATE UNIQUE INDEX idx_customers_tenant_fiscal_identity
    ON customers (tenant_id, document_type, document_number, COALESCE(complement, ''));

CREATE OR REPLACE FUNCTION enforce_customer_immutability() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM invoices WHERE customer_id = OLD.id) THEN
        IF TG_OP = 'DELETE' THEN
            RAISE EXCEPTION 'el cliente % tiene facturas asociadas; no se puede eliminar', OLD.id;
        END IF;
        IF OLD.tenant_id IS DISTINCT FROM NEW.tenant_id
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

ALTER TABLE invoices
    DROP COLUMN customer_document_type,
    DROP COLUMN customer_document_number,
    DROP COLUMN customer_complement,
    DROP COLUMN customer_name,
    DROP COLUMN customer_email,
    DROP COLUMN customer_code;

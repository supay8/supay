-- Phase 6: normalize remaining tenant ownership and add invoice history primitives.

ALTER TABLE customers RENAME COLUMN company_id TO tenant_id;
ALTER TABLE products RENAME COLUMN company_id TO tenant_id;
ALTER TABLE invoices RENAME COLUMN company_id TO tenant_id;
ALTER TABLE sent_packages RENAME COLUMN company_id TO tenant_id;

DROP INDEX IF EXISTS idx_company_product_sku;
DROP INDEX IF EXISTS idx_api_keys_company;

CREATE INDEX IF NOT EXISTS idx_customers_tenant ON customers(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_products_tenant_sku
    ON products(tenant_id, sku);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sent_packages_tenant ON sent_packages(tenant_id);

CREATE TABLE invoice_sequences (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    point_of_sale_id uuid NOT NULL REFERENCES points_of_sale(id) ON DELETE CASCADE,
    next_number integer NOT NULL CHECK (next_number > 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, point_of_sale_id)
);

INSERT INTO invoice_sequences (tenant_id, point_of_sale_id, next_number)
SELECT tenant_id, point_of_sale_id, MAX(invoice_number) + 1
FROM invoices
GROUP BY tenant_id, point_of_sale_id
ON CONFLICT (tenant_id, point_of_sale_id) DO NOTHING;

CREATE TABLE invoice_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    document_type varchar(20) NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    content text,
    storage_ref text,
    mime_type varchar(100),
    sha256 varchar(128),
    is_current boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_invoice_document_source CHECK (content IS NOT NULL OR storage_ref IS NOT NULL),
    CONSTRAINT uq_invoice_document_version UNIQUE (invoice_id, document_type, version)
);

CREATE UNIQUE INDEX idx_invoice_documents_current
    ON invoice_documents(invoice_id, document_type) WHERE is_current = true;
CREATE INDEX idx_invoice_documents_invoice ON invoice_documents(invoice_id, created_at DESC);

INSERT INTO invoice_documents (invoice_id, document_type, version, content, sha256, mime_type)
SELECT id, 'XML', 1, xml, xml_hash, 'application/xml'
FROM invoices
WHERE xml IS NOT NULL AND btrim(xml) <> '';

INSERT INTO invoice_documents (invoice_id, document_type, version, content, sha256, mime_type)
SELECT id, 'FILE', 1, archivo, hash_archivo, 'application/octet-stream'
FROM invoices
WHERE archivo IS NOT NULL AND btrim(archivo) <> '';

ALTER TABLE invoice_events
    ADD COLUMN IF NOT EXISTS event_key varchar(100),
    ADD COLUMN IF NOT EXISTS tenant_id uuid;

UPDATE invoice_events e
SET tenant_id = i.tenant_id
FROM invoices i
WHERE i.id = e.invoice_id AND e.tenant_id IS NULL;

ALTER TABLE invoice_events
    ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE invoice_events
    ADD CONSTRAINT fk_invoice_events_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

CREATE INDEX idx_invoice_events_tenant ON invoice_events(tenant_id, created_at);
CREATE UNIQUE INDEX idx_invoice_events_key
    ON invoice_events(invoice_id, event_key) WHERE event_key IS NOT NULL;

-- Keep the historical trigger aligned with the normalized ownership column.
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

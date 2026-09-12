-- Customers are an append-only analytical dimension. Invoices and their lines
-- are the authoritative, immutable fiscal snapshots.

ALTER TABLE invoices
    ADD COLUMN customer_document_type varchar(20),
    ADD COLUMN customer_document_number varchar(30),
    ADD COLUMN customer_complement varchar(10),
    ADD COLUMN customer_name varchar(150),
    ADD COLUMN customer_email varchar(150),
    ADD COLUMN customer_code varchar(50);

UPDATE invoices i
SET customer_document_type = c.document_type,
    customer_document_number = c.document_number,
    customer_complement = c.complement,
    customer_name = c.name,
    customer_email = c.email,
    customer_code = c.codigo_cliente
FROM customers c
WHERE c.id = i.customer_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM invoices
        WHERE customer_document_type IS NULL
           OR customer_document_number IS NULL
           OR customer_name IS NULL
           OR customer_code IS NULL
    ) THEN
        RAISE EXCEPTION 'cannot create receiver snapshots: invoices without a complete customer relation exist';
    END IF;
END $$;

ALTER TABLE invoices
    ALTER COLUMN customer_document_type SET NOT NULL,
    ALTER COLUMN customer_document_number SET NOT NULL,
    ALTER COLUMN customer_name SET NOT NULL,
	ALTER COLUMN customer_code SET NOT NULL;

ALTER TABLE invoices ADD CONSTRAINT chk_invoices_receiver_snapshot CHECK (
	btrim(customer_document_type) <> '' AND btrim(customer_document_number) <> ''
	AND btrim(customer_name) <> '' AND btrim(customer_code) <> ''
);

DROP INDEX IF EXISTS idx_company_doc;
DROP INDEX IF EXISTS idx_customers_tenant_fiscal_identity;
CREATE INDEX idx_customers_tenant_document_history
    ON customers (tenant_id, document_type, document_number, created_at DESC);
CREATE INDEX idx_customers_tenant_name_history
    ON customers (tenant_id, name, created_at DESC);

-- No SQL client may mutate or delete historical receiver rows.
CREATE OR REPLACE FUNCTION enforce_customer_immutability() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'customers is append-only; create a new fiscal history row'
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_customers_immutability ON customers;
CREATE TRIGGER trg_customers_immutability
BEFORE UPDATE OR DELETE ON customers
FOR EACH ROW EXECUTE FUNCTION enforce_customer_immutability();

-- Status, SIAT response and generated documents may evolve, but the receiver
-- copied at issuance may never be changed.
CREATE OR REPLACE FUNCTION enforce_invoice_receiver_snapshot_immutability() RETURNS trigger AS $$
BEGIN
    IF OLD.customer_id IS DISTINCT FROM NEW.customer_id
       OR OLD.customer_document_type IS DISTINCT FROM NEW.customer_document_type
       OR OLD.customer_document_number IS DISTINCT FROM NEW.customer_document_number
       OR OLD.customer_complement IS DISTINCT FROM NEW.customer_complement
       OR OLD.customer_name IS DISTINCT FROM NEW.customer_name
       OR OLD.customer_email IS DISTINCT FROM NEW.customer_email
       OR OLD.customer_code IS DISTINCT FROM NEW.customer_code THEN
        RAISE EXCEPTION 'invoice receiver snapshot is immutable'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_invoice_receiver_snapshot_immutability
BEFORE UPDATE ON invoices
FOR EACH ROW EXECUTE FUNCTION enforce_invoice_receiver_snapshot_immutability();

CREATE OR REPLACE FUNCTION prevent_invoice_item_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'invoice_items are immutable fiscal snapshots'
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_no_mutate_invoice_items
BEFORE UPDATE OR DELETE ON invoice_items
FOR EACH ROW EXECUTE FUNCTION prevent_invoice_item_mutation();

-- Internal products and mappings are deliberately removed. Invoice lines
-- already contain every value needed by SIAT and by future rendering/audits.
UPDATE invoice_items item
SET code = COALESCE(NULLIF(btrim(item.code), ''), product.sku),
    description = COALESCE(NULLIF(btrim(item.description), ''), product.name),
    codigo_actividad = COALESCE(NULLIF(btrim(item.codigo_actividad), ''), mapping.codigo_actividad),
    codigo_producto_sin = COALESCE(NULLIF(btrim(item.codigo_producto_sin), ''), mapping.codigo_producto_sin::text),
    unit_code = COALESCE(item.unit_code, mapping.unidad_medida)
FROM products product
LEFT JOIN product_mappings mapping
    ON mapping.product_id = product.id AND mapping.is_default = true AND mapping.active = true
WHERE item.product_id = product.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM invoice_items
        WHERE btrim(code) = '' OR btrim(description) = ''
           OR codigo_actividad IS NULL OR btrim(codigo_actividad) = ''
           OR codigo_producto_sin IS NULL OR btrim(codigo_producto_sin) = ''
           OR unit_code IS NULL OR unit_code <= 0
    ) THEN
        RAISE EXCEPTION 'cannot remove products: invoice_items without a complete fiscal snapshot exist';
    END IF;
END $$;

ALTER TABLE invoice_items ADD CONSTRAINT chk_invoice_items_fiscal_snapshot CHECK (
    btrim(code) <> '' AND btrim(description) <> ''
    AND btrim(codigo_actividad) <> '' AND btrim(codigo_producto_sin) <> ''
    AND unit_code > 0
);

ALTER TABLE invoice_items DROP COLUMN IF EXISTS product_id;
DROP TABLE IF EXISTS product_mappings;
DROP TABLE IF EXISTS products;

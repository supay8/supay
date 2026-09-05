-- Roll back Phase 6 while preserving invoice data and legacy document columns.

DROP INDEX IF EXISTS idx_invoice_events_key;
DROP INDEX IF EXISTS idx_invoice_events_tenant;
ALTER TABLE invoice_events DROP CONSTRAINT IF EXISTS fk_invoice_events_tenant;
ALTER TABLE invoice_events DROP COLUMN IF EXISTS event_key;
ALTER TABLE invoice_events DROP COLUMN IF EXISTS tenant_id;

DROP TABLE IF EXISTS invoice_documents;
DROP TABLE IF EXISTS invoice_sequences;

DROP INDEX IF EXISTS idx_sent_packages_tenant;
DROP INDEX IF EXISTS idx_invoices_tenant;
DROP INDEX IF EXISTS idx_products_tenant_sku;
DROP INDEX IF EXISTS idx_customers_tenant;

ALTER TABLE sent_packages RENAME COLUMN tenant_id TO company_id;
ALTER TABLE invoices RENAME COLUMN tenant_id TO company_id;
ALTER TABLE products RENAME COLUMN tenant_id TO company_id;
ALTER TABLE customers RENAME COLUMN tenant_id TO company_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_company_product_sku
    ON products(company_id, sku);
CREATE INDEX IF NOT EXISTS idx_api_keys_company ON api_keys(tenant_id);

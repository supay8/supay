ALTER TABLE invoice_sequences DROP CONSTRAINT IF EXISTS fk_invoice_sequences_pos_tenant;
DROP INDEX IF EXISTS idx_points_of_sale_tenant_id;
DROP INDEX IF EXISTS idx_customers_tenant_fiscal_identity;
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_doc
    ON customers(tenant_id, document_type, document_number);

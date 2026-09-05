-- Phase 6 integrity hardening: make fiscal identity unique per tenant.

DROP INDEX IF EXISTS idx_company_doc;

WITH ranked AS (
    SELECT id,
           first_value(id) OVER (
               PARTITION BY tenant_id, document_type, COALESCE(complement, '')
               ORDER BY created_at, id
           ) AS canonical_id,
           row_number() OVER (
               PARTITION BY tenant_id, document_type, COALESCE(complement, '')
               ORDER BY created_at, id
           ) AS row_number
    FROM customers
)
UPDATE invoices i
SET customer_id = r.canonical_id
FROM ranked r
WHERE i.customer_id = r.id
  AND r.row_number > 1;

WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY tenant_id, document_type, COALESCE(complement, '')
               ORDER BY created_at, id
           ) AS row_number
    FROM customers
)
DELETE FROM customers c
USING ranked r
WHERE c.id = r.id AND r.row_number > 1;

CREATE UNIQUE INDEX idx_customers_tenant_fiscal_identity
    ON customers (tenant_id, document_type, document_number, COALESCE(complement, ''));

ALTER TABLE invoice_sequences
    DROP CONSTRAINT IF EXISTS fk_invoice_sequences_pos_tenant;
CREATE UNIQUE INDEX IF NOT EXISTS idx_points_of_sale_tenant_id
    ON points_of_sale (tenant_id, id);
ALTER TABLE invoice_sequences
    ADD CONSTRAINT fk_invoice_sequences_pos_tenant
    FOREIGN KEY (tenant_id, point_of_sale_id)
    REFERENCES points_of_sale (tenant_id, id)
    ON DELETE CASCADE;

-- Fiscal identity is CUF. Each creation is a distinct invoice; re-sending an
-- existing invoice uses its persisted identity and never a request header.
DROP INDEX IF EXISTS idx_invoice_idem_key;
ALTER TABLE invoices DROP COLUMN idempotency_key;

-- Queue IDs and the existing active-emission constraint handle dispatch retries.
DROP INDEX uq_outbox_idempotency;
ALTER TABLE outbox DROP COLUMN idempotency_key;

-- Keep automatic delivery deduplication separate from explicit manual resends.
ALTER TABLE invoice_email_notifications ADD COLUMN is_automatic boolean NOT NULL DEFAULT true;
UPDATE invoice_email_notifications SET is_automatic = (idempotency_key = 'issuance');
ALTER TABLE invoice_email_notifications DROP CONSTRAINT uq_invoice_email_request,
    DROP CONSTRAINT chk_invoice_email_request, DROP COLUMN idempotency_key;
ALTER TABLE invoice_email_notifications ADD CONSTRAINT chk_invoice_email_recipient CHECK (
    recipient = lower(btrim(recipient)) AND btrim(recipient) <> ''
);
CREATE UNIQUE INDEX uq_invoice_email_automatic ON invoice_email_notifications(tenant_id, invoice_id, recipient)
    WHERE is_automatic;
COMMENT ON COLUMN invoice_email_notifications.is_automatic IS
    'At most one automatic delivery per invoice and recipient. Explicit manual deliveries use their notification UUID for retries.';

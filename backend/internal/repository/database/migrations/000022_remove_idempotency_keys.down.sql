-- Restores the previous column contract. Removed request keys cannot be recovered;
-- fiscal CUFs, invoices, outbox events and delivery history remain intact.
ALTER TABLE invoices ADD COLUMN idempotency_key varchar(100);
CREATE UNIQUE INDEX idx_invoice_idem_key ON invoices(point_of_sale_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
ALTER TABLE outbox ADD COLUMN idempotency_key text;
CREATE UNIQUE INDEX uq_outbox_idempotency ON outbox(tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
ALTER TABLE invoice_email_notifications ADD COLUMN idempotency_key text NOT NULL DEFAULT 'issuance';
UPDATE invoice_email_notifications SET idempotency_key = id::text WHERE NOT is_automatic;
DROP INDEX uq_invoice_email_automatic;
ALTER TABLE invoice_email_notifications DROP CONSTRAINT chk_invoice_email_recipient, DROP COLUMN is_automatic,
    ADD CONSTRAINT uq_invoice_email_request UNIQUE (tenant_id, invoice_id, recipient, idempotency_key),
    ADD CONSTRAINT chk_invoice_email_request CHECK (
        recipient = lower(btrim(recipient)) AND btrim(recipient) <> '' AND btrim(idempotency_key) <> ''
    );

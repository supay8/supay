-- Phase 7: constrain invoice status values at the database boundary.

ALTER TABLE invoices
    DROP CONSTRAINT IF EXISTS chk_invoices_status;

UPDATE invoices
SET status = CASE status
    WHEN 'DRAFT' THEN 'PENDING'
    WHEN 'SUBMITTING' THEN 'SENDING'
    WHEN 'VALIDATED' THEN 'ACCEPTED'
    ELSE status
END
WHERE status IN ('DRAFT', 'SUBMITTING', 'VALIDATED');

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM invoices
        WHERE status NOT IN (
            'PENDING', 'SENDING', 'SENT', 'ACCEPTED',
            'REJECTED', 'OBSERVED', 'OFFLINE', 'CANCELLED'
        )
    ) THEN
        RAISE EXCEPTION 'invoices contiene estados históricos no reconocidos';
    END IF;
END $$;

ALTER TABLE invoices
    ADD CONSTRAINT chk_invoices_status
    CHECK (status IN (
        'PENDING', 'SENDING', 'SENT', 'ACCEPTED',
        'REJECTED', 'OBSERVED', 'OFFLINE', 'CANCELLED'
    ));

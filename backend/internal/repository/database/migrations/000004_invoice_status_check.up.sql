-- Phase 7: constrain invoice status values at the database boundary.

ALTER TABLE invoices
    ADD CONSTRAINT chk_invoices_status
    CHECK (status IN (
        'PENDING', 'SENDING', 'SENT', 'ACCEPTED',
        'REJECTED', 'OBSERVED', 'OFFLINE', 'CANCELLED'
    ));

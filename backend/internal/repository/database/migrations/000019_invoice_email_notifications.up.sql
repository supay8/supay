CREATE TABLE invoice_email_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    invoice_id uuid NOT NULL,
    recipient varchar(320) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PUBLISHING', 'ENQUEUED', 'SENDING', 'SENT')),
    publish_attempts integer NOT NULL DEFAULT 0 CHECK (publish_attempts >= 0),
    delivery_attempts integer NOT NULL DEFAULT 0 CHECK (delivery_attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by varchar(100),
    task_name text,
    last_error text,
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoice_email_notifications_invoice_tenant_fk
        FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT invoice_email_notifications_invoice_unique UNIQUE (invoice_id)
);

CREATE INDEX idx_invoice_email_notifications_publish
    ON invoice_email_notifications (available_at, created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_invoice_email_notifications_stale
    ON invoice_email_notifications (locked_at)
    WHERE status IN ('PUBLISHING', 'SENDING');

-- Phase 9: transactional outbox for asynchronous invoice emission.

CREATE TABLE outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    aggregate_type varchar(50) NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type varchar(100) NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status varchar(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'PUBLISHED')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by varchar(100),
    published_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_type, aggregate_id)
);

CREATE INDEX idx_outbox_dispatch
    ON outbox (available_at, created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_outbox_stale_locks
    ON outbox (locked_at)
    WHERE status = 'PROCESSING';

CREATE INDEX idx_outbox_tenant_created
    ON outbox (tenant_id, created_at DESC);

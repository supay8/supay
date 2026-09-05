-- Phase 8: credential renewal metadata and idempotent certificate alerts.

ALTER TABLE points_of_sale
    ADD COLUMN cuis_expires_at timestamptz;

-- Older CUIS rows did not persist the SIAT expiration. Give them a bounded
-- fallback so the scheduler can rotate them instead of treating them as
-- perpetually valid. New renewals persist the exact SIAT fechaVigencia.
UPDATE points_of_sale
SET cuis_expires_at = COALESCE(cuis_created_at, created_at) + interval '365 days'
WHERE cuis IS NOT NULL AND cuis <> '' AND cuis_expires_at IS NULL;

UPDATE cuis_history h
SET valid_to = p.cuis_expires_at
FROM points_of_sale p
WHERE h.point_of_sale_id = p.id
  AND h.is_active = true
  AND h.valid_to IS NULL;

CREATE OR REPLACE FUNCTION record_point_of_sale_cuis() RETURNS trigger AS $$
BEGIN
    IF NEW.cuis IS NOT NULL AND NEW.cuis <> ''
       AND (NEW.cuis IS DISTINCT FROM OLD.cuis
            OR NEW.cuis_expires_at IS DISTINCT FROM OLD.cuis_expires_at) THEN
        UPDATE cuis_history
        SET is_active = false,
            valid_to = COALESCE(NEW.cuis_created_at, now())
        WHERE point_of_sale_id = NEW.id AND is_active = true;

        INSERT INTO cuis_history (tenant_id, point_of_sale_id, cuis, valid_from, valid_to)
        VALUES (
            NEW.tenant_id,
            NEW.id,
            NEW.cuis,
            COALESCE(NEW.cuis_created_at, now()),
            NEW.cuis_expires_at
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_point_of_sale_cuis_history ON points_of_sale;
CREATE TRIGGER trg_point_of_sale_cuis_history
AFTER UPDATE OF cuis, cuis_expires_at ON points_of_sale
FOR EACH ROW EXECUTE FUNCTION record_point_of_sale_cuis();

CREATE TABLE certificate_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    certificate_id uuid NOT NULL REFERENCES certificates(id) ON DELETE CASCADE,
    threshold_days integer NOT NULL CHECK (threshold_days IN (30, 15, 7)),
    channel varchar(20) NOT NULL CHECK (channel IN ('WEBHOOK')),
    status varchar(20) NOT NULL CHECK (status IN ('PENDING', 'SENT', 'FAILED')),
    attempts integer NOT NULL DEFAULT 1 CHECK (attempts > 0),
    last_error text,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(certificate_id, threshold_days, channel)
);

CREATE INDEX idx_certificate_notifications_tenant
    ON certificate_notifications(tenant_id, created_at DESC);
CREATE INDEX idx_certificate_notifications_retry
    ON certificate_notifications(status, updated_at)
    WHERE status IN ('PENDING', 'FAILED');
CREATE INDEX idx_certificates_expiration
    ON certificates(not_after)
    WHERE status = 'ACTIVE';

-- La renovación preventiva crea el nuevo CUFD antes de que venza el anterior;
-- solo el más reciente debe permanecer marcado como activo.
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY tenant_id, point_of_sale_id
               ORDER BY created_at DESC, id DESC
           ) AS row_number
    FROM cufd_history
    WHERE is_active = true
)
UPDATE cufd_history c
SET is_active = false
FROM ranked r
WHERE c.id = r.id AND r.row_number > 1;

CREATE UNIQUE INDEX idx_cufd_history_one_active
    ON cufd_history(tenant_id, point_of_sale_id)
    WHERE is_active = true;

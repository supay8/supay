DROP INDEX IF EXISTS idx_cufd_history_one_active;
DROP INDEX IF EXISTS idx_certificates_expiration;
DROP INDEX IF EXISTS idx_certificate_notifications_retry;
DROP INDEX IF EXISTS idx_certificate_notifications_tenant;
DROP TABLE IF EXISTS certificate_notifications;

DROP TRIGGER IF EXISTS trg_point_of_sale_cuis_history ON points_of_sale;
CREATE OR REPLACE FUNCTION record_point_of_sale_cuis() RETURNS trigger AS $$
BEGIN
    IF NEW.cuis IS NOT NULL AND NEW.cuis <> '' AND NEW.cuis IS DISTINCT FROM OLD.cuis THEN
        UPDATE cuis_history
        SET is_active = false, valid_to = COALESCE(NEW.cuis_created_at, now())
        WHERE point_of_sale_id = NEW.id AND is_active = true;

        INSERT INTO cuis_history (tenant_id, point_of_sale_id, cuis, valid_from)
        VALUES (NEW.tenant_id, NEW.id, NEW.cuis, COALESCE(NEW.cuis_created_at, now()));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_point_of_sale_cuis_history
AFTER UPDATE OF cuis ON points_of_sale
FOR EACH ROW EXECUTE FUNCTION record_point_of_sale_cuis();

UPDATE cuis_history SET valid_to = NULL WHERE is_active = true;
ALTER TABLE points_of_sale DROP COLUMN IF EXISTS cuis_expires_at;

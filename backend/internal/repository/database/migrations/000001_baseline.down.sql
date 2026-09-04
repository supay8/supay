-- The baseline adopts tables that may predate golang-migrate. Dropping them on
-- rollback would destroy user data, so this migration is intentionally a no-op.
SELECT 1;

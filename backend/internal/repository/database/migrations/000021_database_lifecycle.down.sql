-- Never discard queue/package history to make an older schema fit.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM outbox GROUP BY event_type, aggregate_id HAVING count(*) > 1)
       OR EXISTS (SELECT 1 FROM sent_package_invoices GROUP BY invoice_id HAVING count(*) > 1)
       OR EXISTS (SELECT 1 FROM invoice_email_notifications GROUP BY invoice_id HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'rollback would collapse history; retain 000021 or archive additional attempts explicitly';
    END IF;
    IF EXISTS (SELECT 1 FROM api_keys WHERE length(array_to_string(scopes, ',')) > 255
        OR EXISTS (SELECT 1 FROM unnest(scopes) s WHERE s LIKE '%,%')) THEN
        RAISE EXCEPTION 'rollback would truncate or change API scopes';
    END IF;
    IF EXISTS (SELECT 1 FROM tenants WHERE archived_at IS NOT NULL)
       OR EXISTS (SELECT 1 FROM outbox WHERE idempotency_key IS NOT NULL)
       OR EXISTS (SELECT 1 FROM invoice_email_notifications WHERE idempotency_key <> 'issuance') THEN
        RAISE EXCEPTION 'rollback would discard archive/idempotency metadata';
    END IF;
END $$;
DROP INDEX idx_invoices_created_brin;
DROP INDEX idx_invoice_events_created_brin;
DROP INDEX idx_outbox_created_brin;
DROP FUNCTION archive_tenant(uuid, text);
ALTER TABLE tenants DROP CONSTRAINT chk_tenant_archive, DROP COLUMN archived_at, DROP COLUMN archive_reason;
DO $$ DECLARE tab text;
BEGIN
    FOREACH tab IN ARRAY ARRAY['tenants', 'tenant_configs', 'api_keys', 'sin_products',
        'catalog_sync_states', 'invoices', 'invoice_sequences', 'certificates',
        'certificate_notifications', 'outbox', 'invoice_email_notifications', 'users'] LOOP
        EXECUTE format('DROP TRIGGER trg_set_updated_at ON public.%I', tab);
    END LOOP;
    FOREACH tab IN ARRAY ARRAY['users', 'sessions', 'accounts', 'verifications'] LOOP
        EXECUTE format('DROP TRIGGER trg_set_updated_at ON auth.%I', tab);
    END LOOP;
END $$;
DROP FUNCTION set_updated_at();
ALTER TABLE api_keys DROP CONSTRAINT chk_api_key_scopes;
ALTER TABLE api_keys ALTER COLUMN scopes DROP DEFAULT;
ALTER TABLE api_keys ALTER COLUMN scopes TYPE varchar(255) USING array_to_string(scopes, ',');
ALTER TABLE api_keys ALTER COLUMN scopes SET DEFAULT 'read,write';
DROP INDEX idx_certificates_one_active_per_tenant;
DROP TRIGGER trg_preserve_invoice_document ON invoice_documents;
DROP FUNCTION preserve_invoice_document();
DROP TRIGGER trg_replace_current_invoice_document ON invoice_documents;
DROP FUNCTION replace_current_invoice_document();
DROP INDEX idx_customers_latest_identity;
CREATE OR REPLACE FUNCTION enforce_customer_immutability() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'customers is append-only; create a new fiscal history row'
        USING ERRCODE = 'integrity_constraint_violation';
END $$;
ALTER TABLE invoice_email_notifications DROP CONSTRAINT chk_invoice_email_request,
    DROP CONSTRAINT uq_invoice_email_request, DROP COLUMN idempotency_key,
    ADD CONSTRAINT invoice_email_notifications_invoice_unique UNIQUE(invoice_id);
DROP TRIGGER trg_terminal_package_status ON sent_packages;
DROP FUNCTION preserve_terminal_package_status();
DROP TRIGGER trg_package_membership_immutable ON sent_package_invoices;
DROP FUNCTION prevent_package_membership_mutation();
DROP TRIGGER trg_reserve_package_invoice ON sent_package_invoices;
DROP FUNCTION reserve_package_invoice();
ALTER TABLE sent_package_invoices DROP CONSTRAINT sent_package_invoices_pkey, ADD PRIMARY KEY(invoice_id);
DROP INDEX uq_outbox_active_invoice_emission;
DROP INDEX uq_outbox_idempotency;
ALTER TABLE outbox DROP COLUMN idempotency_key, ADD CONSTRAINT outbox_event_type_aggregate_id_key UNIQUE(event_type, aggregate_id);
DROP VIEW tenant_memberships;
DROP TRIGGER trg_normalize_local_email ON public.users;
DROP TRIGGER trg_normalize_cloud_email ON auth.users;
DROP FUNCTION normalize_auth_email();
DROP INDEX auth.auth_users_email_lower_uq;
ALTER INDEX invoice_files_tenant_invoice_idx RENAME TO invoice_files_company_invoice_idx;
ALTER TABLE invoice_files RENAME COLUMN tenant_id TO company_id;

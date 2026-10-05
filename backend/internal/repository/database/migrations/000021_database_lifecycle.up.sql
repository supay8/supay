-- Preserve fiscal snapshots, isolate auth authorities and retain queue history.
ALTER TABLE invoice_files RENAME COLUMN company_id TO tenant_id;
ALTER INDEX invoice_files_company_invoice_idx RENAME TO invoice_files_tenant_invoice_idx;

-- Identical email rules, independent identities. Never merge accounts by email.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM auth.users GROUP BY lower(btrim(email)) HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'auth.users has duplicate normalized emails; resolve identities before migrating';
    END IF;
END $$;
CREATE UNIQUE INDEX auth_users_email_lower_uq ON auth.users(lower(btrim(email)));
CREATE OR REPLACE FUNCTION normalize_auth_email() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.email := lower(btrim(NEW.email));
    RETURN NEW;
END $$;
CREATE TRIGGER trg_normalize_local_email BEFORE INSERT OR UPDATE OF email ON public.users
    FOR EACH ROW EXECUTE FUNCTION normalize_auth_email();
CREATE TRIGGER trg_normalize_cloud_email BEFORE INSERT OR UPDATE OF email ON auth.users
    FOR EACH ROW EXECUTE FUNCTION normalize_auth_email();
UPDATE auth.users SET email = lower(btrim(email));
-- Existing local emails were already normalized by the repository. Abort rather
-- than merge identities if manual SQL introduced duplicates with whitespace.
UPDATE public.users SET email = lower(btrim(email));

CREATE VIEW tenant_memberships AS
    SELECT 'jwt'::text AS provider, u.id AS user_id, t.id AS tenant_id, m.role::text AS role
    FROM public.user_tenants m
    JOIN public.users u ON u.id = m.user_id AND u.is_active
    JOIN public.tenants t ON t.id = m.tenant_id AND t.is_active AND t.auth_organization_id IS NULL
    UNION ALL
    SELECT 'better_auth'::text, m.user_id, t.id, m.role
    FROM auth.members m
    JOIN public.tenants t ON t.auth_organization_id = m.organization_id AND t.is_active;
COMMENT ON VIEW tenant_memberships IS
    'Derived access projection. JWT: public.user_tenants; cloud: auth.members. Provider is part of identity. Never write/synchronize this view.';

-- Outstanding emissions are idempotent; published emissions are immutable history
-- from the queue perspective. Other event kinds use explicit idempotency keys.
ALTER TABLE outbox DROP CONSTRAINT outbox_event_type_aggregate_id_key;
ALTER TABLE outbox ADD COLUMN idempotency_key text;
CREATE UNIQUE INDEX uq_outbox_idempotency ON outbox(tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX uq_outbox_active_invoice_emission
    ON outbox(tenant_id, aggregate_type, aggregate_id, event_type)
    WHERE aggregate_type = 'invoice' AND event_type = 'invoice.emit.requested'
      AND status IN ('PENDING', 'PROCESSING');

-- A membership is a historical attempt, not a lifetime reservation.
ALTER TABLE sent_package_invoices DROP CONSTRAINT sent_package_invoices_pkey;
ALTER TABLE sent_package_invoices ADD PRIMARY KEY (invoice_id, sent_package_id);
CREATE OR REPLACE FUNCTION reserve_package_invoice() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM 1 FROM invoices WHERE id = NEW.invoice_id AND tenant_id = NEW.tenant_id FOR UPDATE;
    IF EXISTS (
        SELECT 1 FROM sent_package_invoices m JOIN sent_packages p ON p.id = m.sent_package_id
        WHERE m.invoice_id = NEW.invoice_id AND p.status <> 'REJECTED'
    ) THEN
        RAISE EXCEPTION 'invoice already belongs to an unresolved or accepted package' USING ERRCODE = '23505';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trg_reserve_package_invoice BEFORE INSERT ON sent_package_invoices
    FOR EACH ROW EXECUTE FUNCTION reserve_package_invoice();
-- Terminal results cannot be revived after another attempt has reserved invoices.
CREATE OR REPLACE FUNCTION preserve_terminal_package_status() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IN ('ACCEPTED', 'REJECTED') AND OLD.status IS DISTINCT FROM NEW.status THEN
        RAISE EXCEPTION 'terminal package status is immutable' USING ERRCODE = '23000';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trg_terminal_package_status BEFORE UPDATE OF status ON sent_packages
    FOR EACH ROW EXECUTE FUNCTION preserve_terminal_package_status();
CREATE OR REPLACE FUNCTION prevent_package_membership_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'package membership is immutable fiscal history' USING ERRCODE = '23000';
END $$;
CREATE TRIGGER trg_package_membership_immutable BEFORE UPDATE ON sent_package_invoices
    FOR EACH ROW EXECUTE FUNCTION prevent_package_membership_mutation();

ALTER TABLE invoice_email_notifications DROP CONSTRAINT invoice_email_notifications_invoice_unique;
ALTER TABLE invoice_email_notifications ADD COLUMN idempotency_key text NOT NULL DEFAULT 'issuance';
UPDATE invoice_email_notifications SET recipient = lower(btrim(recipient));
ALTER TABLE invoice_email_notifications
    ADD CONSTRAINT uq_invoice_email_request UNIQUE (tenant_id, invoice_id, recipient, idempotency_key),
    ADD CONSTRAINT chk_invoice_email_request CHECK (
        recipient = lower(btrim(recipient)) AND btrim(recipient) <> '' AND btrim(idempotency_key) <> ''
    );

-- Fiscal fields remain immutable; operational contact and deactivation may evolve.
CREATE OR REPLACE FUNCTION enforce_customer_immutability() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'customer fiscal history cannot be deleted' USING ERRCODE = '23000';
    END IF;
    IF (to_jsonb(NEW) - 'email' - 'is_active') IS DISTINCT FROM
       (to_jsonb(OLD) - 'email' - 'is_active') THEN
        RAISE EXCEPTION 'customer fiscal identity is immutable; create a new history row' USING ERRCODE = '23000';
    END IF;
    RETURN NEW;
END $$;
CREATE INDEX idx_customers_latest_identity
    ON customers(tenant_id, document_type, document_number, COALESCE(complement, ''), created_at DESC, id DESC)
    WHERE is_active;

-- The existing partial unique index already prevents two current documents.
-- Serialize replacement on the parent invoice, including the first document.
CREATE OR REPLACE FUNCTION replace_current_invoice_document() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM 1 FROM invoices WHERE id = NEW.invoice_id FOR UPDATE;
    IF NEW.is_current THEN
        IF EXISTS (SELECT 1 FROM invoice_documents
                   WHERE invoice_id = NEW.invoice_id AND document_type = NEW.document_type
                     AND version >= NEW.version) THEN
            RAISE EXCEPTION 'current invoice document must advance the version' USING ERRCODE = '23000';
        END IF;
        UPDATE invoice_documents SET is_current = false
            WHERE invoice_id = NEW.invoice_id AND document_type = NEW.document_type AND is_current;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trg_replace_current_invoice_document BEFORE INSERT ON invoice_documents
    FOR EACH ROW EXECUTE FUNCTION replace_current_invoice_document();
CREATE OR REPLACE FUNCTION preserve_invoice_document() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (to_jsonb(NEW) - 'is_current') IS DISTINCT FROM (to_jsonb(OLD) - 'is_current') THEN
        RAISE EXCEPTION 'invoice document metadata is immutable; insert a new version' USING ERRCODE = '23000';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trg_preserve_invoice_document BEFORE UPDATE ON invoice_documents
    FOR EACH ROW EXECUTE FUNCTION preserve_invoice_document();

-- 000020 removed certificate POS scope and its index. Scope is now tenant-wide.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM certificates WHERE status = 'ACTIVE' GROUP BY tenant_id HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'multiple ACTIVE certificates per tenant; revoke obsolete certificates before migrating';
    END IF;
END $$;
CREATE UNIQUE INDEX idx_certificates_one_active_per_tenant ON certificates(tenant_id) WHERE status = 'ACTIVE';

ALTER TABLE api_keys ALTER COLUMN scopes DROP DEFAULT;
ALTER TABLE api_keys ALTER COLUMN scopes TYPE text[] USING
    CASE WHEN btrim(scopes) = '' THEN ARRAY[]::text[] ELSE regexp_split_to_array(btrim(scopes), '\s*,\s*') END;
ALTER TABLE api_keys ALTER COLUMN scopes SET DEFAULT ARRAY['read', 'write']::text[];
ALTER TABLE api_keys ADD CONSTRAINT chk_api_key_scopes CHECK (
    array_position(scopes, NULL) IS NULL AND array_position(scopes, '') IS NULL
);

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END $$;
DO $$ DECLARE tab text;
BEGIN
    FOREACH tab IN ARRAY ARRAY['tenants', 'tenant_configs', 'api_keys', 'sin_products',
        'catalog_sync_states', 'invoices', 'invoice_sequences', 'certificates',
        'certificate_notifications', 'outbox', 'invoice_email_notifications', 'users'] LOOP
        EXECUTE format('CREATE TRIGGER trg_set_updated_at BEFORE UPDATE ON public.%I FOR EACH ROW EXECUTE FUNCTION set_updated_at()', tab);
    END LOOP;
    FOREACH tab IN ARRAY ARRAY['users', 'sessions', 'accounts', 'verifications'] LOOP
        EXECUTE format('CREATE TRIGGER trg_set_updated_at BEFORE UPDATE ON auth.%I FOR EACH ROW EXECUTE FUNCTION public.set_updated_at()', tab);
    END LOOP;
END $$;

ALTER TABLE tenants ADD COLUMN archived_at timestamptz, ADD COLUMN archive_reason text;
ALTER TABLE tenants ADD CONSTRAINT chk_tenant_archive CHECK (
    (archived_at IS NULL AND archive_reason IS NULL) OR
    (archived_at IS NOT NULL AND NOT is_active AND archive_reason IS NOT NULL AND btrim(archive_reason) <> '')
);
CREATE OR REPLACE FUNCTION archive_tenant(target uuid, reason text) RETURNS void
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    IF reason IS NULL OR btrim(reason) = '' THEN
        RAISE EXCEPTION 'archive reason is required';
    END IF;
    UPDATE public.tenants SET is_active = false, archived_at = COALESCE(archived_at, now()),
        archive_reason = COALESCE(archive_reason, btrim(reason)) WHERE id = target;
    IF NOT FOUND THEN RAISE EXCEPTION 'tenant does not exist'; END IF;
    UPDATE public.api_keys SET is_active = false WHERE tenant_id = target AND is_active;
END $$;
REVOKE ALL ON FUNCTION archive_tenant(uuid, text) FROM PUBLIC;
COMMENT ON FUNCTION archive_tenant(uuid, text) IS
    'Administrative soft archive. Invoker privileges only; revoke access while retaining fiscal records. Does not cancel pending SIAT reconciliation.';

-- Time-ordered inserts make BRIN useful without changing UUID/FK contracts.
CREATE INDEX idx_invoices_created_brin ON invoices USING brin(created_at);
CREATE INDEX idx_invoice_events_created_brin ON invoice_events USING brin(created_at);
CREATE INDEX idx_outbox_created_brin ON outbox USING brin(created_at);

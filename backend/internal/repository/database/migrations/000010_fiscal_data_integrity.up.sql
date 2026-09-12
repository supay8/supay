-- Fiscal data integrity hardening.
--
-- Tenant-owned relationships use composite foreign keys so PostgreSQL, not
-- only the application, rejects references to rows owned by another tenant.
-- Fiscal history is retained: operational entities are disabled/revoked and
-- append-only fiscal records reject hard deletes.

ALTER TABLE tenants
    ADD COLUMN is_active boolean NOT NULL DEFAULT true;
ALTER TABLE customers
    ADD COLUMN is_active boolean NOT NULL DEFAULT true;

-- Multi-parent children need tenant_id in order to prove that every parent is
-- part of the same tenant. Existing values are derived from their owner.
ALTER TABLE product_mappings ADD COLUMN tenant_id uuid;
UPDATE product_mappings m
SET tenant_id = p.tenant_id
FROM products p
WHERE p.id = m.product_id;
ALTER TABLE product_mappings ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE invoice_items ADD COLUMN tenant_id uuid;
UPDATE invoice_items item
SET tenant_id = invoice.tenant_id
FROM invoices invoice
WHERE invoice.id = item.invoice_id;
ALTER TABLE invoice_items ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE sent_package_invoices ADD COLUMN tenant_id uuid;
UPDATE sent_package_invoices member
SET tenant_id = package.tenant_id
FROM sent_packages package
WHERE package.id = member.sent_package_id;
ALTER TABLE sent_package_invoices ALTER COLUMN tenant_id SET NOT NULL;

-- A fiscal invoice must always retain the receiver snapshot it was issued to.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM invoices WHERE customer_id IS NULL) THEN
        RAISE EXCEPTION 'cannot enforce invoice integrity: invoices.customer_id contains NULL values';
    END IF;
END $$;
ALTER TABLE invoices ALTER COLUMN customer_id SET NOT NULL;

-- Persist the precision already required by the SIAT payload. These are
-- widening conversions and therefore do not discard existing values.
ALTER TABLE invoice_items
    ALTER COLUMN quantity TYPE numeric(18,5),
    ALTER COLUMN unit_price TYPE numeric(18,5);

-- Composite candidate keys used by tenant-safe foreign keys.
CREATE UNIQUE INDEX uq_branches_tenant_id ON branches(tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_points_of_sale_tenant_id ON points_of_sale(tenant_id, id);
CREATE UNIQUE INDEX uq_customers_tenant_id ON customers(tenant_id, id);
CREATE UNIQUE INDEX uq_products_tenant_id ON products(tenant_id, id);
CREATE UNIQUE INDEX uq_sin_products_tenant_id ON sin_products(tenant_id, id);
CREATE UNIQUE INDEX uq_cufd_history_tenant_id ON cufd_history(tenant_id, id);
CREATE UNIQUE INDEX uq_cuis_history_tenant_id ON cuis_history(tenant_id, id);
CREATE UNIQUE INDEX uq_contingency_events_tenant_id ON contingency_events(tenant_id, id);
CREATE UNIQUE INDEX uq_invoices_tenant_id ON invoices(tenant_id, id);
CREATE UNIQUE INDEX uq_sent_packages_tenant_id ON sent_packages(tenant_id, id);
CREATE UNIQUE INDEX uq_certificates_tenant_id ON certificates(tenant_id, id);

-- Remove legacy single-column FKs from the tables being hardened. Keeping a
-- CASCADE FK beside a RESTRICT composite FK would still allow the cascade.
DO $$
DECLARE
    fk record;
BEGIN
    FOR fk IN
        SELECT conrelid::regclass::text AS table_name, conname
        FROM pg_constraint
        WHERE contype = 'f'
          AND conrelid IN (
              'branches'::regclass,
              'points_of_sale'::regclass,
              'products'::regclass,
              'product_mappings'::regclass,
              'sin_products'::regclass,
              'cufd_history'::regclass,
              'cuis_history'::regclass,
              'contingency_events'::regclass,
              'customers'::regclass,
              'invoices'::regclass,
              'invoice_items'::regclass,
              'invoice_events'::regclass,
              'sent_packages'::regclass,
              'sent_package_invoices'::regclass,
              'certificates'::regclass,
              'certificate_notifications'::regclass,
              'outbox'::regclass,
              'catalog_sync_states'::regclass,
              'invoice_sequences'::regclass
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.table_name, fk.conname);
    END LOOP;
END $$;

-- Soft-deletable operational hierarchy: no parent is physically removed.
ALTER TABLE branches
    ADD CONSTRAINT fk_branches_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT;
ALTER TABLE points_of_sale
    ADD CONSTRAINT fk_points_of_sale_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_points_of_sale_branch_tenant FOREIGN KEY (tenant_id, branch_id)
        REFERENCES branches(tenant_id, id) ON DELETE RESTRICT;

ALTER TABLE products
    ADD CONSTRAINT fk_products_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT;
ALTER TABLE sin_products
    ADD CONSTRAINT fk_sin_products_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT;
ALTER TABLE product_mappings
    ADD CONSTRAINT fk_product_mappings_product_tenant FOREIGN KEY (tenant_id, product_id)
        REFERENCES products(tenant_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_product_mappings_sin_product_tenant FOREIGN KEY (tenant_id, sin_product_id)
        REFERENCES sin_products(tenant_id, id) ON DELETE RESTRICT;

-- Fiscal credentials and contingency history are retained.
ALTER TABLE cufd_history
    ADD CONSTRAINT fk_cufd_history_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_cufd_history_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE cuis_history
    ADD CONSTRAINT fk_cuis_history_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_cuis_history_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE contingency_events
    ADD CONSTRAINT fk_contingency_events_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_contingency_events_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_contingency_events_cufd_tenant FOREIGN KEY (tenant_id, cufd_id)
        REFERENCES cufd_history(tenant_id, id) ON DELETE RESTRICT;

ALTER TABLE customers
    ADD CONSTRAINT fk_customers_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT;
ALTER TABLE invoices
    ADD CONSTRAINT fk_invoices_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoices_customer_tenant FOREIGN KEY (tenant_id, customer_id)
        REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoices_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoices_cufd_tenant FOREIGN KEY (tenant_id, cufd_id)
        REFERENCES cufd_history(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoices_contingency_tenant FOREIGN KEY (tenant_id, contingency_event_id)
        REFERENCES contingency_events(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoices_adjustment_tenant FOREIGN KEY (tenant_id, ajusta_factura_id)
        REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE invoice_items
    ADD CONSTRAINT fk_invoice_items_invoice_tenant FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoice_items_product_tenant FOREIGN KEY (tenant_id, product_id)
        REFERENCES products(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE invoice_events
    ADD CONSTRAINT fk_invoice_events_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_invoice_events_invoice_tenant FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT;

ALTER TABLE sent_packages
    ADD CONSTRAINT fk_sent_packages_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_sent_packages_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_sent_packages_cufd_tenant FOREIGN KEY (tenant_id, cufd_id)
        REFERENCES cufd_history(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_sent_packages_contingency_tenant FOREIGN KEY (tenant_id, contingency_event_id)
        REFERENCES contingency_events(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE sent_package_invoices
    ADD CONSTRAINT fk_sent_package_invoices_invoice_tenant FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_sent_package_invoices_package_tenant FOREIGN KEY (tenant_id, sent_package_id)
        REFERENCES sent_packages(tenant_id, id) ON DELETE RESTRICT;

ALTER TABLE certificates
    ADD CONSTRAINT fk_certificates_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_certificates_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_certificates_renewed_from_tenant FOREIGN KEY (tenant_id, renewed_from)
        REFERENCES certificates(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE certificate_notifications
    ADD CONSTRAINT fk_certificate_notifications_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_certificate_notifications_certificate_tenant FOREIGN KEY (tenant_id, certificate_id)
        REFERENCES certificates(tenant_id, id) ON DELETE RESTRICT;

ALTER TABLE outbox
    ADD CONSTRAINT fk_outbox_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE RESTRICT;

-- Purely dependent/configurative rows retain CASCADE semantics.
ALTER TABLE catalog_sync_states
    ADD CONSTRAINT fk_catalog_sync_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_catalog_sync_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE CASCADE;
ALTER TABLE invoice_sequences
    ADD CONSTRAINT fk_invoice_sequences_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_invoice_sequences_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE CASCADE;

-- Domain checks at the PostgreSQL boundary.
ALTER TABLE branches
    ADD CONSTRAINT chk_branches_code CHECK (codigo_sucursal >= 0),
    ADD CONSTRAINT chk_branches_name CHECK (btrim(name) <> '');
ALTER TABLE products
    ADD CONSTRAINT chk_products_identity CHECK (btrim(sku) <> '' AND btrim(name) <> '');
ALTER TABLE customers
    ADD CONSTRAINT chk_customers_document_type CHECK (document_type IN ('CI', 'CEX', 'PAS', 'NIT', 'OD')),
    ADD CONSTRAINT chk_customers_identity CHECK (btrim(document_number) <> '' AND btrim(name) <> '');
ALTER TABLE cufd_history
    ADD CONSTRAINT chk_cufd_history_value CHECK (btrim(cufd) <> '');
ALTER TABLE cuis_history
    ADD CONSTRAINT chk_cuis_history_value CHECK (btrim(cuis) <> '');
ALTER TABLE invoices
    ADD CONSTRAINT chk_invoices_amounts CHECK (
        invoice_number > 0 AND tipo_cambio > 0 AND subtotal >= 0 AND discount >= 0 AND total >= 0
    ),
    ADD CONSTRAINT chk_invoices_codes CHECK (
        codigo_metodo_pago > 0 AND codigo_moneda > 0 AND codigo_documento_sector > 0
        AND modalidad IN (1, 2) AND codigo_tipo_factura > 0
    ),
    ADD CONSTRAINT chk_invoices_cancellation CHECK (
        (status = 'CANCELLED' AND motivo_anulacion IS NOT NULL AND fecha_anulacion IS NOT NULL)
        OR status <> 'CANCELLED'
    );
ALTER TABLE invoice_items
    ADD CONSTRAINT chk_invoice_items_amounts CHECK (
        quantity > 0 AND unit_price >= 0 AND discount >= 0 AND subtotal >= 0
    );
ALTER TABLE sent_packages
    ADD CONSTRAINT chk_sent_packages_type CHECK (type IN ('PAQUETE', 'MASIVA', 'COMPRAS')),
    ADD CONSTRAINT chk_sent_packages_status CHECK (
        status IN ('SENDING', 'UNKNOWN', 'SENT', 'PENDING_VALIDATION', 'ACCEPTED', 'REJECTED')
    ),
    ADD CONSTRAINT chk_sent_packages_values CHECK (
        cantidad_facturas > 0 AND modalidad IN (0, 1, 2)
        AND codigo_documento_sector > 0 AND codigo_tipo_factura > 0 AND codigo_emision > 0
    );

-- A single trigger function gives direct SQL clients the same retention rule
-- as the repositories. Deactivation and fiscal status transitions remain valid.
CREATE OR REPLACE FUNCTION prevent_hard_delete() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'hard delete is forbidden for fiscal/retained table %', TG_TABLE_NAME
        USING ERRCODE = 'integrity_constraint_violation',
              HINT = 'Deactivate or annul the row instead.';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_no_delete_tenants BEFORE DELETE ON tenants
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_branches BEFORE DELETE ON branches
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_points_of_sale BEFORE DELETE ON points_of_sale
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_customers BEFORE DELETE ON customers
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_products BEFORE DELETE ON products
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_cufd_history BEFORE DELETE ON cufd_history
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_cuis_history BEFORE DELETE ON cuis_history
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_contingency_events BEFORE DELETE ON contingency_events
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_invoices BEFORE DELETE ON invoices
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_invoice_items BEFORE DELETE ON invoice_items
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_invoice_documents BEFORE DELETE ON invoice_documents
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_invoice_events BEFORE DELETE ON invoice_events
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_sent_packages BEFORE DELETE ON sent_packages
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_sent_package_invoices BEFORE DELETE ON sent_package_invoices
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();
CREATE TRIGGER trg_no_delete_certificates BEFORE DELETE ON certificates
    FOR EACH ROW EXECUTE FUNCTION prevent_hard_delete();

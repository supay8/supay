DROP TRIGGER IF EXISTS trg_no_delete_tenants ON tenants;
DROP TRIGGER IF EXISTS trg_no_delete_branches ON branches;
DROP TRIGGER IF EXISTS trg_no_delete_points_of_sale ON points_of_sale;
DROP TRIGGER IF EXISTS trg_no_delete_customers ON customers;
DROP TRIGGER IF EXISTS trg_no_delete_products ON products;
DROP TRIGGER IF EXISTS trg_no_delete_cufd_history ON cufd_history;
DROP TRIGGER IF EXISTS trg_no_delete_cuis_history ON cuis_history;
DROP TRIGGER IF EXISTS trg_no_delete_contingency_events ON contingency_events;
DROP TRIGGER IF EXISTS trg_no_delete_invoices ON invoices;
DROP TRIGGER IF EXISTS trg_no_delete_invoice_items ON invoice_items;
DROP TRIGGER IF EXISTS trg_no_delete_invoice_documents ON invoice_documents;
DROP TRIGGER IF EXISTS trg_no_delete_invoice_events ON invoice_events;
DROP TRIGGER IF EXISTS trg_no_delete_sent_packages ON sent_packages;
DROP TRIGGER IF EXISTS trg_no_delete_sent_package_invoices ON sent_package_invoices;
DROP TRIGGER IF EXISTS trg_no_delete_certificates ON certificates;
DROP FUNCTION IF EXISTS prevent_hard_delete();

ALTER TABLE branches DROP CONSTRAINT IF EXISTS chk_branches_code, DROP CONSTRAINT IF EXISTS chk_branches_name;
ALTER TABLE products DROP CONSTRAINT IF EXISTS chk_products_identity;
ALTER TABLE customers DROP CONSTRAINT IF EXISTS chk_customers_document_type, DROP CONSTRAINT IF EXISTS chk_customers_identity;
ALTER TABLE cufd_history DROP CONSTRAINT IF EXISTS chk_cufd_history_value;
ALTER TABLE cuis_history DROP CONSTRAINT IF EXISTS chk_cuis_history_value;
ALTER TABLE invoices
    DROP CONSTRAINT IF EXISTS chk_invoices_amounts,
    DROP CONSTRAINT IF EXISTS chk_invoices_codes,
    DROP CONSTRAINT IF EXISTS chk_invoices_cancellation;
ALTER TABLE invoice_items DROP CONSTRAINT IF EXISTS chk_invoice_items_amounts;
ALTER TABLE sent_packages
    DROP CONSTRAINT IF EXISTS chk_sent_packages_type,
    DROP CONSTRAINT IF EXISTS chk_sent_packages_status,
    DROP CONSTRAINT IF EXISTS chk_sent_packages_values;

-- Drop all hardened FKs before removing their composite candidate keys.
DO $$
DECLARE
    fk record;
BEGIN
    FOR fk IN
        SELECT conrelid::regclass::text AS table_name, conname
        FROM pg_constraint
        WHERE contype = 'f'
          AND conname IN (
              'fk_branches_tenant', 'fk_points_of_sale_tenant', 'fk_points_of_sale_branch_tenant',
              'fk_products_tenant', 'fk_sin_products_tenant',
              'fk_product_mappings_product_tenant', 'fk_product_mappings_sin_product_tenant',
              'fk_cufd_history_tenant', 'fk_cufd_history_pos_tenant',
              'fk_cuis_history_tenant', 'fk_cuis_history_pos_tenant',
              'fk_contingency_events_tenant', 'fk_contingency_events_pos_tenant', 'fk_contingency_events_cufd_tenant',
              'fk_customers_tenant', 'fk_invoices_tenant', 'fk_invoices_customer_tenant',
              'fk_invoices_pos_tenant', 'fk_invoices_cufd_tenant', 'fk_invoices_contingency_tenant',
              'fk_invoices_adjustment_tenant', 'fk_invoice_items_invoice_tenant', 'fk_invoice_items_product_tenant',
              'fk_invoice_events_tenant', 'fk_invoice_events_invoice_tenant',
              'fk_sent_packages_tenant', 'fk_sent_packages_pos_tenant', 'fk_sent_packages_cufd_tenant',
              'fk_sent_packages_contingency_tenant', 'fk_sent_package_invoices_invoice_tenant',
              'fk_sent_package_invoices_package_tenant', 'fk_certificates_tenant',
              'fk_certificates_pos_tenant', 'fk_certificates_renewed_from_tenant',
              'fk_certificate_notifications_tenant', 'fk_certificate_notifications_certificate_tenant',
              'fk_outbox_tenant', 'fk_catalog_sync_tenant', 'fk_catalog_sync_pos_tenant',
              'fk_invoice_sequences_tenant', 'fk_invoice_sequences_pos_tenant'
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.table_name, fk.conname);
    END LOOP;
END $$;

-- Restore the pre-hardening referential actions.
ALTER TABLE branches ADD CONSTRAINT fk_branches_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE points_of_sale
    ADD CONSTRAINT fk_points_of_sale_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_points_of_sale_branch FOREIGN KEY (branch_id) REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE products ADD CONSTRAINT products_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE sin_products ADD CONSTRAINT fk_sin_products_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE product_mappings
    ADD CONSTRAINT product_mappings_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(id),
    ADD CONSTRAINT product_mappings_sin_product_id_fkey FOREIGN KEY (sin_product_id) REFERENCES sin_products(id);
ALTER TABLE cufd_history
    ADD CONSTRAINT fk_cufd_history_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_cufd_history_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE;
ALTER TABLE cuis_history
    ADD CONSTRAINT cuis_history_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT cuis_history_point_of_sale_id_fkey FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE;
ALTER TABLE contingency_events
    ADD CONSTRAINT fk_contingency_events_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_contingency_events_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_contingency_events_cufd FOREIGN KEY (cufd_id) REFERENCES cufd_history(id) ON DELETE RESTRICT;
ALTER TABLE customers ADD CONSTRAINT customers_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE invoices
    ADD CONSTRAINT invoices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    ADD CONSTRAINT invoices_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES customers(id),
    ADD CONSTRAINT invoices_point_of_sale_id_fkey FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id),
    ADD CONSTRAINT invoices_cufd_id_fkey FOREIGN KEY (cufd_id) REFERENCES cufd_history(id),
    ADD CONSTRAINT invoices_contingency_event_id_fkey FOREIGN KEY (contingency_event_id) REFERENCES contingency_events(id),
    ADD CONSTRAINT invoices_ajusta_factura_id_fkey FOREIGN KEY (ajusta_factura_id) REFERENCES invoices(id);
ALTER TABLE invoice_items
    ADD CONSTRAINT invoice_items_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES invoices(id),
    ADD CONSTRAINT invoice_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(id);
ALTER TABLE invoice_events
    ADD CONSTRAINT invoice_events_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES invoices(id),
    ADD CONSTRAINT fk_invoice_events_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE sent_packages
    ADD CONSTRAINT sent_packages_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    ADD CONSTRAINT sent_packages_point_of_sale_id_fkey FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id),
    ADD CONSTRAINT sent_packages_cufd_id_fkey FOREIGN KEY (cufd_id) REFERENCES cufd_history(id),
    ADD CONSTRAINT sent_packages_contingency_event_id_fkey FOREIGN KEY (contingency_event_id) REFERENCES contingency_events(id);
ALTER TABLE sent_package_invoices
    ADD CONSTRAINT sent_package_invoices_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES invoices(id),
    ADD CONSTRAINT sent_package_invoices_sent_package_id_fkey FOREIGN KEY (sent_package_id) REFERENCES sent_packages(id);
ALTER TABLE certificates
    ADD CONSTRAINT fk_certificates_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_certificates_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE SET NULL;
ALTER TABLE certificate_notifications
    ADD CONSTRAINT certificate_notifications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT certificate_notifications_certificate_id_fkey FOREIGN KEY (certificate_id) REFERENCES certificates(id) ON DELETE CASCADE;
ALTER TABLE outbox ADD CONSTRAINT outbox_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE catalog_sync_states
    ADD CONSTRAINT fk_catalog_sync_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_catalog_sync_pos FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE;
ALTER TABLE invoice_sequences
    ADD CONSTRAINT invoice_sequences_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT invoice_sequences_point_of_sale_id_fkey FOREIGN KEY (point_of_sale_id) REFERENCES points_of_sale(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_invoice_sequences_pos_tenant FOREIGN KEY (tenant_id, point_of_sale_id)
        REFERENCES points_of_sale(tenant_id, id) ON DELETE CASCADE;

DROP INDEX IF EXISTS uq_branches_tenant_id;
DROP INDEX IF EXISTS uq_customers_tenant_id;
DROP INDEX IF EXISTS uq_products_tenant_id;
DROP INDEX IF EXISTS uq_sin_products_tenant_id;
DROP INDEX IF EXISTS uq_cufd_history_tenant_id;
DROP INDEX IF EXISTS uq_cuis_history_tenant_id;
DROP INDEX IF EXISTS uq_contingency_events_tenant_id;
DROP INDEX IF EXISTS uq_invoices_tenant_id;
DROP INDEX IF EXISTS uq_sent_packages_tenant_id;
DROP INDEX IF EXISTS uq_certificates_tenant_id;

ALTER TABLE invoices ALTER COLUMN customer_id DROP NOT NULL;
ALTER TABLE invoice_items
    ALTER COLUMN quantity TYPE numeric(18,3),
    ALTER COLUMN unit_price TYPE numeric(18,2);
ALTER TABLE product_mappings DROP COLUMN tenant_id;
ALTER TABLE invoice_items DROP COLUMN tenant_id;
ALTER TABLE sent_package_invoices DROP COLUMN tenant_id;
ALTER TABLE customers DROP COLUMN is_active;
ALTER TABLE tenants DROP COLUMN is_active;

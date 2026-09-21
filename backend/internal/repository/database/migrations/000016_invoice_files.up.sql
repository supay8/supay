CREATE TABLE invoice_files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES tenants(id),
    invoice_id uuid NOT NULL,
    kind varchar(20) NOT NULL,
    storage_key text NOT NULL UNIQUE,
    sha256 char(64) NOT NULL,
    size bigint NOT NULL CHECK (size >= 0),
    content_type varchar(100) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoice_files_invoice_tenant_fk FOREIGN KEY (company_id, invoice_id)
        REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT invoice_files_kind_check CHECK (kind IN ('xml', 'pdf')),
    CONSTRAINT invoice_files_invoice_kind_unique UNIQUE (invoice_id, kind)
);
CREATE INDEX invoice_files_company_invoice_idx ON invoice_files (company_id, invoice_id);

-- Persistir la identidad del envío antes de contactar SIAT.
ALTER TABLE sent_packages
    ADD COLUMN modalidad integer NOT NULL DEFAULT 0,
    ADD COLUMN layout varchar(80) NOT NULL DEFAULT '',
    ADD COLUMN cufd text NOT NULL DEFAULT '',
    ADD COLUMN cuis text NOT NULL DEFAULT '',
    ADD COLUMN cufd_id uuid REFERENCES cufd_history(id);

-- Las reservas todavía no tienen un código de recepción asignado por SIAT.
ALTER TABLE sent_packages DROP CONSTRAINT IF EXISTS sent_packages_codigo_recepcion_key;
DROP INDEX IF EXISTS idx_sent_packages_codigo_recepcion;
CREATE UNIQUE INDEX idx_sent_packages_recepcion
    ON sent_packages(codigo_recepcion) WHERE codigo_recepcion <> '';

CREATE TABLE sent_package_invoices (
    invoice_id uuid PRIMARY KEY REFERENCES invoices(id),
    sent_package_id uuid NOT NULL REFERENCES sent_packages(id),
    position integer NOT NULL CHECK (position >= 0),
    CONSTRAINT idx_sent_package_invoice_position UNIQUE (sent_package_id, position)
);

-- Conservar la relación de envíos históricos cuando la recepción permite
-- identificar inequívocamente la pertenencia dentro del mismo tenant y POS.
INSERT INTO sent_package_invoices (invoice_id, sent_package_id, position)
SELECT i.id, p.id,
       row_number() OVER (PARTITION BY p.id ORDER BY i.invoice_number, i.id) - 1
FROM invoices i
JOIN sent_packages p ON p.codigo_recepcion = i.siat_reception_code
    AND p.tenant_id = i.tenant_id AND p.point_of_sale_id = i.point_of_sale_id
WHERE p.codigo_recepcion <> '';

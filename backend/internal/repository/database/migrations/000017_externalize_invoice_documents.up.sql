-- Los bytes de los documentos fiscales pertenecen al object storage. Antes de
-- retirar las copias legacy, exigir que cada XML histórico tenga metadata cuyo
-- hash corresponda exactamente con el contenido que será eliminado.
DO $$
BEGIN
    IF EXISTS (
        WITH legacy_xml AS (
            SELECT i.tenant_id AS company_id, i.id AS invoice_id, i.xml AS content
            FROM invoices i
            WHERE i.xml IS NOT NULL AND btrim(i.xml) <> ''
            UNION ALL
            SELECT i.tenant_id, i.id, d.content
            FROM invoice_documents d
            JOIN invoices i ON i.id = d.invoice_id
            WHERE d.document_type = 'XML'
              AND d.content IS NOT NULL
              AND btrim(d.content) <> ''
        )
        SELECT 1
        FROM legacy_xml x
        WHERE NOT EXISTS (
              SELECT 1
              FROM invoice_files f
              WHERE f.company_id = x.company_id
                AND f.invoice_id = x.invoice_id
                AND f.kind = 'xml'
                AND lower(btrim(f.sha256)) = encode(digest(x.content, 'sha256'), 'hex')
          )
    ) THEN
        RAISE EXCEPTION 'cannot externalize invoice XML: historical rows are not backed up in invoice_files'
            USING HINT = 'Run go run ./cmd/backfill-invoice-xml with the production storage configuration, then retry migrations.';
    END IF;
END $$;

-- invoice_documents conserva únicamente metadata histórica. invoice_files es
-- el catálogo canónico utilizado para localizar los bytes en object storage.
UPDATE invoice_documents d
SET content = NULL,
    storage_ref = f.storage_key,
    sha256 = f.sha256,
    mime_type = f.content_type
FROM invoices i
JOIN invoice_files f
  ON f.company_id = i.tenant_id
 AND f.invoice_id = i.id
 AND f.kind = 'xml'
WHERE d.invoice_id = i.id
  AND d.document_type = 'XML'
  AND d.content IS NOT NULL
  AND btrim(d.content) <> ''
  AND lower(btrim(f.sha256)) = encode(digest(d.content, 'sha256'), 'hex');

-- xml_hash pasa a representar el SHA-256 del XML canónico almacenado, no el
-- hash del archivo comprimido enviado al SIAT (que permanece en hash_archivo).
UPDATE invoices i
SET xml_hash = f.sha256
FROM invoice_files f
WHERE f.company_id = i.tenant_id
  AND f.invoice_id = i.id
  AND f.kind = 'xml';

ALTER TABLE invoices
	DROP COLUMN xml;

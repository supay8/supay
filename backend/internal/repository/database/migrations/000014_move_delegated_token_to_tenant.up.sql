-- El token delegado pertenece al tenant, no al certificado. La aplicación
-- guarda aquí el valor cifrado mediante ENCRYPTION_KEY.
ALTER TABLE tenant_configs
    RENAME COLUMN token_siat TO token_delegado;

UPDATE tenant_configs
SET token_delegado = ''
WHERE token_delegado IS NULL;

ALTER TABLE tenant_configs
    ALTER COLUMN token_delegado SET DEFAULT '',
    ALTER COLUMN token_delegado SET NOT NULL;

-- Conserva los tokens cifrados cargados mediante el flujo antiguo antes de
-- retirar las columnas duplicadas del certificado.
UPDATE tenant_configs tc
SET token_delegado = (
    SELECT c.encrypted_token
    FROM certificates c
    WHERE c.tenant_id = tc.tenant_id
      AND c.status = 'ACTIVE'
      AND btrim(c.encrypted_token) <> ''
    ORDER BY c.updated_at DESC, c.created_at DESC
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1
    FROM certificates c
    WHERE c.tenant_id = tc.tenant_id
      AND c.status = 'ACTIVE'
      AND btrim(c.encrypted_token) <> ''
);

-- certificates conserva únicamente material y metadatos de firma digital.
ALTER TABLE certificates
    DROP COLUMN IF EXISTS encrypted_token,
    DROP COLUMN IF EXISTS modalidad,
    DROP COLUMN IF EXISTS ambiente,
    DROP COLUMN IF EXISTS nit;

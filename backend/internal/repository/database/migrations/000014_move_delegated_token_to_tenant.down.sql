ALTER TABLE certificates
    ADD COLUMN encrypted_token text NOT NULL DEFAULT '',
    ADD COLUMN modalidad integer,
    ADD COLUMN ambiente varchar(20),
    ADD COLUMN nit varchar(20) NOT NULL DEFAULT '';

UPDATE certificates c
SET encrypted_token = tc.token_delegado
FROM tenant_configs tc
WHERE tc.tenant_id = c.tenant_id
  AND c.status = 'ACTIVE';

ALTER TABLE tenant_configs
    RENAME COLUMN token_delegado TO token_siat;

ALTER TABLE tenant_configs
    ALTER COLUMN token_siat DROP NOT NULL,
    ALTER COLUMN token_siat DROP DEFAULT;

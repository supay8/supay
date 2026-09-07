UPDATE tenant_configs
SET codigo_sistema = ''
WHERE codigo_sistema IS NULL;

ALTER TABLE tenant_configs
    ALTER COLUMN codigo_sistema SET NOT NULL,
    ALTER COLUMN codigo_sistema DROP DEFAULT;
ALTER TABLE tenant_configs
    ALTER COLUMN codigo_sistema DROP NOT NULL,
    ALTER COLUMN codigo_sistema SET DEFAULT '';
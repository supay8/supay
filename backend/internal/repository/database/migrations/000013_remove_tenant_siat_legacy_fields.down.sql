ALTER TABLE tenant_configs
    ADD COLUMN codigo_sistema varchar(100) NOT NULL DEFAULT '',
    ADD COLUMN api_token text;

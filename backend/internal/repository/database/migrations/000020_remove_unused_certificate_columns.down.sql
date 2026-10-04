ALTER TABLE certificates
    ADD COLUMN IF NOT EXISTS issuer text,
    ADD COLUMN IF NOT EXISTS subject text,
    ADD COLUMN IF NOT EXISTS thumbprint varchar(100),
    ADD COLUMN IF NOT EXISTS siat_user_code varchar(50),
    ADD COLUMN IF NOT EXISTS config_path text,
    ADD COLUMN IF NOT EXISTS renewed_from uuid;

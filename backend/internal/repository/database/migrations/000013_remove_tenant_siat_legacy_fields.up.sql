-- codigo_sistema identifica al sistema proveedor (Supay), no al tenant.
-- Su única fuente de verdad es SIAT_CODIGO_SISTEMA.
-- api_token nunca participó en autenticación HTTP ni en llamadas al SIAT.
ALTER TABLE tenant_configs
    DROP COLUMN IF EXISTS codigo_sistema,
    DROP COLUMN IF EXISTS api_token;

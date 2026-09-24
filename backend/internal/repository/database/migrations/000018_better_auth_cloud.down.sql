ALTER TABLE tenants
    DROP CONSTRAINT IF EXISTS uq_tenants_auth_organization,
    DROP CONSTRAINT IF EXISTS fk_tenants_auth_organization,
    DROP COLUMN IF EXISTS auth_organization_id;

DROP SCHEMA IF EXISTS auth CASCADE;

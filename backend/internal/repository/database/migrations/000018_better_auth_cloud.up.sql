-- Better Auth v1.7 cloud contract.
--
-- Go/golang-migrate is the only schema owner. The Next.js database role must
-- receive DML privileges on auth.* but must not receive CREATE/ALTER/DROP.
CREATE SCHEMA IF NOT EXISTS auth;

COMMENT ON SCHEMA auth IS
    'Better Auth runtime data. Schema and migrations are owned by the Supay Go backend.';

CREATE TABLE auth.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    email varchar(320) NOT NULL,
    email_verified boolean NOT NULL DEFAULT false,
    image text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_better_auth_users_email UNIQUE (email)
);

CREATE TABLE auth.organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    slug text NOT NULL,
    logo text,
    metadata text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_better_auth_organizations_slug UNIQUE (slug)
);

CREATE TABLE auth.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token text NOT NULL,
    expires_at timestamptz NOT NULL,
    ip_address text,
    user_agent text,
    active_organization_id uuid REFERENCES auth.organizations(id) ON DELETE SET NULL,
    active_team_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_better_auth_sessions_token UNIQUE (token)
);

CREATE TABLE auth.accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    account_id text NOT NULL,
    provider_id text NOT NULL,
    access_token text,
    refresh_token text,
    access_token_expires_at timestamptz,
    refresh_token_expires_at timestamptz,
    scope text,
    id_token text,
    password text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_better_auth_accounts_provider UNIQUE (provider_id, account_id)
);

CREATE TABLE auth.verifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identifier text NOT NULL,
    value text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth.members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES auth.organizations(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'member',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_better_auth_members_user_org UNIQUE (user_id, organization_id),
    CONSTRAINT chk_better_auth_members_role_not_blank CHECK (btrim(role) <> '')
);

CREATE TABLE auth.invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email varchar(320) NOT NULL,
    inviter_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES auth.organizations(id) ON DELETE CASCADE,
    role text,
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE TABLE auth.jwks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    public_key text NOT NULL,
    private_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz
);

CREATE INDEX idx_better_auth_sessions_user ON auth.sessions(user_id);
CREATE INDEX idx_better_auth_sessions_active_org ON auth.sessions(active_organization_id);
CREATE INDEX idx_better_auth_accounts_user ON auth.accounts(user_id);
CREATE INDEX idx_better_auth_verifications_identifier ON auth.verifications(identifier);
CREATE INDEX idx_better_auth_members_organization ON auth.members(organization_id);
CREATE INDEX idx_better_auth_members_user ON auth.members(user_id);
CREATE INDEX idx_better_auth_invitations_organization ON auth.invitations(organization_id);
CREATE INDEX idx_better_auth_invitations_email ON auth.invitations(email);

-- A Better Auth organization represents the identity/authorization boundary;
-- tenants remains the fiscal/business aggregate owned by Go. Provisioning
-- links both without forcing self-hosted tenants to have a cloud organization.
ALTER TABLE tenants
    ADD COLUMN auth_organization_id uuid,
    ADD CONSTRAINT fk_tenants_auth_organization
        FOREIGN KEY (auth_organization_id)
        REFERENCES auth.organizations(id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT uq_tenants_auth_organization UNIQUE (auth_organization_id);


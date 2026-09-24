# Contrato Better Auth para Supay Cloud

Este documento fija el contrato de Better Auth **v1.7.x** que corresponde a la
migración Go `000018_better_auth_cloud`. Next.js usa las tablas, pero no es
propietario de su estructura.

## Propiedad de la base de datos

- Solo `golang-migrate`, desde el backend Go, puede crear o alterar tablas.
- No ejecutar `auth migrate`, `auth generate` contra producción ni migraciones
  de Prisma/Drizzle desde Next.js.
- El rol PostgreSQL de Next.js debe tener `USAGE` sobre el esquema `auth` y
  `SELECT`, `INSERT`, `UPDATE`, `DELETE` sobre `auth.*`; no debe tener
  `CREATE`, `ALTER`, `DROP` ni privilegios sobre las tablas fiscales.
- Al actualizar Better Auth se debe comparar primero su esquema requerido con
  esta migración y crear una nueva migración Go si existe drift.

## Configuración que Next.js debe respetar

El adaptador PostgreSQL debe apuntar al esquema `auth`, usar UUID en todos los
modelos y mapear los nombres camelCase de Better Auth a las columnas snake_case
creadas por Go. Configuración de referencia:

```ts
import { betterAuth } from "better-auth";
import { jwt, organization } from "better-auth/plugins";
import { PostgresDialect } from "kysely";
import { Pool } from "pg";

const commonFields = {
  createdAt: "created_at",
  updatedAt: "updated_at",
};

export const auth = betterAuth({
  baseURL: process.env.BETTER_AUTH_URL!,
  database: {
    dialect: new PostgresDialect({
      pool: new Pool({ connectionString: process.env.DATABASE_URL }),
    }),
    type: "postgres",
    schemaName: "auth",
  },
  advanced: {
    database: {
      generateId: "uuid",
      joins: true,
    },
  },
  user: {
    modelName: "users",
    fields: {
      emailVerified: "email_verified",
      ...commonFields,
    },
  },
  session: {
    modelName: "sessions",
    fields: {
      userId: "user_id",
      expiresAt: "expires_at",
      ipAddress: "ip_address",
      userAgent: "user_agent",
      ...commonFields,
    },
  },
  account: {
    modelName: "accounts",
    encryptOAuthTokens: true,
    fields: {
      userId: "user_id",
      accountId: "account_id",
      providerId: "provider_id",
      accessToken: "access_token",
      refreshToken: "refresh_token",
      accessTokenExpiresAt: "access_token_expires_at",
      refreshTokenExpiresAt: "refresh_token_expires_at",
      idToken: "id_token",
      ...commonFields,
    },
  },
  verification: {
    modelName: "verifications",
    fields: {
      expiresAt: "expires_at",
      ...commonFields,
    },
  },
  emailAndPassword: {
    enabled: true,
    requireEmailVerification: true,
  },
  socialProviders: {
    google: {
      clientId: process.env.GOOGLE_CLIENT_ID!,
      clientSecret: process.env.GOOGLE_CLIENT_SECRET!,
    },
    github: {
      clientId: process.env.GITHUB_CLIENT_ID!,
      clientSecret: process.env.GITHUB_CLIENT_SECRET!,
    },
  },
  plugins: [
    organization({
      disableOrganizationDeletion: true,
      requireEmailVerificationOnInvitation: true,
      schema: {
        organization: {
          modelName: "organizations",
          fields: { createdAt: "created_at" },
        },
        member: {
          modelName: "members",
          fields: {
            userId: "user_id",
            organizationId: "organization_id",
            createdAt: "created_at",
          },
        },
        invitation: {
          modelName: "invitations",
          fields: {
            inviterId: "inviter_id",
            organizationId: "organization_id",
            expiresAt: "expires_at",
            createdAt: "created_at",
          },
        },
        session: {
          fields: {
            activeOrganizationId: "active_organization_id",
            activeTeamId: "active_team_id",
          },
        },
      },
    }),
    jwt({
      jwt: {
        issuer: process.env.BETTER_AUTH_URL!,
        audience: process.env.BETTER_AUTH_AUDIENCE ?? process.env.BETTER_AUTH_URL!,
        expirationTime: "15m",
      },
      schema: {
        jwks: {
          modelName: "jwks",
          fields: {
            publicKey: "public_key",
            privateKey: "private_key",
            createdAt: "created_at",
            expiresAt: "expires_at",
          },
        },
      },
    }),
  ],
});
```

En el cliente de Next.js se habilitan los dos plugins correspondientes. Este
cliente no recibe un ORM ni ejecuta migraciones:

```ts
import { createAuthClient } from "better-auth/react";
import {
  jwtClient,
  organizationClient,
} from "better-auth/client/plugins";

export const authClient = createAuthClient({
  plugins: [organizationClient(), jwtClient()],
});
```

`BETTER_AUTH_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
`GITHUB_CLIENT_ID` y `GITHUB_CLIENT_SECRET` pertenecen al despliegue de
Next.js. El backend Go solo necesita la URL pública, issuer, audience y JWKS.

La configuración exacta debe verificarse al fijar la versión de `better-auth`;
el contrato físico que no puede cambiar sin migración es el SQL de
`000018_better_auth_cloud.up.sql`.

## Flujo de autenticación de la API Go

1. El usuario inicia sesión en Next.js con email/password, Google o GitHub.
2. El cliente obtiene un access token mediante `authClient.token()` del plugin
   JWT.
3. Envía `Authorization: Bearer <token>` y `X-Company-ID: <tenant UUID>` al API.
4. Go valida firma, `kid`, algoritmo, `iss`, `aud`, `exp` y `sub` contra JWKS.
5. Go consulta `auth.members` y exige que la organización de la membresía sea la
   vinculada al tenant solicitado mediante `tenants.auth_organization_id`.

El token nunca concede acceso a un tenant por sí mismo; la membresía vigente se
comprueba en PostgreSQL en cada petición.

## Provisionamiento de organizaciones

`auth.organizations` es la organización de identidad y `tenants` es la empresa
fiscal. Después de crear una organización, un hook server-side de Next.js debe
provisionar la empresa mediante `POST /v1/internal/companies`, enviando
`X-Backend-Token` y el UUID de la organización:

```json
{
  "auth_organization_id": "fb0324c1-15f8-470a-81d6-28b4b851b031",
  "nit": "123456789",
  "business_name": "Mi Empresa SRL",
  "ambiente": "PILOTO",
  "modalidad": 1
}
```

La restricción única garantiza una relación 1:1. Los tenants self-hosted pueden
mantener `auth_organization_id` en `NULL` y continúan usando `users`,
`user_tenants` y el JWT simétrico existentes.

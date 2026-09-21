# Frontend self-hosted

Este host Vite monta `SupayDashboard` desde `../packages/dashboard/src` en desarrollo y desde el build de `@supay/dashboard` en producción.

## Desarrollo

Desde la raíz del repositorio:

```bash
pnpm --filter frontend dev
```

- Edita rutas, páginas y componentes visibles en `packages/dashboard/src`.
- Edita la configuración del backend en `frontend/src/self-hosted-host.ts`.
- Configura `VITE_API_URL` para apuntar al backend; sin ella se usa `http://localhost:8081`.
- Las páginas activas viven únicamente en `packages/dashboard/src/pages`.
- El alias `@` apunta a `frontend/src`; `@supay/dashboard` apunta a `packages/dashboard/src` durante el desarrollo.

Para comprobar HMR, abre una ruta activa, cambia un texto de su archivo en `packages/dashboard/src/pages` y confirma que aparece sin reiniciar Vite. Revierte el texto de prueba al terminar.

## Verificación

```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
```

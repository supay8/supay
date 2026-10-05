# Fern movido → supay-docs

La documentación Fern y el SDK TypeScript vivían aquí:

- `backend/fern/` → ahora `../supay-docs/` (raíz: `docs.yml`, `*.openapi.yml`, `docs/`, `assets/`)
- `backend/sdks/typescript/` → ahora `../supay-docs/sdks/typescript/`

Comandos desde el nuevo repo:

```bash
cd ../supay-docs
fern docs dev
fern generate --local --group local --force
```

No editar copias viejas: la fuente de verdad es `supay-docs`.

# Flujo operativo: Sucursales + Puntos de Venta + CUIS + CUFD

Este documento describe el flujo para dar de alta un punto de venta operativo
ante el SIAT a partir de una sucursal, y los endpoints HTTP asociados.

## Resumen del flujo (`POST /branches/{branchId}/points-of-sale`)

El endpoint deriva la **empresa** y el **`codigo_sucursal`** de la sucursal
(`branchId`). El cliente solo envía:

```json
{
  "name": "Caja Principal",
  "description": "Caja principal",
  "codigo_tipo_punto_venta": 1,
  "local_code": 1
}
```

Campos:

| Campo                     | Requerido | Descripción |
|---------------------------|-----------|-------------|
| `name`                    | Sí        | Nombre del punto de venta (validado contra SIAT) |
| `codigo_tipo_punto_venta` | No        | Clasificador del catálogo sincronizado; si se omite se usa el primero |
| `local_code`              | No        | `codigoPuntoVenta` local (sucursal que atiende sin sistema externo); si se omite se usa 0 |
| `description`             | No        | Descripción interna (default "Punto de venta Supay") |

### Pasos internos

1. **CREATING** — Se crea el registro local con estado `CREATING`.
2. **Paso A — CUIS de sucursal (transitorio)**: se solicita CUIS con
   `codigoPuntoVenta=0` (pertenece a la sucursal, **no se persiste**).
3. **Paso A — Sincronizar catálogo**: `sincronizarParametricaTipoPuntoVenta`
   se persiste en `tipo_punto_ventas` y se valida `codigo_tipo_punto_venta`.
4. **Paso B — Registro**: `registroPuntoVenta` contra FacturacionOperaciones.
5. **Paso C — Confirmación**: solo si `transaccion=true` y `codigoPuntoVenta>0`
   se persiste el **código oficial** (`siat_code`), estado `OPERATIVE`, y se
   solicita **CUIS propio del punto de venta** (con el código oficial) y el
   **CUFD** con ese CUIS (ambos persisten en `cuis` / `cufds`).

Si el SIAT rechaza el registro, el punto queda en `ERROR` con
`siat_error`/`siat_response` y **no** se solicitan CUIS/CUFD.

## Estado tributario (`GET /points-of-sale/{id}/status`)

```json
{
  "branch_code": 1,
  "siat_point_of_sale_code": 3,
  "local_code": 1,
  "cuis": "CUIS-ABC-123",
  "cufd": "CUFD-XYZ-789",
  "cufd_expires_at": "2026-12-31T23:59:59-04:00",
  "status": "OPERATIVE"
}
```

## Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/branches/{branchId}/points-of-sale` | Alta + aprovisionamiento SIAT |
| GET  | `/branches/{branchId}/points-of-sale` | Listar puntos de venta de la sucursal |
| GET  | `/points-of-sale/{id}` | Obtener punto de venta |
| GET  | `/points-of-sale/{id}/status` | Estado tributario (sucursal/código oficial/CUIS/CUFD) |
| POST | `/companies/{companyId}/siat/sync/tipo-punto-venta` | Sincronizar catálogo de tipos de punto de venta |

## Ejemplos con curl

```bash
BASE=http://localhost:8080

# 1. Crear sucursal (codigo_sucursal >= 0, único por empresa)
curl -s -X POST "$BASE/branches" \
  -H 'Content-Type: application/json' \
  -d '{"company_id":"<COMPANY_ID>","codigo_sucursal":1,"name":"Casa Matriz","address":"Av. Balvin 123"}'

# 2. Alta y aprovisionamiento de un punto de venta
curl -s -X POST "$BASE/branches/<BRANCH_ID>/points-of-sale" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Caja Principal","codigo_tipo_punto_venta":1,"local_code":1}'

# 3. Estado tributario del punto de venta
curl -s "$BASE/points-of-sale/<POS_ID>/status"

# 4. Sincronizar catálogo de tipos de punto de venta
curl -s -X POST "$BASE/companies/<COMPANY_ID>/siat/sync/tipo-punto-venta" \
  -H 'Content-Type: application/json' \
  -d '{"codigo_sucursal":1}'
```

## Respuesta de error del SIAT

El punto de venta se persiste igual (para diagnóstico) con:

```json
{
  "error": "siat registro punto venta rechazado: ERROR-1: Punto de venta rechazado por el SIAT",
  "point_of_sale": {
    "status": "ERROR",
    "siat_error": "ERROR-1: Punto de venta rechazado por el SIAT",
    "siat_response": { "request": "<SOAP>", "response": "<SOAP>", "error": "..." }
  }
}
```

## Notas

- El CUIS de sucursal (`codigoPuntoVenta=0`) es **transitorio**: no se persiste
  en `cuis` (esa tabla queda reservada para los CUIS de puntos de venta).
- `SIAT_AMBIENTE` mapea a producción/piloto (1/2) y tiene prioridad sobre
  `SIAT_ENVIRONMENT`.
- `SIAT_MODALIDAD` se envía en todas las solicitudes SIAT.

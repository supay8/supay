# Checklist de lanzamiento — Beta cerrada Supay (piloto SIAT)

> Alcance: beta cerrada, 2–3 pilotos, sectores base (1, 8, 11, 24/47/48),
> **solo ambiente PILOTO del SIAT**. Fuera de alcance: beta abierta, producción SIAT.
> Runbooks ejecutables por terceros: `beta-selfhosted.md` · `beta-cloud.md`.

## Puerta 0 — Secretos (ambos tracks, primero)

- [ ] `git log -- backend/.env` vacío y `.env` en `.gitignore`; si se commiteó, **rotar**
      R2, OpenRouter, SIAT, DB.
- [ ] Regenerar `BACKEND_SECRET`, `ENCRYPTION_KEY`, `JWT_SECRET`, `STORAGE_SIGNING_SECRET`
      (`openssl rand -base64 48/32`). DoD: sin `cambiar-por` ni `tu_clave` en el `.env`.

## Puerta 1 — Fixes de código (común)

- [ ] Ejemplos `api-v1.md` + Postman: solo-`sku` devuelve 400; corregir a snapshot completo
      (`description`, `codigo_producto_sin`, `unidad_medida`).
- [ ] `invoice_repo.go:150-170 `Update()` → filtrar `tenant_id` + test tenantA×tenantB.
- [ ] `UNIQUE(outbox.event_type,aggregate_id)` documentado como workaround (factura nueva
      tras `REJECTED`) o corregido. Nota: `Idempotency-Key` está **retirado**
      (migración `000022_remove_idempotency_keys`): cada `POST` crea factura nueva, no reintentar a ciegas.
- [ ] DoD: `go test ./...` verde + `TEST_DATABASE_URL=... go test ./internal/repository/postgres -run TestInvoice`.

## Puerta 2 — Self-hosted → `beta-selfhosted.md`

- [ ] §§2–3: clon fresco `up` + 5 secretos + `SIAT_AMBIENTE=2`.
- [ ] §4: onboarding piloto (8 pasos) + §5 primera factura + §6 smoke `self-hosted-smoke.sh`.
- [ ] §7: backup `pg_dump` nocturno con cron. §8: dashboard `VITE_API_URL` + build verde.

## Puerta 3 — Cloud → `beta-cloud.md`

- [ ] §§2–4: 7 secretos + manifiestos sin placeholders + imagen inmutable.
- [ ] §5: Job migrate verde **antes** del servicio. §6: servicio + `/health`.
- [ ] §7: email OFF (o encendido completo con Tasks+SMTP). §8: LB excluye `/metrics|/v1/metrics`.
- [ ] §9: smoke `pilot-smoke.sh` verde por piloto (R2 privado, descargas firmadas).

## Puerta 4 — Apertura (ambos)

- [ ] Matriz tenantA×tenantB → 404/401. `PDF == DB == SIAT` en sector 1.
- [ ] `minScale=1` (cloud, semana launch). Alertas: `5xx`, outbox sin publicar, `>50 PENDING 5m`.
- [ ] Riesgos aceptados comunicados por escrito (sectores 51–55 experimentales, re-emit = factura
      nueva, v1.1 pendiente: tenant total, worker dedicado, `EmitAsync`, OTel, SDK, CI/cobertura).

## Go / No-Go

- Self-hosted 1 cliente, piloto → **Go** con Puertas 0–2 + 4.
- Cloud beta cerrada 2–3 pilotos, piloto → **Go** con Puertas 0–4.
- Beta abierta / producción SIAT → **No-Go** hasta v1.1.

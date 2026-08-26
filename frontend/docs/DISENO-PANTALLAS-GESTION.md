# Wireframes textuales — Pantallas de gestión (v0.4)

> Cierra la fase de diseño. Complementa `DISENO-UX-PANEL.md` (v0.2) y
> `DISENO-WIREFRAMES-CRITICOS.md` (v0.3). **Diseño, no implementación.**
> `[Componente]` = shadcn/ui · `↔` = mapeo al contrato v1.

---

## ⚠ Hallazgo: dependencia del contrato detectada

Al diseñar la pantalla de sectores apareció un gap: el brief define
`GET /invoices/sectores` devolviendo `{soportado, label, ejemplo, habilitado}` por sector,
pero **sin metadatos de campos** (`campos[]`). Sin eso, el formulario de emisión no puede
renderizar dinámicamente los campos adicionales del sector elegido (§1.2 de v0.3 asume
"render dirigido por datos del endpoint").

**Pedido al backend (decisión pendiente):** extender cada ítem con algo así como
`campos: [{clave, label, tipo, requerido, ejemplo}]`, o exponer
`GET /invoices/sectores/{codigo}`.
Alternativa descartada: hardcodear catálogo en el frontend (viola el principio del
contrato y se pudre con cada sector nuevo del SIAT).

Mientras tanto, estos wireframes asumen `campos[]` disponible. Si el backend lo niega,
la emisión degrada a: campos ocultos + VALIDATION_ERROR inline como único feedback
(peor UX para sectores educativos/boleto aéreo).

---

## 1. Mapa de rutas completo (sitemap)

| Ruta | Pantalla | Wireframe |
|---|---|---|
| `/invoices` | Listado de facturas (default) | v0.3 §2 |
| `/invoices/new` | Emisión | v0.3 §1 |
| `/customers` | Clientes | §5.1 acá |
| `/products` | Productos | §5.2 acá |
| `/company` | Datos fiscales | §2.1 |
| `/company/branches` | Sucursales | §2.2 |
| `/company/points-of-sale` | Puntos de venta | §2.3 |
| `/company/sectors` | Actividad económica | §2.4 |
| `/company/siat` | Conexión SIAT | §2.5 |
| `/operation/contingencia` | Contingencia | §3.1 |
| `/operation/lotes` | Envío masivo | §3.2 |
| `/operation/compras` | Compras proveedores | §3.3 |
| `/setup` | Wizard primera vez | v0.3 §3 |

Guard: cualquier ruta exige empresa con setup completo salvo `/setup`;
empresa sin conectar → redirect a `/setup` con banner explicativo.

---

## 2. Grupo "Mi empresa"

### 2.1 Datos fiscales — `/company`

```
│  DATOS DE LA EMPRESA                        [badge success Lista para facturar]
│
│  Razón social*    [Comercial Andina SRL          ]
│  NIT*             [1020304015                    ]  ⓘ readonly tras setup*
│  Dirección        [Av. Principal 123             ]
│  Teléfono         [________________              ]
│
│  ──────────────────────────────────────────────
│  Certificado digital   cert-2026.p12 · vence 12/2026  [Reemplazar…]
│                                    ← si vence en <30 días: [alert warning]
│  [ Guardar cambios ]  ← disabled sin cambios; sonner al guardar
```
\* NIT editable solo pre-setup (identidad fiscal); post-setup readonly con tooltip
explicativo. ↔ PATCH /companies/{id} · errores VALIDATION_ERROR inline (regla §5.4 v0.2).

### 2.2 Sucursales — `/company/branches`

Data-table estándar (patrón §5): Código · Descripción · # POS activos · acciones.
Crear/editar en [dialog] (código*, descripción). Eliminar → [alert-dialog] que avisa
dependencias ("tiene 2 puntos de venta") y deshabilita confirmación si existen.
↔ /branches CRUD.

### 2.3 Puntos de venta — `/company/points-of-sale` (pantalla clave del grupo)

```
│  PUNTOS DE VENTA                              [ + Nuevo punto de venta ]
│
│  ┌─ Casa Matriz · Caja 1 ────────────────────────────────────────────┐
│  │ [badge success Lista para facturar]   Código sucursal·POS: 0·1    │
│  │ CUIS vigente ✓ · Firma del día ✓ · Catálogos: hoy 08:14           │
│  │                                              Activo [switch ●──]  │
│  │                                        [Acciones ▾]               │
│  └───────────────────────────────────────────────────────────────────┘
│  ┌─ Casa Matriz · Caja 2 ────────────────────────────────────────────┐
│  │ [badge warning Sin conexión]                                      │
│  │ Este punto de venta aún no está conectado con el SIAT.            │
│  │                                  [ Conectar ahora → ] (idempotente)│
│  └───────────────────────────────────────────────────────────────────┘
```

```
Card = [card] por POS agrupados por sucursal (mejor escaneo que tabla: hay pocos POS
por empresa pero muchos datos de salud).
├─ Estado salud      [badge]: Lista para facturar / Sin conexión / Contingencia
├─ Detalle (click)   [sheet]: historial CUIS/CUFD (fechas), última sincronización,
│                     respuesta SIAT cruda colapsada ([collapsible]) para diagnóstico
├─ Acciones ▾        [dropdown-menu]: Conectar/reconectar (→ POST setup, idempotente),
│                     Solicitar firma del día, Sincronizar catálogos, Ver detalle
└─ Switch activo     [switch] ↔ PATCH /point-of-sale/{id}
```
Regla: **el usuario jamás ve "CUIS/CUFD" como acción principal** — ve "Conectar".
Los términos aparecen solo en el sheet de diagnóstico, con [tooltip].
Las acciones manuales individuales (CUIS/CUFD/sync) quedan como diagnóstico avanzado;
el camino normal es siempre el setup idempotente.

### 2.4 Actividad económica — `/company/sectors`

```
│  ACTIVIDAD ECONÓMICA                                        ⓘ fuente: SIAT
│  Tu actividad actual:  ● Compra y Venta                     [ Cambiar… ]
│  ─────────────────────────────────────────────────────────────
│  CATÁLOGO COMPLETO (solo lectura, informativo)     🔍 filtrar…
│  ┌──────────────────────────────────────────────────────────────┐
│  │ ▸ Compra y Venta                [badge success Habilitado]   │
│  │ ▸ Sectores Educativos           [badge success Habilitado]   │
│  │     "Ej.: Colegio particular — factura mensualidad…"         │
│  │     Al facturar pedirá: Nombre del estudiante*, Período*     │
│  │ ▸ Boleto Aéreo                   [badge muted No disponible] │
│  │     El SIAT no recibe boletos individuales; requiere envío   │
│  │     masivo.                                                  │
│  └──────────────────────────────────────────────────────────────┘
```
```
├─ Sector actual       radio destacado arriba; cambiarlo → [alert-dialog]:
│                       "Los próximos borradores usarán este sector. ¿Cambiar?"
├─ Lista               [accordion] dentro de [card]/[table]; búsqueda [input-group]
├─ habilitado=false    fila atenuada + razón legible
├─ soportado=false     badge "No disponible" + explicación corta
└─ Campos por sector   lista desde campos[] (ver Hallazgo ↑); requeridos con *
```
Cambia expectativa clave vs v0.1: aquí es **informativo**; el cambio efectivo de sector
por factura sigue disponible puntualmente en emisión. ↔ GET /invoices/sectores +
persistencia de elección en company (PATCH — verificar soporte del campo en contrato).

### 2.5 Conexión SIAT — `/company/siat`

Vista de diagnóstico del resultado del setup + reintento:

```
│  CONEXIÓN CON EL SIAT
│  Estado general: [badge success Conectada]   Última verificación: hoy 08:14 ↻
│
│  ☑ Registro de la empresa          hace 12 días
│  ☑ Código de autorización diario (CUIS) — vigente
│  ☑ Catálogos oficiales sincronizados — hoy 08:14
│  ☑ Código de firma del día (CUFD) — vigente
│
│  Por punto de venta:  2 de 3 conectados → [Ver puntos de venta]
│
│  [ Volver a verificar conexión ]  ← mismo POST setup, idempotente, seguro
```
Checklist persistida = misma anatomía del paso final del wizard (reuso mental).
Si algo expira → item en ámbar + botón contextual "Renovar".

---

## 3. Grupo "Operación" (tenue, cerrado por defecto)

### 3.1 Contingencia — `/operation/contingencia`

```
│  🟡 CONTINGENCIA
│  Estado: [badge warning Evento activo — corte de servicio SIAT] desde ayer 17:42
│
│  Facturas emitidas durante contingencia: 3   [Ver en listado (filtro OFFLINE)]
│  Se enviarán automáticamente al reestablecerse el servicio.
│
│  ── Historial de eventos ─────────────────────────────────────
│  Corte de servicio   ayer 17:42 → activo     3 facturas
│  Falla de red SAP    02/08 09:10 → cerrada   7 facturas ✓ enviadas
│
│  [ Registrar evento manualmente… ]  ← solo si backend no detecta solo
```
Registrar → [dialog]: tipo de evento (catálogo SIAT con labels humanos) + momento inicio.
Cerrar evento → [alert-dialog] con advertencia. ↔ POST evento-significativo.
Banner global del header enlaza acá (§4 v0.3).

### 3.2 Lotes (envío masivo) — `/operation/lotes`

```
│  ENVÍO MASIVO
│  1. Seleccioná borradores   [Ir al listado con filtro Borradores]
│     (multi-select en el listado → acción "Enviar como lote")
│  2. Paquete                 Lote #8 · 24 facturas · Bs 15.320 total
│     [badge azul Enviado] → [Validar recepción]
│  3. Resultado               22 aceptadas · 2 observadas [ver cuáles]
│
│  ── Historial ──  Lote #7 ✓ Validado · Lote #6 ⚠ Observado · …
```
Flujo mínimo: selección masiva en `/invoices` (checkboxes aparecen solo en modo lote)
+ página de seguimiento de paquetes. ↔ POST paquete + validar. Nota: sectores que no
admiten paquete (ej. Boleto Aéreo) se excluyen con mensaje claro al intentar armarlo.

### 3.3 Compras — `/operation/compras`

CRUD de lectura-simple: tabla (proveedor, NIT, importe, fecha, estado envío) +
[dialog] de registro manual + envío periódico al SIAT. Patrón §5. Máximo esfuerzo
de diseño: bajo — frecuencia mínima, no compite (regla IA v0.2).

---

## 4. Shell — anatomía final

```
┌ Sidebar [sidebar] ──────┬ Header [sticky] ──────────────────────────────┐
│ SUPAY  (logo/wordmark)  │ 🟡 banner contingencia (condicional)          │
│                         │ POS activo [select] · [+ Nueva factura][kbd N]│
│ FACTURAS                ├───────────────────────────────────────────────┤
│  Facturas   ●           │                                               │
│  Clientes               │              contenido de ruta                │
│  Productos              │                                               │
│ MI EMPRESA   (group)    │                                               │
│  ▸ Datos fiscales       │                                               │
│  ▸ Sucursales           │                                               │
│  ▸ Puntos de venta      │                                               │
│  ▸ Actividad económica  │                                               │
│  ▸ Conexión SIAT        │                                               │
│ OPERACIÓN    (tenue)    │                                               │
│  ▸ Contingencia         │                                               │
│  ▸ Lotes                │                                               │
│  ▸ Compras              │                                               │
│ ──────                  │                                               │
│ Comercial Andina SRL    │  ← footer sidebar: empresa activa + NIT       │
└─────────────────────────┴───────────────────────────────────────────────┘
Sidebar colapsable a iconos ([sidebar] collapsible="icon"). Badge de punto rojo
en "Facturas" si hay rechazadas sin resolver. Mobile: [sheet] lateral.
```

---

## 5. CRUDs estándar (patrón único reutilizado)

Patrón común para Clientes, Productos, Sucursales:

```
[data-table] (Nº/datos clave · búsqueda [input-group] · paginación)
 + [button] "+ Nuevo" → [dialog] con [field]s (validación inline envelope)
 + fila → [sheet] detalle con acciones [dropdown-menu]
 + eliminar → [alert-dialog] con chequeo de dependencias
 + vacío → [empty] con CTA · carga → [skeleton]
```

**5.1 Clientes**: columnas Nombre · Documento (NIT/CI) · Última factura. Creación
rápida ya existe en emisión (misma dialog reutilizada). ↔ /customers.

**5.2 Productos**: columnas Código · Descripción · Precio · Unidad. Detalle incluye
sección "Correspondencia SIAT" (mappings: código producto SIN, actividad) editable
inline — es lo que evita rechazos por codificación. ↔ /products + /products/{id}/mappings.

---

## 6. Estado de la fase de diseño

✅ Completa: usuarios/JTBD · IA/navegación · 4 flujos críticos · todas las pantallas ·
shell · sistema visual/estados · polling · mapeo envelope→UI · sitemap/rutas.

**Bloqueos/pendientes antes de implementar:**
1. Metadatos `campos[]` en /invoices/sectores (Hallazgo §0) — decidir con backend.
2. Confirmar dónde persiste el sector por defecto de la empresa (campo en companies).
3. Autenticación de personas sigue fuera de alcance (multitenancy llega después);
   shell reserva footer de sidebar para identidad futura.

Con 1–3 resueltos, el orden de implementación sugerido es: shell+rutas →
listado+detalle → emisión → setup → gestión → operación.

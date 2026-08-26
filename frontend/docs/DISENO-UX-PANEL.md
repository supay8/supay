# Diseño UX — Panel de administración Supay (v0.2)

> Documento de diseño para aprobación. **No es implementación**: describe pantallas,
> flujos y qué componente de shadcn/ui corresponde a cada pieza. El backend todavía no
> existe; todo se diseña contra el **Contrato v1** (rutas planas, envelope de error
> `{"error":{"code","message","details"}}`, listas `{items,total,limit,offset}`,
> `POST /invoices` → draft, `POST /invoices/{id}/emit`, `GET /invoices/sectores`,
> `POST /companies/{id}/setup` idempotente).

---

## 0. Decisiones tomadas (cierran preguntas abiertas de v0.1)

| # | Decisión | Impacto en UI |
|---|---|---|
| 1 | Multitenancy se resuelve en backend | La app trabaja siempre con **una empresa activa** por sesión; sin selector de empresas en v1 |
| 2 | Empresa primero; los puntos de venta pertenecen a la empresa | Setup: empresa → sucursal/POS. Navegación agrupada bajo "Mi empresa" |
| 3 | Sí habrá búsqueda de facturas por número/NIT/CUF | El listado incluye buscador de texto además de los filtros del contrato |
| 4 | Sí habrá polling para eventos asíncronos | Estados transitorios se auto-refrescan; patrón definido en §5.3 |
| 5 | PDF por descarga directa | Botón "Descargar PDF" en confirmación, detalle y fila |

---

## 1. Usuarios y tareas (resumen)

- **Operador de caja** — emite facturas muchas veces al día. Velocidad > todo.
- **Administrador** — configura una vez (empresa, POS, sector, certificado, conexión SIAT); maneja contingencia.
- **Contador** — lista con filtros, verifica estados SIAT, anula/revierte, descarga PDFs.

Tarea crítica: **emitir factura** → CTA global siempre visible, flujo en una sola pantalla,
cero fricción. Segunda prioridad: **listado con estados legibles de un vistazo**.

---

## 2. Inventario de componentes shadcn/ui usados

Proyecto inicializado con CLI shadcn v4 (estilo `base-vega`, Tailwind v4, lucide).
Solo `button` está agregado hoy; el resto se agregará al implementar.

| Componente | Uso principal |
|---|---|
| `sidebar` | Navegación principal colapsable |
| `button` (+ `button-group`) | CTAs, acciones de fila/formulario |
| `data-table` (table + TanStack) | Listado de facturas, clientes, productos, lotes |
| `input-group` | Buscador (facturas, clientes, productos) |
| `popover` + `command` | Combos con búsqueda: cliente, producto, sector |
| `dialog` | Creación rápida de cliente/producto sin salir del flujo |
| `alert-dialog` | Anular factura, revertir anulación, activar contingencia |
| `sheet` | Panel lateral de detalle (factura, cliente, POS) |
| `dropdown-menu` | Acciones de fila, menú de contexto de POS, menú de operación |
| `select` | Punto de venta activo, motivo de anulación simple, moneda/método de pago |
| `badge` | Sistema de estados (§5.1) |
| `alert` | Rechazos SIAT, banner de contingencia, avisos de setup |
| `radio-group` | Motivo de anulación |
| `field` / `input` / `label` | Formularios (emisión, clientes, empresa) |
| `calendar` (en `popover`) | Filtros `from`/`to` |
| `tabs` | Sub-secciones de detalle (Resumen / Items / SIAT) |
| `accordion` | Detalle de sectores y sus campos adicionales |
| `progress` + checklist | Progreso del setup (CUIS → sync → CUFD) |
| `spinner` | Envío al SIAT, carga de tablas |
| `skeleton` | Loading de listados y detalles |
| `empty` | Listados vacíos con acción sugerida |
| `sonner` | Resultados de emisión, errores no bloqueantes |
| `tooltip` | Términos técnicos (CUIS, CUFD, CUF) |
| `scroll-area` | Detalles largos dentro de Sheet |
| `separator` | Secciones del formulario de emisión |
| `pagination` | Paginación server-side del listado |
| `breadcrumb` | Ruta dentro de Mi empresa / setup |
| `switch` | Activar/desactivar POS |
| `avatar` | Menú de sesión (futuro, si hay usuarios) |
| `kbd` | Atajos (`N` nueva factura) — opcional v1 |

---

## 3. Arquitectura de información

```
Shell: Sidebar izquierda + Header superior
│
├── Header (siempre visible)
│   ├── Badge "Modo contingencia" (ámbar, solo si aplica)     → alert/banner
│   ├── Select "Punto de venta activo"                        → select
│   └── Button "+ Nueva factura"  [N]                         → button + kbd
│
├── Sidebar
│   ├── Facturas            ← vista por defecto
│   ├── Clientes
│   ├── Productos
│   ├── MI EMPRESA          ← grupo colapsable
│   │   ├── Datos fiscales
│   │   ├── Sucursales
│   │   ├── Puntos de venta
│   │   ├── Actividad económica (sectores)
│   │   └── Conexión SIAT (setup)
│   └── OPERACIÓN           ← grupo colapsable, tenue, cerrado por defecto
│       ├── Contingencia
│       ├── Lotes (envío masivo)
│       └── Compras (proveedores)
```

Reglas:
- **Emisión no es una sección**, es una acción global del header.
- Lotes/Compras visualmente más tenues que Facturas (no competir con la tarea crítica).
- No existen secciones que el contrato v1 no soporte (sin reportes, sin usuarios, sin dashboards).

---

## 4. Flujos clave, pantalla por pantalla

### 4.1 Emitir una factura (pantalla más importante)

Ruta: `/invoices/new` — página completa (no modal).

| Paso | Qué ve el usuario | Componentes |
|---|---|---|
| 1. Entrada | Click en "+ Nueva factura" desde cualquier pantalla | `button` del header |
| 2. Cliente | Buscador por nombre/NIT/CI con resultados; opción "Crear cliente nuevo…" al final de resultados | `popover` + `command`; creación rápida abre `dialog` con `field`s mínimos (nombre + documento) |
| 3. Punto de venta | Heredado del header; editable | `select` compacto |
| 4. Sector | Pre-seleccionado según actividad configurada de la empresa. Al cambiarlo, los campos adicionales aparecen/desaparecen solos (fuente: `GET /invoices/sectores` → `label`, `ejemplo`, `habilitado`, `soportado`). Requeridos vs opcionales distinguidos; placeholder/help = `ejemplo` | `select` (o combobox si la lista crece); campos dinámicos con `field` + `label`; hint con `form-description` |
| 5. Pago y moneda | Defaults pre-seleccionados | `select` ×2, plegables bajo "Opciones de pago" |
| 6. Ítems | Buscador de producto + cantidad + precio precargado + descuento; alta rápida inline; totales fijos abajo siempre visibles | Tabla editable; búsqueda de producto = `popover`+`command`; montos en footer sticky con `separator` |
| 7. Emitir | Un botón primario. Internamente: `POST /invoices` (draft) → `POST /invoices/{id}/emit`. Botón pasa a "Enviando al SIAT…", deshabilitado (nunca doble submit) | `button` con `spinner` |
| 8a. Aceptada | Vista de confirmación verde: número, CUF copiable, "Descargar PDF", "**Emitir otra**" (conserva cliente y POS) | Vista dedicada + `sonner` de éxito |
| 8b. VALIDATION_ERROR | Sin pantalla genérica: `details` pintados inline junto a cada campo, foco al primero, todo lo escrito se conserva | `field-errors` inline; foco programático |
| 8c. SIAT_REJECTED | La factura queda REJECTED (visible en listado). Banner rojo con mensajes literales del SIAT traducidos a lenguaje humano + acciones "Ver qué corregir" / "Crear corrección" (draft prellenado) | `alert` destructive + `button-group` |
| 8d. SIAT_UNAVAILABLE | Mensaje calmado: guardada. Oferta explícita "Activar contingencia" → registra evento significativo, queda OFFLINE y se envía sola al volver | `alert-dialog` de confirmación; luego badge ámbar persistente en header |

### 4.2 Configurar empresa nueva (setup)

Wizard de 4 pasos, ruta `/setup`. Idempotencia explícita en cada reintento.

| Paso | Contenido | Componentes |
|---|---|---|
| ① Datos de la empresa | Razón social, NIT, etc. | `field`s + validación inline |
| ② Sucursal y punto de venta | Alta de sucursal y primer POS | Formularios simples + `select` tipo de POS |
| ③ Actividad económica | Lista de sectores `habilitado=true`; al elegir uno se anticipan los campos extra que pedirá al facturar; `habilitado=false` atenuado con explicación; `soportado=false` marcado "no disponible" | Lista seleccionable (`item`-like) o `table` con `badge`s; preview de campos con `accordion` |
| ④ Certificado digital | Upload del certificado | Input file estilizado |
| Conectar | Un único botón "Conectar con SIAT" → `POST /companies/{id}/setup`. Checklist animada con nombres humanos: ☑ Registro ☑ Código de autorización diario *(CUIS)* ◌ Catálogos oficiales ◌ Código de firma *(CUFD)*. Términos técnicos entre paréntesis/`tooltip` | Checklist con iconos + `progress`; `tooltip` en términos |
| Fin | Badge permanente "Lista para facturar" en el POS; mismo estado vive en Mi empresa → Puntos de venta | `badge` success |
| Error a mitad | "Algo falló en el paso N. Podés reintentar con seguridad — no se duplicará nada." Mismo botón | `alert` + reintento del mismo `button` |

### 4.3 Revisar y anular

1. **Facturas** (`/invoices`): filtros persistentes arriba — punto de venta, estado (chips removibles), rango fechas, buscador de texto (número/NIT/CUF). Paginación server-side `{items,total,limit,offset}`.
   - Componentes: `data-table`; buscador = `input-group`; estado = chips (removibles); fechas = `calendar` en `popover`; `pagination`.
2. **Fila** → click abre panel lateral (no pérdida de contexto): datos, ítems, timeline de estado (creada → enviada → aceptada), mensajes SIAT, PDF.
   - Componentes: `sheet` + `scroll-area` + `tabs` (Resumen / Items / SIAT); acciones de fila también en `dropdown-menu` (Ver, Descargar PDF, Anular).
3. **Anular** (solo estados anulables): elegir motivo (catálogo SIAT en lenguaje humano) → confirmación con consecuencia explícita ("quedará anulada oficialmente ante el SIAT").
   - Componentes: `alert-dialog` con `radio-group` de motivos.
4. Resultado: estado CANCELLED con motivo y fecha. **"Revertir anulación"** en el detalle con advertencia equivalente (`alert-dialog`). Errores (fuera de plazo, etc.) viajan en el envelope → mensaje accionable, nunca genérico (`sonner` error con `message` del envelope).

### 4.4 Resolver un rechazo SIAT

1. Entradas: chip "Rechazadas (N)" sobre el listado + banner ámbar en header mientras existan sin resolver (`alert`).
2. Detalle en tres bloques: **qué pasó** (mensajes literales + fecha + código recepción), **qué significa** (traducción humana), **qué podés hacer** (acciones concretas).
3. Camino recomendado: "**Crear factura corregida**" → nuevo draft prellenado, foco en el campo problemático si es identificable. Vínculo visible entre rechazada y su corrección.

---

## 5. Sistema visual y patrones transversales

### 5.1 Estados (idénticos en fila, detalle, confirmación y header)

| Estado API | UI | Componente |
|---|---|---|
| ACCEPTED | Verde sólido + texto "Aceptada" | `badge` variant success |
| SENDING | Azul + spinner breve | `badge` + `spinner` |
| PENDING (draft) | Gris contorno "Borrador" | `badge` outline |
| REJECTED | Rojo "Rechazada" | `badge` destructive |
| OFFLINE / contingencia | Ámbar "En contingencia" | `badge` warning |
| OBSERVED | Ámbar claro "Observada" | `badge` warning suave |
| CANCELLED | Gris apagado + tachado "Anulada" | `badge` muted |

Regla dura: **color + texto siempre juntos** (accesibilidad). El ámbar escala al header global:
si algo requiere acción, se ve sin navegar.

### 5.2 Densidad del listado

Tabla compacta (~40px/fila), columnas mínimas: número, cliente, total, fecha, estado.
Todo lo demás vive en el `sheet` de detalle. Hover sutil, sin zebra. Filtros como chips
removibles sobre la tabla. Total visible ("247 facturas").

### 5.3 Polling (decisión #4)

- Auto-refresh **solo** cuando hay filas en estados transitorios (PENDING/SENDING) o contingencia activa.
- Indicador sutil "Actualizado hace Xs" + `button` refresh manual; sin toasts por cada cambio.
- Al detectarse transición a estado final de una fila visible: actualizar la celda con micro-highlight (sin modal ni interrupciones).

### 5.4 Errores del envelope → UI (regla única)

| `code` | Tratamiento |
|---|---|
| VALIDATION_ERROR | Inline junto a cada campo (`details`), foco al primero, sin perder datos |
| SIAT_REJECTED | `alert` destructive con mensaje literal del SIAT + acciones concretas |
| SIAT_UNAVAILABLE | Oferta de contingencia (`alert-dialog`); nunca bloquea el dato ya escrito |
| NotFound / Conflict / otros | `sonner` con `message` del envelope; la UI nunca inventa textos de error |

### 5.5 Loading / vacío

- Primer render de listas: `skeleton` con forma de tabla.
- Refetch posterior: mantener datos viejos + `spinner` sutil.
- Sin resultados: `empty` con acción sugerida ("No hay facturas este mes — Emitir la primera").

---

## 6. Fuera de alcance v1 (charter-style, explícito)

Usuarios/roles/permisos (depende de auth de personas), multi-empresa por sesión,
reportes/analytics, notificaciones push/webhooks, personalización de temas por tenant.

## 7. Próximo paso sugerido

Aprobado este documento → wireframe textual de las 2 pantallas críticas
(emisión y listado) con jerarquía exacta de componentes, recién después tocar código.

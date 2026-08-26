# Wireframes textuales — Pantallas críticas (v0.3)

> Complemento de `DISENO-UX-PANEL.md` (v0.2). Sigue siendo **diseño, no implementación**.
> `[Componente]` = shadcn/ui · `(variante)` = prop · `→` = interacción · `↔` = mapeo al contrato v1.

Cubre los 4 flujos aprobados: **emisión**, **listado + detalle**, **anulación/rechazo**, **setup**.

---

## 1. Emisión de factura — `/invoices/new` (página completa)

### 1.1 Wireframe

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Sidebar │  Facturas / Nueva factura                          POS activo ▾ │
│         │  ← Volver                                            [breadcrumb]│
│         ├────────────────────────────────────────────────────────────────────┤
│         │  CLIENTE                                                           │
│         │  ┌──────────────────────────────────────────────┐                  │
│         │  │ 🔍 Buscar por nombre, NIT o CI…              │ ← Command        │
│         │  └──────────────────────────────────────────────┘   en Popover    │
│         │  Seleccionado: ● Ana Pérez — CI 1234567        ✕                  │
│         │  ────────────────────────────────────────────                      │
│         │                                                                    │
│         │  PUNTO DE VENTA              SECTOR                                │
│         │  [Casa Matriz · Caja 1 ▾]    [Compra y Venta ▾  🔒preseleccionado] │
│         │                                                                    │
│         │  ── Campos de la actividad (aparecen según sector) ──              │
│         │  * Nombre del estudiante   [________________________________]      │
│         │    Período facturado       [2026-__ ]  ⓘ ej.: "2026-1er bimestre"  │
│         │  ────────────────────────────────────────────                      │
│         │                                                                    │
│         │  PAGO (plegado por defecto)                                   ▾    │
│         │  Método [Efectivo ▾]   Moneda [Bolivianos ▾]                       │
│         │  ────────────────────────────────────────────                      │
│         │                                                                    │
│         │  ÍTENS                                          [+ Agregar producto]│
│         │  ┌──────────────────────┬──────┬────────┬───────┬─────────┬───┐    │
│         │  │ Producto 🔍          │ Cant │ Precio │ Dto.  │ Subtotal│ ✕ │    │
│         │  ├──────────────────────┼──────┼────────┼───────┼─────────┼───┤    │
│         │  │ Coca-Cola 600 ml     │  2   │ 10,00  │  0,00 │   20,00 │   │    │
│         │  │ Instalación servicio │  1   │150,00  │ 10,00 │  140,00 │   │    │
│         │  └──────────────────────┴──────┴────────┴───────┴─────────┴───┘    │
│         │                                                                    │
│         ├────────────────────────────────────────────────────────────────────┤
│ (sticky)│                Subtotal  Bs 170,00                                 │
│         │                Descuento −Bs  10,00                                │
│         │                ══ TOTAL ═ Bs 160,00                                │
│         │  [Cancelar]                          [ Emitir factura →  (spinner)]│
└────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Jerarquía de componentes

```
Page (ruta protegida por contexto de empresa)
├─ Breadcrumb                     [breadcrumb]  "Facturas / Nueva factura"
├─ Section Cliente
│  ├─ Combobox búsqueda           [popover]+[command]        ↔ GET /customers?q=
│  ├─ Cliente seleccionado        chip con [button](ghost) ✕ para quitar
│  └─ Dialog creación rápida      [dialog]+[field]+[input]   ↔ POST /customers
├─ Separator                      [separator]
├─ Section Contexto fiscal
│  ├─ Select punto de venta       [select]                   (default: header)
│  ├─ Combobox sector             [select]/[command]         ↔ GET /invoices/sectores
│  │   · opciones: solo habilitado=true · soportado=false deshabilitado con razón
│  └─ Campos dinámicos del sector [field]+[input]+[label]
│      · requerido → label con * ; opcional → hint con ejemplo del endpoint
│      · render dirigido por datos del endpoint (frontend NO hardcodea catálogos)
├─ Separator
├─ Section Pago (Collapsible cerrada)   [collapsible] o acordeón simple
│  └─ Select método / moneda      [select]×2  (defaults precargados)
├─ Separator
├─ Section Ítems
│  ├─ Tabla editable              [table] + inputs por celda
│  ├─ Buscador de producto        [popover]+[command]        ↔ GET /products?q=
│  ├─ Alta rápida inline          [dialog] (nombre, precio, unidad)
│  └─ Footer sticky totales       [separator]+tipografía tabular
├─ Barra de acciones (sticky bottom, borde superior)
│  ├─ Cancelar                    [button](ghost) → confirmación si hay datos
│  └─ Emitir factura              [button] primario; enviando → [spinner], disabled
└─ Overlays de resultado          ver 1.3
```

### 1.3 Resultados post-emit (mapeo envelope → UI)

**8a · ACCEPTED** — reemplaza el formulario por vista de confirmación:

```
│  ✅ Factura emitida                                     (vista dedicada)
│     Factura N° 000124 · CUF 5A9F…(copiar 📋)   [badge success Aceptada]
│     [ Descargar PDF ]   [ Ver en listado ]   [ Emitir otra ⟳ ]
```
"Emitir otra" vuelve al form conservando cliente + POS + pago (ráfaga de caja).
Copiar CUF = [tooltip]+clipboard. PDF = descarga directa (decisión #5). Sonner de éxito breve.

**8b · VALIDATION_ERROR** — sin overlay: errores pintados inline.

```
│  * Nombre del estudiante   [________]
│    ⚠ Este campo es obligatorio para Sectores Educativos   ← field-error rojo
```
Reglas: cada entrada de `details` junto a su campo · foco automático al primero ·
cero pérdida de lo escrito · botón vuelve a estado normal.

**8c · SIAT_REJECTED** — overlay sobre el form, factura ya existe como REJECTED:

```
┌─ [alert destructive] ────────────────────────────────────┐
│ ⛔ El SIAT rechazó la factura N° 000124                   │
│ Motivo: «Mensaje literal del SIAT» + traducción humana    │
│ La factura quedó registrada como Rechazada.               │
│ [ Ver qué corregir ]  [ Crear corrección prellenada ]     │
└───────────────────────────────────────────────────────────┘
```
"Crear corrección" → nuevo draft prellenado (flujo 4.4 del doc v0.2).

**8d · SIAT_UNAVAILABLE** — decisión del usuario antes de continuar:

```
┌─ [alert-dialog] ─────────────────────────────────────────┐
│ 🟡 El SIAT no responde ahora                              │
│ Tu factura N° 000124 quedó guardada y NO se perdió.       │
│ ¿Activar modo contingencia? Quedará pendiente de envío    │
│ y se mandará sola cuando el servicio vuelva.              │
│ [ Guardar y activar contingencia ]  [ Solo guardar ]      │
└───────────────────────────────────────────────────────────┘
```
Ambas salidas conservan el draft ↔ POST evento significativo solo en la primera.
Al activarse: badge ámbar persistente en header global (ver §4.1 doc v0.2).

---

## 2. Listado de facturas — `/invoices` (vista por defecto)

### 2.1 Wireframe

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Sidebar │  Facturas                                    [ + Nueva factura ] │
│         ├────────────────────────────────────────────────────────────────────┤
│         │  FILTROS                                                           │
│         │  [POS: Todos ▾] (Aceptadas ✕)(Rechazadas ✕)(Borradores)  [01 ago – │
│         │  31 ago 📅▾]   [🔍 N° factura, NIT o CUF…            ]  ↻ hace 5s   │
│         ├────────────────────────────────────────────────────────────────────┤
│         │  Nº      Cliente              Total      Fecha        Estado       │
│         │  000123  Comercial Andina    Bs 1.250,00  hoy 14:32  ● Aceptada    │
│         │  000122  Ana Pérez           Bs    89,90  hoy 13:05  ● Rechazada ⚠ │
│         │  000121  Consumidor final    Bs    45,00  hoy 12:58  ◐ Enviando ⟳  │
│         │  000120  Juan Quispe         Bs   210,50  ayer      ○ Borrador     │
│         │  000119  Comercial Andina    Bs   980,00  ayer      ✓ Anulada      │
│         │  (fila hover sutil · click abre panel lateral derecho)             │
│         │                                                                    │
│         │                          ‹ [1] 2 3 … 9 ›        247 facturas       │
└────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Jerarquía de componentes

```
Page
├─ Header de página + CTA            [button] "+ Nueva factura"
├─ Barra de filtros (sticky top)
│  ├─ Select punto de venta          [select]        ↔ point_of_sale_id
│  ├─ Chips de estado removibles     [badge]✕ (multi-select vía [dropdown-menu]
│  │                                  con checkbox-items)   ↔ status
│  ├─ Rango de fechas                [popover]+[calendar]    ↔ from / to
│  ├─ Buscador texto                 [input-group] 🔍        ↔ q (número/NIT/CUF)
│  └─ Indicador polling              texto sutil + [button](ghost) ↻
├─ Tabla                             [data-table]
│  ├─ Columnas mínimas: Nº · Cliente · Total · Fecha · Estado
│  ├─ Estado                         [badge] del sistema §5.1 (+[spinner] si transitorio)
│  ├─ Acciones rápidas por fila      [dropdown-menu]: Ver detalle · Descargar PDF ·
│  │                                  Anular (si aplica) · Crear corrección (REJECTED)
│  └─ Click en fila → panel lateral  [sheet]
├─ Paginación                        [pagination] + total  ↔ {items,total,limit,offset}
├─ Loading                           [skeleton] con forma de tabla (primer fetch);
│                                     refetch mantiene datos + spinner sutil
└─ Vacío                             [empty]: "Sin facturas este período"
                                      + acción "Emitir la primera"
```

### 2.3 Panel lateral de detalle (Sheet)

```
┌ Sheet (derecha, ~480px, ScrollArea) ──────────────────┐
│ Factura 000122                            ✕           │
│ [badge destructive Rechazada]  hoy 13:05 · Casa Matriz│
│                                                       │
│ (Tabs Resumen | Items | SIAT)                         │
│ ── Resumen ──                                         │
│ Cliente      Ana Pérez · CI 1234567                   │
│ Total        Bs 89,90 · Efectivo                      │
│ Sector       Compra y Venta                           │
│ Timeline     ● Creada 13:04 → ◉ Enviada 13:05 →       │
│              ⛔ Rechazada 13:05                       │
│ ── Items ──   tabla simple: desc · cant · subtotal    │
│ ── SIAT ──    CUF (copiable) · código recepción ·     │
│               mensajes literales del SIAT             │
│                                                       │
│ [Descargar PDF]  [Anular…]  [Revertir anulación]      │
│  (según estado: solo acciones válidas visibles)       │
└───────────────────────────────────────────────────────┘
```
Términos técnicos (CUF, código de recepción) con [tooltip]. Acciones de estado
centralizadas: una misma función decide qué botones existen según `status`.

### 2.4 Anulación (desde Sheet o menú de fila)

```
Paso 1  [alert-dialog]
  ¿Anular factura N° 000122?
  Motivo (*): ( ) Nota de crédito        ← [radio-group], catálogo SIAT
              (•) Error de emisión          con labels humanos
              ( ) Otro motivo
  ⓘ La factura quedará anulada oficialmente ante el SIAT.
  [ Cancelar ]  [ Anular definitivamente (destructive) ]

Paso 2  → estado CANCELLED; badge tachado; motivo + fecha en tab SIAT.
Paso 3  Revertir → [alert-dialog] espejo con advertencia equivalente.
Errores → sonner destructivo con message del envelope (nunca texto inventado).
```

---

## 3. Setup — `/setup` (wizard)

```
Paso ① ② ③ ④  (stepper lineal arriba, clicables solo hacia atrás)
────────────────────────────────────────────────────────────
① Datos de la empresa     fields: razón social*, NIT*, etc.
② Sucursal y POS          alta sucursal + primer punto de venta
③ Actividad económica     lista de sectores habilitados (radio-cards):
                          [● Compra y Venta   — te pedirá: nada extra]
                          [○ Sectores Educativos — te pedirá: estudiante,
                             período]  ← anticipación desde /invoices/sectores
                          [○ …]  deshabilitados: badge "no disponible"
④ Certificado digital     dropzone/input file + validación

Última pantalla:
  [ Conectar con SIAT ]  ← ÚNICO botón; ejecuta POST /companies/{id}/setup
  Checklist animada:
   ☑ Registro de tu empresa
   ☑ Código de autorización diario (CUIS)      ← tooltip técnico
   ◌ Actualizando catálogos oficiales…  [progress]
   ◌ Código de firma del día (CUFD)
  Error a mitad: [alert] "Falló en el paso 3. Podés reintentar con
  seguridad — no se duplicará nada."  + mismo botón (idempotencia).
  Éxito: [badge success Lista para facturar] en el POS + redirect a Facturas.
```

---

## 4. Contingencia (banner global)

```
Header (toda la app, mientras dure):
┌──────────────────────────────────────────────────────────────┐
│ 🟡 Modo contingencia activo · 3 facturas pendientes de envío │
│    [Ver detalle]                                    (dismiss ✕ solo cierra el banner,
│                                                     el estado sigue en Operación) │
└──────────────────────────────────────────────────────────────┘
→ [alert warning] fijo bajo el header · click "Ver detalle" va a Operación → Contingencia
→ Al volver el SIAT: envío automático (backend) + las filas OFFLINE pasan
  a SENDING (spinner) → ACCEPTED (badge verde), sin toasts por factura;
  un único sonner de resumen: "3 facturas enviadas correctamente".
```

---

## 5. Atajos y micro-interacciones (opcional v1, barato de agregar)

| Atajo | Acción | Componente |
|---|---|---|
| `N` | Nueva factura (fuera de inputs) | listener global + [kbd] en tooltip del CTA |
| `Enter` en ítem | Agregar siguiente línea | comportamiento de tabla |
| `Esc` | Cierra dialog/sheet sin perder draft | default de los componentes |

## 6. Qué queda fuera de estos wireframes

Clientes/Productos/Sucursales (CRUD estándar data-table + dialog, mismo patrón §2),
Lotes/Compras (tablas simples de lectura + upload), menú de sesión. Si el diseño
de estas dos pantallas críticas se aprueba, esos se derivan por analogía.

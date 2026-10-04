# Plan de Mejora del Frontend — Supay Dashboard

## Resumen

Tras analizar las **19 páginas**, **29 componentes UI**, el **host self-hosted**, las **rutas del backend Go** y los **tipos/interfaces**, identifiqué las siguientes áreas de mejora organizadas por prioridad.

---

## 🔴 Prioridad Alta — UX Profesional y Resolución de IDs

### 1. Dashboard Home — De KPIs planos a un panel accionable premium
**Archivo:** [`dashboard-home-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/dashboard-home-page.tsx)

- Las `StatCard` actuales son cajas rectangulares planas sin identidad visual
- **Mejora:** Rediseñar con gradientes sutiles, iconos de estado con animación pulse, y mini-sparklines de tendencia
- Agregar sección de "Actividad reciente" con las últimas 5 facturas (cualquier estado) con transiciones animadas
- Agregar indicador visual de salud general del sistema (semáforo SIAT)

### 2. Puntos de Venta — Dejar de mostrar IDs, mostrar nombres
**Archivo:** [`points-of-sale-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/points-of-sale-page.tsx)

- Actualmente muestra `branchName(pos.branch_id)` que puede caer en `Sucursal ${codigo}` cuando no encuentra el nombre — feo
- El campo `pos.description` se muestra sin contexto ni jerarquía visual
- **Mejora:** Crear tarjetas ricas con el nombre de la sucursal como badge superior, el nombre del POS prominente, y un indicador visual de estado tipo semáforo con animación
- En el detalle Sheet, reemplazar códigos crudos por información legible con tooltips explicativos

### 3. Facturas — Tabla profesional con nombres en lugar de IDs
**Archivo:** [`invoices-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/invoices-page.tsx)

- La tabla ya muestra `customer.name` correctamente ✅
- **Falta:** Mostrar el nombre del punto de venta en cada fila (actualmente no se muestra, solo se puede filtrar)
- Agregar columna "Punto de venta" que resuelva `point_of_sale.description` del objeto invoice
- Mejorar el filtro de POS para que se muestre en un Combobox más visualmente atractivo

### 4. Formulario Nueva Factura — UX premium de emisión
**Archivo:** [`invoice-new-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/invoice-new-page.tsx)

- Los selects de POS muestran `pos.description` ✅ pero el diseño es funcional sin ser atractivo
- La tabla de ítems es densa y confusa con 4 inputs técnicos (SKU, Actividad, Código SIN, Unidad)
- **Mejora:** Agrupar los datos fiscales del ítem en un tooltip/popover para limpiar la tabla
- Agregar animación de entrada para nuevas líneas
- Mejorar la barra sticky inferior con glassmorphism y breakdown visual del total

### 5. Detalle de Factura Sheet — Resolución completa de relaciones
**Archivo:** [`invoice-detail-sheet.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/components/invoices/invoice-detail-sheet.tsx)

- Ya muestra `invoice.point_of_sale.description` ✅
- Ya muestra `invoice.customer.name` ✅
- **Mejora:** Agregar nombre de la sucursal (resolved desde branches) 
- Mejorar visualmente los tabs con transiciones suaves
- El CUF largo necesita un diseño tipo código con copy mejorado

---

## 🟡 Prioridad Media — Estética y Consistencia Visual

### 6. Estilos Globales — Elevar la estética general
**Archivo:** [`styles.css`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/styles.css)

- El token `--secondary: #ffeeefe;` tiene un typo (hex inválido 7 caracteres)
- Agregar tokens para `--success`, `--warning`, `--info` que se usan en clases pero no están definidos
- Agregar animaciones globales: `fadeIn`, `slideUp`, `scaleIn` para transiciones de entrada
- Mejorar el glassmorphism del header

### 7. Sucursales — Tabla con más contexto visual
**Archivo:** [`branches-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/branches-page.tsx)

- Las sucursales se muestran bien pero sin indicadores de cuántos POS tiene cada una
- **Mejora:** Agregar badge con conteo de puntos de venta por sucursal
- Agregar indicador de POS activos conectados al SIAT

### 8. Página SIAT — Checklist más visual
**Archivo:** [`siat-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/siat-page.tsx)

- El checklist usa emoji `✓` en lugar de iconos reales
- Los botones de operaciones granulares son pequeños y sin jerarquía
- **Mejora:** Usar iconos `lucide-react` reales con color, agregar progress visual

### 9. Clientes y Productos — Tablas más ricas
**Archivos:** [`customers-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/customers-page.tsx), [`products-page.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/products-page.tsx)

- Ambas páginas son solo lectura pero les falta paginación visual
- **Mejora:** Agregar avatares/iconos para clientes, badges de tipo de documento
- Productos: agregar badge visual para la actividad económica

### 10. Header — Breadcrumb y POS selector mejorados
**Archivo:** [`header.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/components/layout/header.tsx)

- El selector de POS en el header muestra `pos.description` ✅ 
- **Mejora:** Agregar indicador visual de si el POS seleccionado tiene CUIS vigente
- Mejorar el menú de notificaciones con animación de entrada

---

## 🟢 Prioridad Baja — Polish

### 11. Operaciones (Lotes/Compras) — Mostrar nombres de POS en lugar de UUIDs
**Archivo:** [`operation-pages.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/pages/operation-pages.tsx)

- Los IDs de facturas se muestran como UUIDs crudos en el Alert de contingencia
- **Mejora:** Mostrar N° de factura + cliente en lugar de UUIDs cuando sea posible

### 12. PageHeader y FormSection — Animaciones de entrada
**Archivo:** [`page-parts.tsx`](file:///c:/Users/ramit/Documents/projects/supay/packages/dashboard/src/components/shared/page-parts.tsx)

- Agregar micro-animaciones de entrada (fadeIn + slideUp)

---

## Archivos a Modificar (en orden de ejecución)

| # | Archivo | Cambios |
|---|---------|---------|
| 1 | `styles.css` | Fix typo, agregar tokens, animaciones globales |
| 2 | `page-parts.tsx` | Animaciones de entrada, glassmorphism |
| 3 | `dashboard-home-page.tsx` | Rediseño StatCards premium, actividad reciente |
| 4 | `invoices-page.tsx` | Columna POS con nombre, tabla premium |
| 5 | `invoice-detail-sheet.tsx` | Nombre sucursal, tabs animados, CUF mejorado |
| 6 | `points-of-sale-page.tsx` | Tarjetas ricas, semáforo animado |
| 7 | `branches-page.tsx` | Badge conteo POS, indicadores |
| 8 | `siat-page.tsx` | Iconos reales, progress visual |
| 9 | `invoice-new-page.tsx` | Tabla items limpia, sticky bar glassmorphism |
| 10 | `header.tsx` | POS health indicator, notif animation |
| 11 | `customers-page.tsx` | Avatares, badges documento |
| 12 | `products-page.tsx` | Badges actividad, visual polish |
| 13 | `operation-pages.tsx` | IDs → nombres legibles |

> [!IMPORTANT]
> No se toca el backend Go, ni se modifican los tipos/interfaces, ni la capa host. Todos los cambios son puramente de presentación y UX dentro del dashboard package existente.

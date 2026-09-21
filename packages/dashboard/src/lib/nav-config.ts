import {
  AlertTriangle,
  Boxes,
  Building2,
  FileText,
  KeyRound,
  Layers,
  Package,
  Plug,
  ShoppingCart,
  Store,
  Users,
} from "lucide-react"
import type { LucideIcon } from "lucide-react"

export interface NavItem {
  label: string
  to: string
  icon: LucideIcon
}

export interface NavSection {
  label: string
  /** Los ítems son rutas de primer nivel: el breadcrumb muestra solo el ítem. */
  flat: boolean
  items: NavItem[]
}

export const NAV_SECTIONS: NavSection[] = [
  {
    label: "Facturación",
    flat: true,
    items: [
      { label: "Facturas", to: "/invoices", icon: FileText },
      { label: "Clientes", to: "/customers", icon: Users },
      { label: "Productos", to: "/products", icon: Package },
    ],
  },
  {
    label: "Mi empresa",
    flat: false,
    items: [
      { label: "Datos fiscales", to: "/company", icon: Building2 },
      { label: "Sucursales", to: "/company/branches", icon: Building2 },
      { label: "Puntos de venta", to: "/company/points-of-sale", icon: Store },
      { label: "Actividad económica", to: "/company/sectors", icon: Layers },
      { label: "Conexión SIAT", to: "/company/siat", icon: Plug },
      { label: "API keys", to: "/company/api-keys", icon: KeyRound },
    ],
  },
  {
    label: "Operación",
    flat: false,
    items: [
      { label: "Contingencia", to: "/operation/contingencia", icon: AlertTriangle },
      { label: "Lotes", to: "/operation/lotes", icon: Boxes },
      { label: "Compras", to: "/operation/compras", icon: ShoppingCart },
    ],
  },
]

export interface ActiveNav {
  parent?: string
  child?: string
}

const CHILD_OVERRIDES: Record<string, string> = {
  "/invoices/new": "Nueva factura",
}

export function mergeNavSections(
  defaults: NavSection[],
  extensions?: NavSection[] | ((defaults: NavSection[]) => NavSection[])
): NavSection[] {
  if (!extensions) return defaults
  if (typeof extensions === "function") return extensions(defaults)
  return [...defaults, ...extensions]
}

export function findActiveNav(pathname: string, sections: NavSection[] = NAV_SECTIONS): ActiveNav | null {
  if (pathname === "/setup") {
    return { child: "Configuración inicial" }
  }

  for (const section of sections) {
    for (const item of section.items) {
      if (pathname === item.to) {
        if (section.flat) return { child: item.label }
        return { parent: section.label, child: item.label }
      }
    }
  }

  const override = CHILD_OVERRIDES[pathname]
  if (override && pathname.startsWith("/invoices")) {
    return { parent: "Facturas", child: override }
  }

  for (const section of sections) {
    if (!section.flat && pathname.startsWith(section.label === "Mi empresa" ? "/company" : "/operation")) {
      const item = section.items.find((i) => pathname.startsWith(i.to))
      return { parent: section.label, child: item?.label ?? "" }
    }
  }

  return null
}

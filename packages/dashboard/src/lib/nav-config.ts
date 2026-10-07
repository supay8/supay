import {
  AlertTriangle,
  Boxes,
  Building2,
  FileText,
  KeyRound,
  Layers,
  LayoutDashboard,
  Package,
  Plug,
  ShieldCheck,
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
    label: "Monitoreo",
    flat: true,
    items: [
      { label: "Panel", to: "/dashboard", icon: LayoutDashboard },
      { label: "Facturas", to: "/invoices", icon: FileText },
    ],
  },
  {
    label: "Catálogos",
    flat: true,
    items: [
      { label: "Receptores", to: "/customers", icon: Users },
      { label: "Catálogo SIN", to: "/products", icon: Package },
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
    ],
  },
  {
    label: "Seguridad",
    flat: false,
    items: [
      { label: "Certificado digital", to: "/company/certificates", icon: ShieldCheck },
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
  parentTo?: string
  child?: string
  childTo?: string
}

const CHILD_OVERRIDES: Record<string, string> = {}

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
    return { parent: "Mi empresa", parentTo: "/company", child: "Configuración inicial", childTo: "/setup" }
  }

  for (const section of sections) {
    for (const item of section.items) {
      if (pathname === item.to) {
        if (section.flat) return { child: item.label, childTo: item.to }
        return { parent: section.label, parentTo: item.to, child: item.label, childTo: item.to }
      }
    }
  }

  const override = CHILD_OVERRIDES[pathname]
  if (override) {
    return { parent: "Monitoreo", parentTo: "/dashboard", child: override, childTo: pathname }
  }

  for (const section of sections) {
    if (section.flat) continue
    const item = section.items.find((i) => pathname.startsWith(i.to))
    if (item) {
      return { parent: section.label, parentTo: item.to, child: item.label, childTo: item.to }
    }
  }

  return null
}

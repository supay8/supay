import * as React from "react"
import { BrowserRouter } from "react-router-dom"
import type { QueryClient } from "@tanstack/react-query"
import type { DashboardHost } from "./host"
import type { DashboardConfig } from "./config"
import type { NavSection } from "./lib/nav-config"
import { NAV_SECTIONS, mergeNavSections } from "./lib/nav-config"
import { DashboardHostProvider } from "./host-context"
import { AuthProvider } from "./auth-context"
import { DashboardExtensionsProvider } from "./dashboard-context"
import { DashboardProviders } from "./providers"
import { DashboardRoutes, type DashboardRoute } from "./routing"

export interface SupayDashboardProps {
  mode?: "self-hosted" | "cloud"
  host: DashboardHost
  config?: DashboardConfig
  /** Rutas extra para Cloud (ej /billing, /usage) */
  routes?: DashboardRoute[]
  /** Secciones de navegación extra o función de merge */
  navSections?: NavSection[] | ((defaults: NavSection[]) => NavSection[])
  queryClient?: QueryClient
  capabilities?: Record<string, unknown>
  slots?: {
    headerActions?: React.ReactNode
    sidebarFooter?: React.ReactNode
  }
  /** Si true, no crea BrowserRouter interno (útil si host ya provee router) */
  disableRouter?: boolean
}

function SupayDashboardInner({
  routes,
}: {
  routes?: DashboardRoute[]
}) {
  return <DashboardRoutes extraRoutes={routes} />
}

export function SupayDashboard({
  mode = "self-hosted",
  host,
  config,
  routes,
  navSections,
  queryClient,
  capabilities,
  slots,
  disableRouter,
}: SupayDashboardProps) {
  const mergedNav = mergeNavSections(NAV_SECTIONS, navSections)
  const dashboardConfig: DashboardConfig = {
    ...config,
    mode: config?.mode ?? mode,
  }

  const content = (
    <DashboardHostProvider host={host}>
      <AuthProvider>
      <DashboardExtensionsProvider
        value={{
          config: dashboardConfig,
          navSections: mergedNav,
          capabilities,
          slots,
        }}
      >
        <DashboardProviders queryClient={queryClient}>
          {disableRouter ? (
            <SupayDashboardInner routes={routes} />
          ) : (
            <BrowserRouter>
              <SupayDashboardInner routes={routes} />
            </BrowserRouter>
          )}
        </DashboardProviders>
      </DashboardExtensionsProvider>
      </AuthProvider>
    </DashboardHostProvider>
  )

  return content
}

export type { DashboardRoute, NavSection }

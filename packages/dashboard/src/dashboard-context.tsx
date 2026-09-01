import * as React from "react"
import type { DashboardConfig } from "./config"
import type { NavSection } from "./lib/nav-config"

export interface DashboardExtensions {
  config: DashboardConfig
  navSections: NavSection[]
  capabilities?: Record<string, unknown>
  slots?: {
    headerActions?: React.ReactNode
    sidebarFooter?: React.ReactNode
  }
}

const DashboardExtensionsContext = React.createContext<DashboardExtensions | null>(null)

export function DashboardExtensionsProvider({
  value,
  children,
}: {
  value: DashboardExtensions
  children: React.ReactNode
}) {
  return (
    <DashboardExtensionsContext.Provider value={value}>
      {children}
    </DashboardExtensionsContext.Provider>
  )
}

export function useDashboardExtensions(): DashboardExtensions {
  const ctx = React.useContext(DashboardExtensionsContext)
  if (!ctx) throw new Error("useDashboardExtensions must be used within SupayDashboard")
  return ctx
}

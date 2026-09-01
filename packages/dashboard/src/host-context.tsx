import * as React from "react"
import type { DashboardHost } from "./host"

const DashboardHostContext = React.createContext<DashboardHost | null>(null)

export function DashboardHostProvider({
  host,
  children,
}: {
  host: DashboardHost
  children: React.ReactNode
}) {
  return (
    <DashboardHostContext.Provider value={host}>
      {children}
    </DashboardHostContext.Provider>
  )
}

export function useDashboardHost(): DashboardHost {
  const host = React.useContext(DashboardHostContext)
  if (!host) {
    throw new Error("useDashboardHost must be used within DashboardHostProvider")
  }
  return host
}

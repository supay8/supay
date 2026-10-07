import * as React from "react"
import { useQueryClient } from "@tanstack/react-query"

/**
 * Limpia el caché de React Query al cambiar de empresa o cerrar sesión.
 * Sin esto, los listados con key sin companyId muestran datos ajenos.
 * Se monta dentro de DashboardProviders (tiene acceso al QueryClient).
 */
export function CompanyScopeSync() {
  const queryClient = useQueryClient()
  React.useEffect(() => {
    function onChange() {
      queryClient.clear()
    }
    window.addEventListener("supay:company-changed", onChange)
    return () => window.removeEventListener("supay:company-changed", onChange)
  }, [queryClient])
  return null
}

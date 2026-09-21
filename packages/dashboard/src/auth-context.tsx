import * as React from "react"
import type { AuthUser, CreateMyCompanyInput, UserCompany } from "./host"
import { useDashboardHost } from "./host-context"

const ACTIVE_COMPANY_STORAGE_KEY = "supay_company_id"

function getStoredActiveCompanyId(): string {
  if (typeof window !== "undefined") {
    return localStorage.getItem(ACTIVE_COMPANY_STORAGE_KEY) ?? ""
  }
  return ""
}

function setStoredActiveCompanyId(id: string): void {
  if (typeof window !== "undefined") {
    if (id) {
      localStorage.setItem(ACTIVE_COMPANY_STORAGE_KEY, id)
    } else {
      localStorage.removeItem(ACTIVE_COMPANY_STORAGE_KEY)
    }
  }
}

interface AuthContextValue {
  user: AuthUser | null
  isLoading: boolean
  isAuthenticated: boolean
  companies: UserCompany[]
  companiesLoading: boolean
  activeCompany: UserCompany | null
  hasCompany: boolean
  signup: (name: string, email: string, password: string) => Promise<void>
  login: (email: string, password: string) => Promise<void>
  logout: () => void
  refreshCompanies: () => Promise<void>
  createCompany: (payload: CreateMyCompanyInput) => Promise<UserCompany>
  switchCompany: (companyId: string) => void
}

const AuthContext = React.createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const host = useDashboardHost()
  const [user, setUser] = React.useState<AuthUser | null>(null)
  const [isLoading, setIsLoading] = React.useState(true)
  const [companies, setCompanies] = React.useState<UserCompany[]>([])
  const [companiesLoading, setCompaniesLoading] = React.useState(false)
  const [activeCompanyId, setActiveCompanyId] = React.useState<string | null>(null)

  const loadCompanies = React.useCallback(async (): Promise<UserCompany[]> => {
    setCompaniesLoading(true)
    try {
      const list = await host.listMyCompanies()
      setCompanies(list)
      // Auto-selección: respeta la guardada si sigue existiendo, si no la primera
      const stored = getStoredActiveCompanyId()
      const match = stored
        ? list.find((uc) => uc.company.id === stored) ?? null
        : null
      const picked = match ?? list[0] ?? null
      setActiveCompanyId(picked ? picked.company.id : null)
      setStoredActiveCompanyId(picked ? picked.company.id : "")
      return list
    } finally {
      setCompaniesLoading(false)
    }
  }, [host])

  // Restaura la sesión si hay un token guardado
  React.useEffect(() => {
    let cancelled = false
    setIsLoading(true)
    host
      .getCurrentUser()
      .then(async (current) => {
        if (cancelled) return
        setUser(current)
        await loadCompanies()
      })
      .catch(() => {
        // Token ausente o inválido: queda anónimo y limpia el token
        if (!cancelled) {
          host.logout()
          setUser(null)
          setCompanies([])
          setActiveCompanyId(null)
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [host, loadCompanies])

  const signup = React.useCallback(
    async (name: string, email: string, password: string) => {
      const result = await host.signup(name, email, password)
      setUser(result.user)
      await loadCompanies()
    },
    [host, loadCompanies]
  )

  const login = React.useCallback(
    async (email: string, password: string) => {
      const result = await host.login(email, password)
      setUser(result.user)
      await loadCompanies()
    },
    [host, loadCompanies]
  )

  const logout = React.useCallback(() => {
    host.logout()
    host.setActiveCompanyId?.("")
    setUser(null)
    setCompanies([])
    setActiveCompanyId(null)
    setStoredActiveCompanyId("")
  }, [host])

  const refreshCompanies = React.useCallback(async () => {
    await loadCompanies()
  }, [loadCompanies])

  const createCompany = React.useCallback(
    async (payload: CreateMyCompanyInput): Promise<UserCompany> => {
      const company = await host.createMyCompany(payload)
      const list = await loadCompanies()
      // Fija como activa la recién creada (auto-selección habría elegido otra
      // si el usuario ya tenía empresas)
      const created = list.find((uc) => uc.company.id === company.id) ?? null
      if (!created) {
        throw new Error("La empresa se creó pero no se pudo recuperar.")
      }
      setActiveCompanyId(created.company.id)
      setStoredActiveCompanyId(created.company.id)
      return created
    },
    [host, loadCompanies]
  )

  const activeCompany =
    companies.find((uc) => uc.company.id === activeCompanyId) ?? null

  // Propaga la empresa activa al host para el header X-Company-ID
  React.useEffect(() => {
    host.setActiveCompanyId?.(activeCompanyId ?? "")
  }, [host, activeCompanyId])

  const switchCompany = React.useCallback(
    (companyId: string) => {
      setActiveCompanyId(companyId)
      setStoredActiveCompanyId(companyId)
      host.setActiveCompanyId?.(companyId)
    },
    [host]
  )

  const value = React.useMemo(
    () => ({
      user,
      isLoading,
      isAuthenticated: user !== null,
      companies,
      companiesLoading,
      activeCompany,
      hasCompany: activeCompany !== null,
      signup,
      login,
      logout,
      refreshCompanies,
      createCompany,
      switchCompany,
    }),
    [
      user,
      isLoading,
      companies,
      companiesLoading,
      activeCompany,
      signup,
      login,
      logout,
      refreshCompanies,
      createCompany,
      switchCompany,
    ]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = React.useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return ctx
}

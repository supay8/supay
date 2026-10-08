import { useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { Building2, Check } from "lucide-react"
import { LogoSupay } from "../components/logo"
import { useAuth } from "../auth-context"
import { HostError } from "../host"
import { formatCompanyTitle } from "../lib/display-names"
import { NIT_ERROR, isValidNit } from "../lib/validation"
import { Button } from "../components/ui/button"

function roleLabel(role: string): string {
  const r = role.trim().toLowerCase()
  if (r === "owner") return "Propietario"
  if (r === "admin") return "Administrador"
  if (r === "member" || r === "miembro") return "Miembro"
  return role
}

export function CompaniesPage() {
  const { companies, companiesLoading, activeCompany, switchCompany, createCompany, refreshCompanies } = useAuth()
  const navigate = useNavigate()
  const [nit, setNit] = useState("")
  const [businessName, setBusinessName] = useState("")
  const [municipio, setMunicipio] = useState("")
  const [direccion, setDireccion] = useState("")
  const [telefono, setTelefono] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const nitValid = isValidNit(nit)
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    if (!nitValid) {
      setError(NIT_ERROR)
      return
    }
    setIsLoading(true)
    try {
      await createCompany({
        nit: nit.trim(),
        business_name: businessName.trim(),
        municipio: municipio.trim() || undefined,
        direccion: direccion.trim() || undefined,
        telefono: telefono.trim() || undefined,
      })
      navigate("/setup", { replace: true })
    } catch (err) {
      if (err instanceof HostError && err.status === 409) {
        setError("Ya existe una empresa registrada con este NIT.")
      } else if (err instanceof Error) {
        setError(err.message || "No se pudo crear la empresa. Intenta de nuevo.")
      } else {
        setError("No se pudo crear la empresa. Intenta de nuevo.")
      }
    } finally {
      setIsLoading(false)
    }
  }

  function choose(id: string) {
    switchCompany(id)
    navigate("/dashboard", { replace: true })
  }

  return (
    <div className="bg-background relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-10">
      <div className="relative z-10 w-full max-w-md">
        <div className="mb-6 flex flex-col items-center">
          <LogoSupay size={50} className="shrink-0 text-primary" />
          <h1 className="text-foreground text-xl font-medium tracking-tight">
            {companies.length > 0 ? "Elegí tu empresa" : "Crea tu empresa"}
          </h1>
          <p className="text-muted-foreground mt-1 font-mono text-sm">
            {companies.length > 0
              ? "Una cuenta, varias empresas. Cambiá cuando quieras desde el encabezado."
              : "Necesitas un tenant para empezar a facturar"}
          </p>
        </div>

        {companiesLoading ? (
          <p className="text-muted-foreground text-center text-sm">Cargando empresas…</p>
        ) : companies.length > 0 ? (
          <div className="mb-4 flex flex-col gap-2">
            {companies.map(({ company, role }) => {
              const isActive = activeCompany?.company.id === company.id
              return (
                <button
                  key={company.id}
                  type="button"
                  onClick={() => choose(company.id)}
                  className="bg-card border-border flex items-center gap-3 rounded-xl border p-4 text-left shadow-sm transition-colors hover:bg-muted/40"
                >
                  <Building2 className="text-muted-foreground size-5 shrink-0" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">
                      {formatCompanyTitle(company.business_name, company.nit)}
                    </span>
                    <span className="text-muted-foreground text-xs">
                      Rol {roleLabel(role)} · {company.ambiente === "PRODUCCION" ? "Producción" : "Piloto"}
                    </span>
                  </span>
                  {isActive && <Check className="size-4 shrink-0 text-success" />}
                </button>
              )
            })}
            <Button variant="ghost" size="sm" onClick={() => refreshCompanies()}>
              Recargar lista
            </Button>
          </div>
        ) : null}

        <div className="bg-card border-border rounded-xl border p-6 shadow-2xl shadow-black/5 dark:shadow-black/40">
          <p className="mb-4 text-sm font-medium">
            {companies.length > 0 ? "O creá una nueva empresa" : "Datos de la empresa"}
          </p>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <p role="alert" className="text-destructive font-mono text-sm">
                {error}
              </p>
            )}
            <div className="space-y-1.5">
              <label htmlFor="company-nit" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                NIT *
              </label>
              <input
                id="company-nit"
                type="text"
                required
                value={nit}
                onChange={(event) => setNit(event.target.value)}
                placeholder="123456789"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label htmlFor="company-name" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Razón social *
              </label>
              <input
                id="company-name"
                type="text"
                required
                value={businessName}
                onChange={(event) => setBusinessName(event.target.value)}
                placeholder="Mi Empresa SRL"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label htmlFor="company-municipio" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Municipio
              </label>
              <input
                id="company-municipio"
                type="text"
                value={municipio}
                onChange={(event) => setMunicipio(event.target.value)}
                placeholder="La Paz"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label htmlFor="company-direccion" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Dirección
              </label>
              <input
                id="company-direccion"
                type="text"
                value={direccion}
                onChange={(event) => setDireccion(event.target.value)}
                placeholder="Av. Principal #123"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label htmlFor="company-telefono" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Teléfono
              </label>
              <input
                id="company-telefono"
                type="text"
                value={telefono}
                onChange={(event) => setTelefono(event.target.value)}
                placeholder="2 123456"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>
            <button
              type="submit"
              disabled={isLoading}
              className="bg-primary text-primary-foreground hover:bg-primary-hover mt-2 flex h-10 w-full cursor-pointer items-center justify-center rounded-md text-sm font-medium shadow-sm transition-all active:scale-[0.99] disabled:pointer-events-none disabled:opacity-50"
            >
              {isLoading ? (
                <span className="border-primary-foreground inline-block size-4 animate-spin rounded-full border-2 border-t-transparent" />
              ) : (
                "Crear empresa y continuar"
              )}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}

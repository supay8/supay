import { LogoSupay } from "../components/logo"
import { useState, type FormEvent } from "react"
import { Link, useNavigate } from "react-router-dom"
import { useAuth } from "../auth-context"
import { HostError } from "../host"

export function SignupPage() {
  const { signup } = useAuth()
  const navigate = useNavigate()
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setIsLoading(true)
    try {
      await signup(name.trim(), email.trim(), password)
      navigate("/", { replace: true })
    } catch (err) {
      if (err instanceof HostError && err.status === 409) {
        setError("Ya existe un usuario registrado con este correo.")
      } else if (err instanceof Error) {
        setError(err.message || "No se pudo crear la cuenta. Intenta de nuevo.")
      } else {
        setError("No se pudo crear la cuenta. Intenta de nuevo.")
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="bg-background relative flex min-h-screen items-center justify-center overflow-hidden px-4">
      <div
        className="pointer-events-none absolute inset-0 opacity-[0.03] dark:opacity-[0.05]"
        style={{
          backgroundImage: "radial-gradient(var(--text) 1px, transparent 1px)",
          backgroundSize: "24px 24px",
        }}
      />

      <div className="relative z-10 w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center">
           <LogoSupay size={50} className="shrink-0 text-primary"  />
          <h1 className="text-foreground text-xl font-medium tracking-tight">
            Crear cuenta en Supay
          </h1>
          <p className="text-muted-foreground mt-1 font-mono text-sm">
            infraestructura.v1
          </p>
        </div>

        <div className="bg-card border-border rounded-xl border p-6 shadow-2xl shadow-black/5 dark:shadow-black/40">
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <p role="alert" className="text-destructive font-mono text-sm">
                {error}
              </p>
            )}
            <div className="space-y-1.5">
              <label htmlFor="signup-name" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Nombre completo
              </label>
              <input
                id="signup-name"
                type="text"
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Ada Lovelace"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>

            <div className="space-y-1.5">
              <label htmlFor="signup-email" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Correo electrónico
              </label>
              <input
                id="signup-email"
                type="email"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="ingeniero@empresa.com"
                className="bg-background border-input text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:ring-primary h-10 w-full rounded-md border px-3 font-mono text-sm transition-all focus:ring-1 focus:outline-none"
              />
            </div>

            <div className="space-y-1.5">
              <label htmlFor="signup-password" className="text-muted-foreground block font-mono text-xs uppercase tracking-wider">
                Contraseña
              </label>
              <input
                id="signup-password"
                type="password"
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="••••••••••••"
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
                "Registrarse"
              )}
            </button>
          </form>
        </div>

        <p className="text-muted-foreground mt-6 text-center font-mono text-xs">
          ¿Ya tienes una cuenta?{" "}
          <Link to="/login" className="text-foreground underline-offset-4 hover:underline">
            Iniciar sesión
          </Link>
        </p>
      </div>
    </div>
  )
}
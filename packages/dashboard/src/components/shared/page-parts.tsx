import type { ReactNode } from "react"
import { RefreshCw } from "lucide-react"
import { ApiError } from "../../host"
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../components/ui/alert"
import { Button } from "../../components/ui/button"

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string
  description?: string
  actions?: ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex flex-col gap-0.5">
        <h1 className="text-lg font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="text-muted-foreground text-sm">{description}</p>
        )}
      </div>
      {actions}
    </div>
  )
}

export function QueryErrorState({
  error,
  onRetry,
}: {
  error: unknown
  onRetry: () => void
}) {
  return (
    <Alert variant="destructive">
      <AlertTitle>No se pudo cargar la información</AlertTitle>
      <AlertDescription>
        {error instanceof ApiError ? error.message : String(error)}
      </AlertDescription>
      <Button variant="outline" size="sm" onClick={onRetry}>
        <RefreshCw data-icon="inline-start" />
        Reintentar
      </Button>
    </Alert>
  )
}

export function FormSection({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <section className="rounded-lg border p-5">
      <div className="mb-4 flex flex-col gap-0.5">
        <p className="text-sm font-medium">{title}</p>
        {description && (
          <p className="text-muted-foreground text-xs">{description}</p>
        )}
      </div>
      {children}
    </section>
  )
}

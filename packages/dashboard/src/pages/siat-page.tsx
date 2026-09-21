import { useMutation, useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { useDashboardHost } from "../host-context"
import { useAuth } from "../auth-context"
import { toast } from "sonner"

import { ApiError } from "../host"
import { formatDateTime } from "../lib/format"
import { FormSection, PageHeader, QueryErrorState } from "../components/shared/page-parts"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../components/ui/tooltip"
import { Button } from "../components/ui/button"
import { Skeleton } from "../components/ui/skeleton"

function Tech({ term }: { term: string }) {
  const map: Record<string, string> = {
    CUIS: "Código Único de Inicio de Vigencia que el SIAT entrega por punto de venta. Se renueva periódicamente.",
    CUFD: "Código Único de Facturación Diaria: firma con la que se emiten las facturas del día.",
  }
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span className="underline decoration-dotted underline-offset-2" />
        }
      >
        ({term})
      </TooltipTrigger>
      <TooltipContent className="max-w-64">{map[term]}</TooltipContent>
    </Tooltip>
  )
}

export function SiatConnectionPage() {
  const host = useDashboardHost()
  const navigate = useNavigate()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const company = activeCompany?.company ?? null
  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  const checkMutation = useMutation({
    mutationFn: async () => {
      const first = posQuery.data?.items[0]
      if (!first) throw new Error("Sin puntos de venta para verificar")
      return host.setupPointOfSale(first.id)
    },
    onSuccess: () => {
      toast.success("Conexión verificada")
      posQuery.refetch()
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError
          ? `${error.message} Podés reintentar con seguridad.`
          : "No se pudo verificar"
      ),
  })

  const readinessQuery = useQuery({
    queryKey: ["catalog-readiness", companyId, posQuery.data?.items[0]?.id],
    queryFn: () =>
      host.getCatalogReadiness
        ? host.getCatalogReadiness(companyId, posQuery.data?.items[0]?.id)
        : Promise.resolve(null),
    enabled: companyId !== "" && (posQuery.data?.items.length ?? 0) > 0,
    staleTime: 60_000,
  })

  function opsFor(posId: string) {
    return {
      cuis: () => host.requestCuis?.(posId) ?? Promise.reject(new Error("No soportado")),
      cufd: () => host.requestCufd?.(posId) ?? Promise.reject(new Error("No soportado")),
      sync: () => host.sincronizar?.(posId) ?? Promise.reject(new Error("No soportado")),
    }
  }

  const granularMutation = useMutation({
    mutationFn: async ({ kind, posId }: { kind: "cuis" | "cufd" | "sync"; posId: string }) => {
      const ops = opsFor(posId)
      return ops[kind]()
    },
    onSuccess: (res) => {
      toast.success(res.reception_code ? `OK: ${res.reception_code}` : "Operación completada")
      posQuery.refetch()
      readinessQuery.refetch()
    },
    onError: (error) =>
      toast.error(error instanceof ApiError ? error.message : "Falló la operación SIAT"),
  })

  const posList = posQuery.data?.items ?? []
  const connected = posList.filter((p) => p.cuis).length
  const lastCuis = posList
    .map((p) => p.cuis_created_at)
    .filter(Boolean)
    .sort()
    .at(-1)

  return (
    <div className="mx-auto flex max-w-xl flex-col gap-6">
      <PageHeader
        title="Conexión SIAT"
        description="Estado del vínculo entre tu empresa y Impuestos."
        actions={
          <Button
            size="sm"
            variant="outline"
            disabled={checkMutation.isPending}
            onClick={() => checkMutation.mutate()}
          >
            Volver a verificar
          </Button>
        }
      />

      {posQuery.isError && (
        <QueryErrorState
          error={posQuery.error}
          onRetry={() => posQuery.refetch()}
        />
      )}

      <FormSection title="Checklist de conexión">
        {readinessQuery.data != null && (
          <p className="mb-3 text-xs">
            Readiness:{" "}
            {readinessQuery.data.ready ? (
              <span className="text-success font-medium">listo para emitir</span>
            ) : (
              <span className="font-medium text-warning">
                falta: {(readinessQuery.data.missing ?? []).join(", ") || "ver detalle"}
              </span>
            )}
          </p>
        )}
        {posQuery.isPending && (
          <div className="flex flex-col gap-3">
            {[...Array(4)].map((_, i) => (
              <Skeleton key={i} className="h-5 w-full" />
            ))}
          </div>
        )}
        {company && (
          <ul className="flex flex-col gap-3 text-sm">
            <li className="flex items-center justify-between gap-3">
              <span>
                Registro de la empresa{" "}
                <span className="text-muted-foreground text-xs">
                  · {formatDateTime(company.created_at)}
                </span>
              </span>
              <span className="text-success text-xs font-medium">✓</span>
            </li>
            <li className="flex items-center justify-between gap-3">
              <span>
                Código de autorización diario <Tech term="CUIS" />
                {lastCuis && (
                  <span className="text-muted-foreground text-xs">
                    {" "}
                    · {formatDateTime(lastCuis)}
                  </span>
                )}
              </span>
              {connected > 0 ? (
                <span className="text-success text-xs font-medium">
                  vigente
                </span>
              ) : (
                <span className="font-medium text-warning text-xs">
                  sin datos
                </span>
              )}
            </li>
            <li className="flex items-center justify-between gap-3">
              <span>Puntos de venta conectados</span>
              <span className="font-medium tabular-nums">
                {connected}/{posList.length}
              </span>
            </li>
            <li className="flex items-center justify-between gap-3">
              <span>Código de firma del día <Tech term="CUFD" /></span>
              <span className={connected > 0 ? "text-success text-xs font-medium" : "text-muted-foreground text-xs"}>
                {connected > 0 ? "por punto de venta" : "—"}
              </span>
            </li>
          </ul>
        )}
      </FormSection>

      <FormSection
        title="Operaciones por etapa"
        description="Reintentos granulares sin repetir el setup completo."
      >
        <div className="flex flex-col gap-2">
          {(posQuery.data?.items ?? []).slice(0, 3).map((p) => (
            <div key={p.id} className="flex flex-wrap items-center gap-2 text-sm">
              <span className="text-muted-foreground min-w-40 flex-1">{p.description}</span>
              <Button
                size="sm"
                variant="outline"
                disabled={granularMutation.isPending}
                onClick={() => granularMutation.mutate({ kind: "cuis", posId: p.id })}
              >
                CUIS
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={granularMutation.isPending}
                onClick={() => granularMutation.mutate({ kind: "cufd", posId: p.id })}
              >
                CUFD
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={granularMutation.isPending}
                onClick={() => granularMutation.mutate({ kind: "sync", posId: p.id })}
              >
                Sincronizar
              </Button>
            </div>
          ))}
          {(posQuery.data?.items ?? []).length === 0 && (
            <p className="text-muted-foreground text-sm">Sin puntos de venta.</p>
          )}
        </div>
      </FormSection>

      <FormSection
        title="Certificado digital"
        description="La firma .p12 se gestiona en Seguridad (un activo por NIT)."
      >
        <div className="flex items-center justify-between gap-4 text-sm">
          <span className="text-muted-foreground">
            Subí o renová el certificado sin salir del flujo SIAT.
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate("/company/certificates")}
          >
            Gestionar certificado
          </Button>
        </div>
      </FormSection>

      <p className="text-muted-foreground text-xs">
        La verificación usa la misma conexión inicial: es idempotente y nunca
        duplica registros.
      </p>
    </div>
  )
}

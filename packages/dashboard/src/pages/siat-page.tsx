import { useMutation, useQuery } from "@tanstack/react-query"
import { useDashboardHost } from "@/host-context"
import { toast } from "sonner"

import { ApiError } from "@/host"
import { PLACEHOLDER_COMPANY_ID } from "@/lib/invoice-status"
import { formatDateTime } from "@/lib/format"
import { FormSection, PageHeader, QueryErrorState } from "@/components/shared/page-parts"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

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
  const companyQuery = useQuery({
    queryKey: ["company", PLACEHOLDER_COMPANY_ID],
    queryFn: () => host.getCompany(PLACEHOLDER_COMPANY_ID),
    retry: false,
  })
  const posQuery = useQuery({
    queryKey: ["point-of-sale"],
    queryFn: () => host.listPointsOfSale(PLACEHOLDER_COMPANY_ID),
    retry: false,
  })

  const checkMutation = useMutation({
    mutationFn: () =>
      host.setupCompany(PLACEHOLDER_COMPANY_ID, {}),
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

  const company = companyQuery.data
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

      {companyQuery.isError && (
        <QueryErrorState
          error={companyQuery.error}
          onRetry={() => companyQuery.refetch()}
        />
      )}

      <FormSection title="Checklist de conexión">
        {(companyQuery.isPending || posQuery.isPending) && (
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

      <p className="text-muted-foreground text-xs">
        La verificación usa la misma conexión inicial: es idempotente y nunca
        duplica registros.
      </p>
    </div>
  )
}

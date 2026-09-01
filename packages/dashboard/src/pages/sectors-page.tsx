import { useMemo, useState } from "react"
import { useDashboardHost } from "@/host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { ApiError } from "@/host"
import { PLACEHOLDER_COMPANY_ID } from "@/lib/invoice-status"
import { PageHeader, QueryErrorState } from "@/components/shared/page-parts"
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

export function SectorsPage() {
  const host = useDashboardHost()
  const queryClient = useQueryClient()

  const companyQuery = useQuery({
    queryKey: ["company", PLACEHOLDER_COMPANY_ID],
    queryFn: () => host.getCompany(PLACEHOLDER_COMPANY_ID),
    retry: false,
  })
  const sectoresQuery = useQuery({
    queryKey: ["sectores"],
    queryFn: () => host.listSectores(),
    staleTime: 5 * 60_000,
  })

  const [pendingSector, setPendingSector] = useState<number | null>(null)

  const company = companyQuery.data
  const sectores = useMemo(
    () => sectoresQuery.data ?? [],
    [sectoresQuery.data]
  )
  const habilitados = useMemo(() => sectores.filter((s) => s.habilitado), [sectores])
  const currentCodigo = company?.codigo_actividad
    ? Number(company.codigo_actividad)
    : null

  const selectMutation = useMutation({
    mutationKey: ["set-sector"],
    mutationFn: (codigo: number) =>
      host.updateCompany(PLACEHOLDER_COMPANY_ID, {
        codigo_actividad: String(codigo),
      }),
    onSuccess: () => {
      setPendingSector(null)
      queryClient.invalidateQueries({ queryKey: ["company"] })
      toast.success("Actividad actualizada para próximas facturas")
    },
    onError: (error) => {
      setPendingSector(null)
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo actualizar"
      )
    },
  })

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <PageHeader
        title="Actividad económica"
        description="Define qué campos adicionales te pide el sistema al emitir."
      />

      {companyQuery.isError && (
        <QueryErrorState
          error={companyQuery.error}
          onRetry={() => companyQuery.refetch()}
        />
      )}

      <section className="rounded-lg border p-5">
        <p className="text-muted-foreground mb-3 text-xs">
          Tu actividad actual — aplica a las próximas facturas. Podés cambiar el
          sector puntualmente al emitir.
        </p>
        {sectoresQuery.isPending && (
          <div className="grid gap-2 sm:grid-cols-3">
            {[...Array(6)].map((_, i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        )}
        <div className="grid gap-2 sm:grid-cols-3">
          {habilitados.slice(0, 12).map((sector) => {
            const selected =
              currentCodigo === sector.codigo ||
              (currentCodigo === null && sector === habilitados[0])
            return (
              <button
                key={sector.codigo}
                type="button"
                onClick={() => {
                  setPendingSector(sector.codigo)
                  selectMutation.mutate(sector.codigo)
                }}
                className={cn(
                  "rounded-md border px-3 py-2 text-left text-xs font-medium transition-colors",
                  selected ? "border-ring ring-ring ring-1" : "hover:bg-muted/50",
                  pendingSector === sector.codigo && "opacity-60"
                )}
              >
                {sector.label}
              </button>
            )
          })}
        </div>
        {habilitados.length > 12 && (
          <p className="text-muted-foreground mt-3 text-[11px]">
            +{habilitados.length - 12} actividades más disponibles al emitir una
            factura.
          </p>
        )}
      </section>

      <section className="rounded-lg border">
        <div className="border-b px-5 py-4">
          <p className="text-sm font-medium">Catálogo completo</p>
          <p className="text-muted-foreground text-xs">
            Fuente oficial del SIAT, sincronizada vía Supay.
          </p>
        </div>
        {sectores.length > 0 && (
          <Accordion className="px-5 py-2">
            {sectores.map((sector) => (
              <AccordionItem key={sector.codigo} value={String(sector.codigo)}>
                <AccordionTrigger className="text-sm">
                  <span className="flex items-center gap-2">
                    {sector.label}
                    {!sector.habilitado && (
                      <span className="text-muted-foreground text-xs">
                        · no disponible
                      </span>
                    )}
                    {!sector.soportado && (
                      <span className="text-muted-foreground text-xs">
                        · no soportado en esta versión
                      </span>
                    )}
                  </span>
                </AccordionTrigger>
                <AccordionContent className="flex flex-col gap-2 text-[13px]">
                  {sector.ejemplo && (
                    <p className="text-muted-foreground">
                      Ej.: <span className="text-foreground">{sector.ejemplo}</span>
                    </p>
                  )}
                  {sector.campos && sector.campos.length > 0 && (
                    <ul className="flex flex-col gap-1">
                      {sector.campos.map((campo) => (
                        <li key={campo.clave} className="flex items-center gap-1.5">
                          <span className={cn(campo.requerido && "font-medium")}>
                            {campo.label}
                            {campo.requerido && " *"}
                          </span>
                          <span className="text-muted-foreground text-xs">
                            ({campo.tipo})
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        )}
      </section>

      <Button
        variant="ghost"
        size="sm"
        className="self-start"
        onClick={() =>
          queryClient.invalidateQueries({ queryKey: ["sectores"] })
        }
      >
        Recargar catálogo del SIAT
      </Button>
    </div>
  )
}

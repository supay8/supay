import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { TriangleAlert } from "lucide-react"

import { api } from "@/lib/api"
import { formatCurrency, formatDateTime } from "@/lib/format"
import { FormSection, PageHeader, QueryErrorState } from "@/components/shared/page-parts"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { LogoSupay } from "@/components/logo"

export function ContingenciaPage() {
  const navigate = useNavigate()

  const offlineQuery = useQuery({
    queryKey: ["invoices", "contingencia"],
    queryFn: () => api.invoices.list({ status: ["OFFLINE"], limit: 50 }),
    refetchInterval: 8000,
    retry: false,
  })

  const offline = offlineQuery.data?.items ?? []
  const total = offlineQuery.data?.total ?? 0

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <PageHeader
        title="Contingencia"
        description="Qué pasa cuando el SIAT no responde y cómo se resuelve solo."
      />


      <section className="rounded-lg border p-5 text-sm">
        <p className="text-muted-foreground">
          Si el SIAT deja de responder, Supay sigue emitiendo en modo local: las
         
      <LogoSupay size={48} className="mx-auto text-primary" />

        </p>
      </section>

      <FormSection
        title="Facturas pendientes de envío"
        description="Emitidas durante contingencia; se mandan solas al reestablecerse el servicio."
      >
        {offlineQuery.isPending && (
          <div className="flex flex-col gap-2">
            {[...Array(3)].map((_, i) => (
              <Skeleton key={i} className="h-8 w-full" />
            ))}
          </div>
        )}

        {offlineQuery.isError && (
          <QueryErrorState
            error={offlineQuery.error}
            onRetry={() => offlineQuery.refetch()}
          />
        )}

        {!offlineQuery.isPending &&
          !offlineQuery.isError &&
          (offline.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-8 text-center">
              <span className="text-success inline-flex items-center gap-1.5 text-sm font-medium">
                <span className="bg-success size-1.5 rounded-full" />
                Todo en orden — sin facturas en contingencia
              </span>
              <span className="text-muted-foreground text-xs">
                El banner ámbar del encabezado aparecerá aquí si se activa el
                modo.
              </span>
            </div>
          ) : (
            <>
              <div className="mb-1 flex items-center gap-2 text-warning text-xs font-medium">
                <TriangleAlert className="size-3.5" />
                Modo contingencia activo · {total} factura(s) esperando envío
              </div>
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                      N°
                    </TableHead>
                    <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                      Cliente
                    </TableHead>
                    <TableHead className="text-muted-foreground text-right text-[11px] font-medium tracking-wider uppercase">
                      Total
                    </TableHead>
                    <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                      Fecha
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {offline.map((invoice) => (
                    <TableRow key={invoice.id}>
                      <TableCell className="font-mono text-xs">
                        {String(invoice.invoice_number).padStart(6, "0")}
                      </TableCell>
                      <TableCell>{invoice.customer.name}</TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatCurrency(invoice.total)}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-xs">
                        {formatDateTime(invoice.issue_date)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <div className="mt-2 flex justify-end">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => navigate("/invoices?status=OFFLINE")}
                >
                  Ver todas en el listado
                </Button>
              </div>
            </>
          ))}
      </FormSection>

      <p className="text-muted-foreground text-xs">
        El registro del evento significativo ante el SIAT lo maneja el backend
        automáticamente; no requiere acción manual{" "}
        <Tooltip>
          <TooltipTrigger
            render={
              <span className="underline decoration-dotted underline-offset-2" />
            }
          >
            (evento significativo)
          </TooltipTrigger>
          <TooltipContent className="max-w-64">
            Aviso obligatorio al SIAT de que se facturó fuera de línea, con su
            motivo y período.
          </TooltipContent>
        </Tooltip>
        .
      </p>
    </div>
  )
}


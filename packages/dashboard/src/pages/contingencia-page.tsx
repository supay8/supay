import { useQuery } from "@tanstack/react-query"
import { useDashboardHost } from "../host-context"
import { useAuth } from "../auth-context"
import { useNavigate } from "react-router-dom"
import { TriangleAlert } from "lucide-react"


import { formatCurrency, formatDateTime } from "../lib/format"
import { formatInvoiceTitle, formatPosShort } from "../lib/display-names"
import { FormSection, PageHeader, QueryErrorState } from "../components/shared/page-parts"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../components/ui/tooltip"
import { Button } from "../components/ui/button"
import { Skeleton } from "../components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table"

export function ContingenciaPage() {
  const host = useDashboardHost()
  const navigate = useNavigate()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""

  const offlineQuery = useQuery({
    queryKey: ["invoices", companyId, "contingencia"],
    queryFn: () => host.listInvoices({ status: ["OFFLINE"], limit: 50 }),
    refetchInterval: 8000,
    retry: false,
    enabled: companyId !== "",
  })

  const offline = offlineQuery.data?.items ?? []
  const total = offlineQuery.data?.total ?? 0

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <PageHeader
        title="Contingencia"
        description="Soporte cuando el SIAT no responde: lo emite tu ERP/POS, acá lo reenviás."
      />

      <section className="rounded-lg border p-5 text-sm">
        <p className="text-muted-foreground">
          Si el SIAT deja de responder, tu ERP/POS sigue emitiendo en modo local:
          las facturas quedan con validez legal bajo un evento de contingencia.
          Tu integración las reintenta sola; desde acá podés forzar el reenvío por
          paquete en Lotes.
        </p>
      </section>

      <FormSection
        title="Facturas pendientes de envío"
        description="Emitidas por tu ERP/POS durante contingencia; reenvialas por paquete en Lotes."
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
              <div className="overflow-x-auto rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="text-muted-foreground text-xs font-medium">
                      N°
                    </TableHead>
                    <TableHead className="text-muted-foreground text-xs font-medium">
                      Cliente
                    </TableHead>
                    <TableHead className="text-muted-foreground hidden text-xs font-medium md:table-cell">
                      Punto de venta
                    </TableHead>
                    <TableHead className="text-muted-foreground text-right text-xs font-medium">
                      Total
                    </TableHead>
                    <TableHead className="text-muted-foreground hidden text-xs font-medium lg:table-cell">
                      Fecha
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {offline.map((invoice) => (
                    <TableRow key={invoice.id}>
                      <TableCell className="font-mono text-xs">
                        {formatInvoiceTitle(invoice)}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col">
                          <span className="font-medium">{invoice.customer.name}</span>
                          <span className="text-muted-foreground text-[11px] md:hidden">
                            {invoice.point_of_sale ? formatPosShort(invoice.point_of_sale) : "—"}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground hidden text-xs md:table-cell">
                        {invoice.point_of_sale ? formatPosShort(invoice.point_of_sale) : "—"}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatCurrency(invoice.total)}
                      </TableCell>
                      <TableCell className="text-muted-foreground hidden text-xs lg:table-cell">
                        {formatDateTime(invoice.issue_date)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              </div>
              <div className="mt-2 flex justify-end gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => navigate("/invoices?status=OFFLINE")}
                >
                  Ver todas en el listado
                </Button>
                <Button
                  size="sm"
                  onClick={() => {
                    const firstPos = offline[0]?.point_of_sale_id
                    navigate(firstPos ? `/operation/lotes?pos=${firstPos}` : "/operation/lotes")
                  }}
                >
                  Armar lote para enviar
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


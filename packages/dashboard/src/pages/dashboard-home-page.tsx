import { useDashboardHost } from "../host-context"
import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router-dom"

import { useAuth } from "../auth-context"
import { formatCurrency } from "../lib/format"
import { PageHeader } from "../components/shared/page-parts"
import { Button } from "../components/ui/button"
import { Skeleton } from "../components/ui/skeleton"

function StatCard({
  label,
  value,
  hint,
  to,
}: {
  label: string
  value: string
  hint?: string
  to?: string
}) {
  const inner = (
    <div className="flex flex-col gap-1 rounded-lg border p-4">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="text-2xl font-semibold tabular-nums">{value}</span>
      {hint && <span className="text-muted-foreground text-xs">{hint}</span>}
    </div>
  )
  return to ? <Link to={to}>{inner}</Link> : inner
}

/**
 * Home operativo: KPIs accionables con deep-links.
 * Sin endpoints nuevos: reutiliza listInvoices + listPointsOfSale.
 */
export function DashboardHomePage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    enabled: companyId !== "",
    staleTime: 60_000,
  })

  const pendingQuery = useQuery({
    queryKey: ["invoices", "home-pending"],
    queryFn: () =>
      host.listInvoices({ status: ["PENDING", "SENDING"], limit: 5 }),
    staleTime: 5_000,
  })

  const rejectedQuery = useQuery({
    queryKey: ["invoices", "home-rejected"],
    queryFn: () => host.listInvoices({ status: ["REJECTED"], limit: 5 }),
    staleTime: 30_000,
  })

  const offlineQuery = useQuery({
    queryKey: ["invoices", "home-offline"],
    queryFn: () => host.listInvoices({ status: ["OFFLINE"], limit: 50 }),
    staleTime: 8_000,
  })

  const acceptedQuery = useQuery({
    queryKey: ["invoices", "home-accepted"],
    queryFn: () => host.listInvoices({ status: ["ACCEPTED"], limit: 20 }),
    staleTime: 30_000,
  })

  const posList = posQuery.data?.items ?? []
  const connected = posList.filter((p) => p.cuis).length
  const acceptedTotal = (acceptedQuery.data?.items ?? []).reduce((acc, i) => acc + i.total, 0)

  return (
    <div className="flex flex-col gap-5">
      <PageHeader
        title="Panel"
        description="Estado de facturación y conexión SIAT."
        actions={
          <Button size="sm" render={<Link to="/invoices/new" />}>
            Nueva factura
          </Button>
        }
      />

      {(posQuery.isPending || pendingQuery.isPending) && (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <Skeleton key={i} className="h-24 w-full" />
          ))}
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Puntos de venta conectados"
          value={`${connected}/${posList.length}`}
          hint="CUIS vigente"
          to="/company/siat"
        />
        <StatCard
          label="En curso"
          value={String(pendingQuery.data?.total ?? pendingQuery.data?.items.length ?? 0)}
          hint="PENDING + SENDING"
          to="/invoices"
        />
        <StatCard
          label="Rechazadas"
          value={String(rejectedQuery.data?.total ?? rejectedQuery.data?.items.length ?? 0)}
          hint="Requieren corrección"
          to="/invoices"
        />
        <StatCard
          label="Contingencia"
          value={String(offlineQuery.data?.total ?? offlineQuery.data?.items.length ?? 0)}
          hint="OFFLINE por enviar"
          to="/operation/contingencia"
        />
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <div className="flex flex-col gap-2 rounded-lg border p-4">
          <p className="text-sm font-medium">Últimas aceptadas · {formatCurrency(acceptedTotal)}</p>
          <div className="flex flex-col gap-1.5">
            {(acceptedQuery.data?.items ?? []).slice(0, 5).map((i) => (
              <div key={i.id} className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">
                  N° {String(i.invoice_number).padStart(6, "0")} · {i.customer.name}
                </span>
                <span className="font-medium tabular-nums">{formatCurrency(i.total)}</span>
              </div>
            ))}
            {(acceptedQuery.data?.items ?? []).length === 0 && (
              <p className="text-muted-foreground text-sm">Aún sin facturas aceptadas.</p>
            )}
          </div>
          <div>
            <Button size="sm" variant="outline" render={<Link to="/invoices" />}>
              Ver facturas
            </Button>
          </div>
        </div>

        <div className="flex flex-col gap-2 rounded-lg border p-4">
          <p className="text-sm font-medium">Acciones pendientes</p>
          {(rejectedQuery.data?.items ?? []).length > 0 && (
            <p className="text-sm">
              Hay {(rejectedQuery.data?.items ?? []).length} rechazadas por el SIAT. Corregilas
              desde el listado.
            </p>
          )}
          {(offlineQuery.data?.items ?? []).length > 0 && (
            <p className="text-sm">
              Hay {(offlineQuery.data?.items ?? []).length} en contingencia (OFFLINE). Se envían
              por paquete al volver la conexión.
            </p>
          )}
          {connected < posList.length && (
            <p className="text-sm">
              {posList.length - connected} punto(s) sin CUIS. Revisá la conexión SIAT.
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" render={<Link to="/company/siat" />}>
              Conexión SIAT
            </Button>
            <Button size="sm" variant="outline" render={<Link to="/operation/lotes" />}>
              Enviar lote
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

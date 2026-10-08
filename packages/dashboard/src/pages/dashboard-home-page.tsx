import { useDashboardHost } from "../host-context"
import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router-dom"
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  Clock,
  FileText,
  Plug,
  Store,
  TrendingUp,
  XCircle,
} from "lucide-react"

import { useAuth } from "../auth-context"
import { formatCurrency } from "../lib/format"
import { PageHeader, QueryErrorState } from "../components/shared/page-parts"
import { Button } from "../components/ui/button"
import { Skeleton } from "../components/ui/skeleton"

function StatCard({
  label,
  value,
  hint,
  to,
  icon: Icon,
  variant = "default",
}: {
  label: string
  value: string
  hint?: string
  to?: string
  icon?: typeof FileText
  variant?: "default" | "success" | "warning" | "danger"
}) {
  const gradients = {
    default: "from-surface/80 to-surface/40 border-border/60",
    success: "from-success/5 to-success/[0.02] border-success/20",
    warning: "from-warning/5 to-warning/[0.02] border-warning/20",
    danger: "from-destructive/5 to-destructive/[0.02] border-destructive/20",
  }
  const iconColors = {
    default: "text-muted-foreground",
    success: "text-success",
    warning: "text-warning",
    danger: "text-destructive",
  }

  const inner = (
    <div
      className={`group relative flex flex-col gap-2 overflow-hidden rounded-xl border bg-gradient-to-br p-4 transition-all duration-200 hover:shadow-md hover:shadow-black/5 ${gradients[variant]}`}
    >
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-xs font-medium tracking-wide">
          {label}
        </span>
        {Icon && (
          <Icon
            className={`size-4 transition-transform duration-200 group-hover:scale-110 ${iconColors[variant]}`}
          />
        )}
      </div>
      <span className="text-2xl font-bold tabular-nums tracking-tight">
        {value}
      </span>
      {hint && (
        <span className="text-muted-foreground text-[11px]">{hint}</span>
      )}
      {to && (
        <ArrowRight className="absolute right-3 bottom-3 size-3.5 text-muted-foreground/40 transition-all duration-200 group-hover:translate-x-0.5 group-hover:text-muted-foreground" />
      )}
    </div>
  )
  return to ? (
    <Link to={to} className="focus-visible:ring-ring rounded-xl outline-none focus-visible:ring-2">
      {inner}
    </Link>
  ) : (
    inner
  )
}

function SystemHealthIndicator({
  connected,
  total,
}: {
  connected: number
  total: number
}) {
  const ratio = total > 0 ? connected / total : 0
  const status = ratio >= 1 ? "healthy" : ratio > 0 ? "partial" : "offline"
  const config = {
    healthy: {
      color: "bg-success",
      glow: "shadow-success/40",
      label: "Sistema operativo",
      desc: "Todos los puntos de venta conectados al SIAT",
    },
    partial: {
      color: "bg-warning",
      glow: "shadow-warning/40",
      label: "Conexión parcial",
      desc: `${total - connected} punto(s) sin conexión SIAT`,
    },
    offline: {
      color: "bg-destructive",
      glow: "shadow-destructive/40",
      label: "Sin conexión",
      desc: "Ningún punto de venta conectado",
    },
  }
  const c = config[status]

  return (
    <div className="animate-fade-in flex items-center gap-3 rounded-lg border border-border/60 bg-surface/50 px-4 py-2.5">
      <span
        className={`animate-pulse-dot size-2.5 rounded-full ${c.color} shadow-[0_0_8px] ${c.glow}`}
      />
      <div className="flex flex-col">
        <span className="text-[13px] font-medium">{c.label}</span>
        <span className="text-muted-foreground text-[11px]">{c.desc}</span>
      </div>
    </div>
  )
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
    queryKey: ["invoices", companyId, "home-pending"],
    queryFn: () =>
      host.listInvoices({ status: ["PENDING", "SENDING"], limit: 5 }),
    staleTime: 5_000,
    enabled: companyId !== "",
  })

  const rejectedQuery = useQuery({
    queryKey: ["invoices", companyId, "home-rejected"],
    queryFn: () => host.listInvoices({ status: ["REJECTED"], limit: 5 }),
    staleTime: 30_000,
    enabled: companyId !== "",
  })

  const offlineQuery = useQuery({
    queryKey: ["invoices", companyId, "home-offline"],
    queryFn: () => host.listInvoices({ status: ["OFFLINE"], limit: 50 }),
    staleTime: 8_000,
    enabled: companyId !== "",
  })

  const acceptedQuery = useQuery({
    queryKey: ["invoices", companyId, "home-accepted"],
    queryFn: () => host.listInvoices({ status: ["ACCEPTED"], limit: 20 }),
    staleTime: 30_000,
    enabled: companyId !== "",
  })

  const posList = posQuery.data?.items ?? []
  const connected = posList.filter((p) => p.cuis).length
  const acceptedTotal = (acceptedQuery.data?.items ?? []).reduce(
    (acc, i) => acc + i.total,
    0
  )
  const recentAccepted = (acceptedQuery.data?.items ?? []).slice(0, 5)

  const pendingCount =
    pendingQuery.data?.total ??
    pendingQuery.data?.items.length ??
    0
  const rejectedCount =
    rejectedQuery.data?.total ??
    rejectedQuery.data?.items.length ??
    0
  const offlineCount =
    offlineQuery.data?.total ??
    offlineQuery.data?.items.length ??
    0

  const isLoading =
    posQuery.isPending || pendingQuery.isPending
  const hasQueryError =
    posQuery.isError ||
    pendingQuery.isError ||
    rejectedQuery.isError ||
    offlineQuery.isError ||
    acceptedQuery.isError

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <PageHeader
          title="Panel"
          description="Administración del backend: conexión SIAT y monitoreo de lo que emite tu ERP/POS."
          actions={
            <Button size="sm" variant="outline" render={<Link to="/company/siat" />}>
              <Plug data-icon="inline-start" />
              Conexión SIAT
            </Button>
          }
        />
      </div>

      {hasQueryError && (
        <div className="flex flex-col gap-2">
          <QueryErrorState
            error={posQuery.error ?? pendingQuery.error ?? rejectedQuery.error ?? offlineQuery.error ?? acceptedQuery.error}
            onRetry={() => {
              posQuery.refetch()
              pendingQuery.refetch()
              rejectedQuery.refetch()
              offlineQuery.refetch()
              acceptedQuery.refetch()
            }}
          />
          <p className="text-muted-foreground text-xs">
            Los contadores en 0 pueden ser por este error, no porque esté todo al día.
          </p>
        </div>
      )}

      {!posQuery.isPending && !posQuery.isError && posList.length === 0 && (
        <div className="flex flex-col gap-3 rounded-xl border border-primary/20 bg-primary/5 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-semibold">Te falta 1 paso para facturar</p>
            <p className="text-muted-foreground text-xs">
              Creá tu sucursal, punto de venta y conectá el SIAT. Te lleva 5 minutos.
            </p>
          </div>
          <Button size="sm" render={<Link to="/setup" />}>
            Completar configuración
          </Button>
        </div>
      )}

      <SystemHealthIndicator
        connected={connected}
        total={posList.length}
      />

      {isLoading && (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <Skeleton key={i} className="h-28 w-full rounded-xl" />
          ))}
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Puntos conectados"
          value={`${connected}/${posList.length}`}
          hint="Con CUIS vigente"
          to="/company/siat"
          icon={Store}
          variant={
            posList.length === 0
              ? "default"
              : connected === posList.length
                ? "success"
                : "warning"
          }
        />
        <StatCard
          label="En curso"
          value={String(pendingCount)}
          hint="Borradores enviándose"
          to="/invoices?status=PENDING"
          icon={Clock}
          variant={pendingCount > 0 ? "warning" : "default"}
        />
        <StatCard
          label="Rechazadas"
          value={String(rejectedCount)}
          hint="Requieren corrección"
          to="/invoices?status=REJECTED"
          icon={XCircle}
          variant={rejectedCount > 0 ? "danger" : "default"}
        />
        <StatCard
          label="Contingencia"
          value={String(offlineCount)}
          hint="Pendientes por reenviar"
          to="/operation/contingencia"
          icon={AlertTriangle}
          variant={offlineCount > 0 ? "warning" : "default"}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {/* Últimas aceptadas */}
        <div className="animate-slide-up flex flex-col gap-3 rounded-xl border bg-gradient-to-br from-success/[0.03] to-transparent p-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <TrendingUp className="size-4 text-success" />
              <p className="text-sm font-semibold">Últimas aceptadas</p>
            </div>
            <span className="text-success tabular-nums text-sm font-bold" title="Suma de las últimas 20 aceptadas">
              {formatCurrency(acceptedTotal)}
            </span>
          </div>
          <div className="flex flex-col gap-1.5">
            {recentAccepted.length === 0 ? (
              <p className="text-muted-foreground py-4 text-center text-sm">
                Aún sin facturas aceptadas.
              </p>
            ) : (
              recentAccepted.map((i, idx) => (
                <div
                  key={i.id}
                  className={`animate-slide-up stagger-${idx + 1} flex items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors hover:bg-surface/80`}
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <CheckCircle2 className="size-3.5 shrink-0 text-success/60" />
                    <span className="text-muted-foreground truncate">
                      <span className="text-foreground font-medium">
                        N° {String(i.invoice_number).padStart(6, "0")}
                      </span>
                      {" · "}
                      {i.customer.name}
                    </span>
                  </div>
                  <span className="shrink-0 font-medium tabular-nums">
                    {formatCurrency(i.total)}
                  </span>
                </div>
              ))
            )}
          </div>
          <div className="pt-1">
            <Button
              size="sm"
              variant="outline"
              render={<Link to="/invoices" />}
            >
              Ver facturas
              <ArrowRight data-icon="inline-end" />
            </Button>
          </div>
        </div>

        {/* Acciones pendientes */}
        <div className="animate-slide-up stagger-2 flex flex-col gap-3 rounded-xl border p-5">
          <div className="flex items-center gap-2">
            <FileText className="size-4 text-muted-foreground" />
            <p className="text-sm font-semibold">Acciones pendientes</p>
          </div>
          <div className="flex flex-col gap-2.5">
            {rejectedCount > 0 && (
              <div className="flex items-start gap-2.5 rounded-lg border border-destructive/10 bg-destructive/[0.03] p-3 text-sm">
                <XCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
                <div>
                  <p className="font-medium">
                    {rejectedCount} factura(s) rechazada(s)
                  </p>
                  <p className="text-muted-foreground text-xs">
                    Requieren corrección antes de reenviar al SIAT.
                  </p>
                </div>
              </div>
            )}
            {offlineCount > 0 && (
              <div className="flex items-start gap-2.5 rounded-lg border border-warning/10 bg-warning/[0.03] p-3 text-sm">
                <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" />
                <div>
                  <p className="font-medium">
                    {offlineCount} en contingencia
                  </p>
                  <p className="text-muted-foreground text-xs">
                    Se envían por paquete al volver la conexión.
                  </p>
                </div>
              </div>
            )}
            {connected < posList.length && posList.length > 0 && (
              <div className="flex items-start gap-2.5 rounded-lg border border-warning/10 bg-warning/[0.03] p-3 text-sm">
                <Plug className="mt-0.5 size-4 shrink-0 text-warning" />
                <div>
                  <p className="font-medium">
                    {posList.length - connected} punto(s) sin CUIS
                  </p>
                  <p className="text-muted-foreground text-xs">
                    Revisá la conexión SIAT para habilitar la facturación.
                  </p>
                </div>
              </div>
            )}
            {!hasQueryError &&
              rejectedCount === 0 &&
              offlineCount === 0 &&
              connected >= posList.length && (
                <div className="flex items-center gap-2.5 rounded-lg border border-success/10 bg-success/[0.03] p-3 text-sm">
                  <CheckCircle2 className="size-4 shrink-0 text-success" />
                  <p className="font-medium text-success">
                    Todo al día — sin acciones pendientes
                  </p>
                </div>
              )}
          </div>
          <div className="flex flex-wrap gap-2 pt-1">
            <Button
              size="sm"
              variant="outline"
              render={<Link to="/company/siat" />}
            >
              <Plug data-icon="inline-start" />
              Conexión SIAT
            </Button>
            <Button
              size="sm"
              variant="outline"
              render={<Link to="/operation/lotes" />}
            >
              Enviar lote
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

import { useLocation, useNavigate } from "react-router-dom"
import { useDashboardHost } from "../../host-context"
import { useAuth } from "../../auth-context"
import { Bell, MessageSquarePlus, Plus } from "lucide-react"
import { useQuery } from "@tanstack/react-query"
import { toast } from "sonner"

import { findActiveNav } from "../../lib/nav-config"
import { useDashboardExtensions } from "../../dashboard-context"
import { formatDateTime } from "../../lib/format"
import { OrganizationSwitcher } from "../../components/layout/organization-switcher"
import { Badge } from "../../components/ui/badge"
import { Button } from "../../components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../components/ui/select"
import { SidebarTrigger } from "../../components/ui/sidebar"
import type { Invoice } from "../../lib/types"

function NotificationsMenu() {
  const host = useDashboardHost()
  const navigate = useNavigate()

  const rejectedQuery = useQuery({
    queryKey: ["invoices", "rejected-count"],
    queryFn: () => host.listInvoices({ status: ["REJECTED"], limit: 5 }),
    staleTime: 15_000,
    refetchInterval: 30_000,
    retry: false,
  })

  const total = rejectedQuery.data?.total ?? 0
  const items = (rejectedQuery.data?.items ?? []) as Invoice[]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Notificaciones${total > 0 ? ` (${total})` : ""}`}
          />
        }
      >
        <Bell className="size-4" />
        {total > 0 && (
          <span
            className="bg-destructive absolute top-1.5 right-1.5 size-1.5 rounded-full"
            style={{
              boxShadow:
                "0 0 6px color-mix(in srgb, var(--destructive) 55%, transparent)",
            }}
          />
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={10} className="w-80 p-0">
        <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
          <span className="text-sm font-semibold tracking-tight">
            Notificaciones
          </span>
          <Badge variant="secondary" className="h-4 px-1.5 text-[10px]">
            {total} {total === 1 ? "nueva" : "nuevas"}
          </Badge>
        </div>

        {items.length === 0 ? (
          <div className="flex flex-col items-center py-8 text-center">
            <Bell className="text-muted-foreground/20 mb-3 size-8" />
            <span className="text-[13px] font-medium">Todo al día</span>
            <span className="text-muted-foreground mt-1 text-xs">
              No tienes alertas de facturación pendientes.
            </span>
          </div>
        ) : (
          <>
            <div className="max-h-64 overflow-y-auto py-1">
              {items.map((invoice) => (
                <DropdownMenuItem
                  key={invoice.id}
                  onClick={() => navigate("/invoices?status=REJECTED")}
                  className="flex-col items-start gap-0.5 px-4 py-2.5"
                >
                  <span className="w-full text-[13px] font-medium">
                    Factura{" "}
                    {String(invoice.invoice_number).padStart(6, "0")} rechazada
                    por el SIAT
                  </span>
                  <span className="text-muted-foreground w-full text-xs">
                    {invoice.customer.name} ·{" "}
                    {formatDateTime(invoice.issue_date)}
                  </span>
                </DropdownMenuItem>
              ))}
            </div>
            <DropdownMenuSeparator className="m-0" />
            <div className="p-1.5">
              <Button
                variant="ghost"
                size="sm"
                className="w-full text-xs"
                onClick={() => navigate("/invoices?status=REJECTED")}
              >
                Ver todas las rechazadas
              </Button>
            </div>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function AppHeader() {
  const location = useLocation()
  const navigate = useNavigate()
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const { navSections } = useDashboardExtensions()
  const crumbs = findActiveNav(location.pathname, navSections)
  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    retry: false,
    enabled: companyId !== "",
    staleTime: 60_000,
  })
  const posList = posQuery.data?.items ?? []

  return (
    <header className="bg-background/95 supports-[backdrop-filter]:bg-background/60 sticky top-0 z-10 flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border/60 px-4 backdrop-blur">
      <div className="flex min-w-0 items-center gap-2">
        <SidebarTrigger />
        <OrganizationSwitcher />
      </div>

        {crumbs && (
          <div className="hidden min-w-0 items-center gap-1.5 md:flex">
            <span className="text-border h-4 w-px" />
            {crumbs.parent && (
              <>
                <span className="truncate text-[13px] font-medium tracking-tight text-muted-foreground">
                  {crumbs.parent}
                </span>
                <span className="text-muted-foreground/60 text-[13px]">/</span>
              </>
            )}
            {crumbs.child && (
              <span className="truncate text-[13px] font-medium tracking-tight">
                {crumbs.child}
              </span>
            )}
          </div>
        )}

      <div className="flex shrink-0 items-center gap-2">
        {posList.length > 0 && (
          <Select defaultValue={posList[0].id}>
            <SelectTrigger
              className="hidden h-8 w-40 text-xs lg:flex"
              aria-label="Punto de venta activo"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {posList.map((pos) => (
                <SelectItem key={pos.id} value={pos.id}>
                  {pos.description}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        <Button size="sm" onClick={() => navigate("/invoices/new")}>
          <Plus data-icon="inline-start" />
          Nueva factura
        </Button>

        <button
          type="button"
          onClick={() =>
            toast.info("Gracias por ayudarnos a mejorar Supay")
          }
          className="hover:bg-accent hover:text-foreground focus-visible:ring-primary/40 flex h-8 items-center gap-2 rounded-md border border-border/60 bg-surface px-2.5 text-xs font-medium text-muted-foreground transition-all outline-none focus-visible:ring-2"
        >
          <MessageSquarePlus className="size-3.5" />
          <span className="hidden md:inline">Feedback</span>
        </button>

        <NotificationsMenu />
      </div>
    </header>
  )
}

import { useEffect, useState } from "react"
import { useDashboardHost } from "../host-context"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { useNavigate, useSearchParams } from "react-router-dom"
import { format } from "date-fns"
import type { DateRange } from "react-day-picker"
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  KeyRound,
  RefreshCw,
  Search,
  X,
} from "lucide-react"

import { formatPosShort } from "../lib/display-names"
import { useIsMobile } from "../hooks/use-mobile"
import { useAuth } from "../auth-context"
import {
  INVOICE_STATUS_LABELS,
  TRANSIENT_STATUSES,
} from "../lib/invoice-status"
import { formatCurrency, formatDateTime, formatTime } from "../lib/format"
import { INVOICE_STATUSES, type InvoiceStatus } from "../lib/types"
import { StatusBadge } from "../components/invoices/status-badge"
import { InvoiceDetailSheet } from "../components/invoices/invoice-detail-sheet"
import { QueryErrorState } from "../components/shared/page-parts"
import { Badge } from "../components/ui/badge"
import { Button } from "../components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "../components/ui/empty"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "../components/ui/input-group"
import { Calendar } from "../components/ui/calendar"
import { Popover, PopoverContent, PopoverTrigger } from "../components/ui/popover"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select"
import { Skeleton } from "../components/ui/skeleton"
import { Spinner } from "../components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table"
import type { Invoice } from "../lib/types"

const PAGE_SIZE = 20

function RowActions({
  invoice,
  onOpenDetail,
}: {
  invoice: Invoice
  onOpenDetail: (id: string) => void
}) {
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label="Ver detalle"
      onClick={(e) => {
        e.stopPropagation()
        onOpenDetail(invoice.id)
      }}
    >
      <ChevronRight />
    </Button>
  )
}

export function InvoicesPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const initialStatus = searchParams.get("status")
  const [posFilter, setPosFilter] = useState<string>("all")
  const [statuses, setStatuses] = useState<InvoiceStatus[]>(
    initialStatus && INVOICE_STATUSES.includes(initialStatus as InvoiceStatus)
      ? [initialStatus as InvoiceStatus]
      : []
  )
  const [range, setRange] = useState<DateRange | undefined>(undefined)
  const [inputValue, setInputValue] = useState("")
  const [q, setQ] = useState("")
  const [page, setPage] = useState(0)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const isMobile = useIsMobile()

  const hasActiveFilters =
    posFilter !== "all" || statuses.length > 0 || range !== undefined || inputValue.trim() !== ""

  function clearFilters() {
    setPosFilter("all")
    setStatuses([])
    setRange(undefined)
    setInputValue("")
    setQ("")
    setPage(0)
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      setQ(inputValue.trim())
      setPage(0)
    }, 300)
    return () => clearTimeout(timer)
  }, [inputValue])

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  const from = range?.from ? format(range.from, "yyyy-MM-dd") : undefined
  const to = range?.to ? format(range.to, "yyyy-MM-dd") : undefined

  const listQuery = useQuery({
    queryKey: ["invoices", companyId, "list", { posFilter, statuses, from, to, q, page }],
    queryFn: () =>
      host.listInvoices({
        // NOTE: el backend resuelve el tenant por X-Company-ID; el filtro POS es opcional.
        // No enviamos company_id por query: el scope va en la key + header.
        point_of_sale_id: posFilter === "all" ? undefined : posFilter,
        status: statuses.length > 0 ? statuses : undefined,
        from,
        to,
        q: q || undefined,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
    placeholderData: keepPreviousData,
    enabled: companyId !== "",
    refetchInterval: (query) =>
      query.state.data?.items.some((i) =>
        TRANSIENT_STATUSES.includes(i.status)
      )
        ? 5000
        : false,
  })

  const invoices = listQuery.data?.items ?? []
  const total = listQuery.data?.total ?? invoices.length
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const hasTransient = invoices.some((i) =>
    TRANSIENT_STATUSES.includes(i.status)
  )

  function toggleStatus(status: InvoiceStatus) {
    setStatuses((current) =>
      current.includes(status)
        ? current.filter((s) => s !== status)
        : [...current, status]
    )
    setPage(0)
  }

  function selectRange(selected: DateRange | undefined) {
    setRange(selected)
    setPage(0)
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Facturas</h1>
        <p className="text-sm text-muted-foreground">
          Lo emitido por tu empresa, con su estado real ante el SIAT.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={posFilter}
          onValueChange={(v) => {
            setPosFilter(v ?? "Todos los puntos")
            setPage(0)
          }}
        >
          <SelectTrigger
            className="h-9 w-44 text-xs"
            aria-label="Filtrar por punto de venta"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Todos los puntos</SelectItem>
            {(posQuery.data?.items ?? []).map((pos) => (
              <SelectItem key={pos.id} value={pos.id}>
                {formatPosShort(pos)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <DropdownMenu>
          <DropdownMenuTrigger render={<Button variant="outline" className="h-9 min-h-9 text-xs" />}>
            Estado
            {statuses.length > 0 && (
              <Badge variant="secondary" className="ml-1 h-4 px-1.5 text-[10px]">
                {statuses.length}
              </Badge>
            )}
            <ChevronDown className="ml-1 size-3 opacity-50" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-52">
            <DropdownMenuLabel>Filtrar por estado</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {INVOICE_STATUSES.map((status) => (
              <DropdownMenuCheckboxItem
                key={status}
                checked={statuses.includes(status)}
                onCheckedChange={() => toggleStatus(status)}
                closeOnClick={false}
              >
                {INVOICE_STATUS_LABELS[status]}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        <Popover>
          <PopoverTrigger
            render={
              <Button variant="outline" className="h-9 min-h-9 text-xs" />
            }
          >
            {range?.from && range?.to
              ? `${format(range.from, "dd/MM")} – ${format(range.to, "dd/MM")}`
              : range?.from
                ? `Desde ${format(range.from, "dd/MM")}`
                : "Fechas"}
          </PopoverTrigger>
          <PopoverContent className="w-auto max-w-[calc(100vw-2rem)] overflow-x-auto p-0" align="start">
            <Calendar
              mode="range"
              numberOfMonths={isMobile ? 1 : 2}
              selected={range}
              onSelect={selectRange}
            />
          </PopoverContent>
        </Popover>

        {statuses.map((status) => (
          <Badge key={status} variant="outline" className="gap-1 pr-1">
            {INVOICE_STATUS_LABELS[status]}
            <button
              type="button"
              aria-label={`Quitar filtro ${INVOICE_STATUS_LABELS[status]}`}
              className="min-h-6 min-w-6 rounded-full p-1.5 hover:bg-muted"
              onClick={() => toggleStatus(status)}
            >
              <X className="size-3" />
            </button>
          </Badge>
        ))}

        {hasActiveFilters && (
          <Button variant="ghost" size="sm" className="min-h-9" onClick={clearFilters}>
            Limpiar todo
          </Button>
        )}

        <InputGroup className="ml-auto h-9 w-full max-w-56">
          <InputGroupAddon>
            <Search className="size-3.5" />
          </InputGroupAddon>
          <InputGroupInput
            className="text-xs"
            placeholder="N°, NIT o CUF…"
            value={inputValue}
            onChange={(event) => setInputValue(event.target.value)}
          />
        </InputGroup>

        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          {listQuery.dataUpdatedAt > 0 && (
            <span className="hidden md:inline">
              Actualizado {formatTime(listQuery.dataUpdatedAt)}
            </span>
          )}
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Actualizar"
            onClick={() => listQuery.refetch()}
          >
            {listQuery.isFetching ? <Spinner /> : <RefreshCw />}
          </Button>
        </div>
      </div>

      {listQuery.isError && (
        <QueryErrorState
          error={listQuery.error}
          onRetry={() => listQuery.refetch()}
        />
      )}

      <div className="animate-fade-in overflow-x-auto rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="text-muted-foreground text-xs font-medium">
                N°
              </TableHead>
              <TableHead className="text-muted-foreground text-xs font-medium">
                Cliente
              </TableHead>
              <TableHead className="text-muted-foreground text-xs font-medium hidden md:table-cell">
                Punto de venta
              </TableHead>
              <TableHead className="text-muted-foreground text-right text-[11px] font-medium tracking-wider uppercase">
                Total
              </TableHead>
              <TableHead className="text-muted-foreground text-xs font-medium">
                Fecha
              </TableHead>
              <TableHead className="text-muted-foreground text-xs font-medium">
                Estado
              </TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {listQuery.isPending &&
              [...Array(8)].map((_, i) => (
                <TableRow key={i}>
                  {[...Array(7)].map((_, j) => (
                    <TableCell key={j} className={j === 2 ? "hidden md:table-cell" : ""}>
                      <Skeleton className="h-5 w-full max-w-32" />
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            {!listQuery.isPending &&
              invoices.map((invoice) => (
                <TableRow
                  key={invoice.id}
                  className="cursor-pointer"
                  onClick={() => setSelectedId(invoice.id)}
                >
                  <TableCell className="font-mono text-muted-foreground text-xs">
                    {String(invoice.invoice_number).padStart(6, "0")}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col">
                      <span className="font-medium">{invoice.customer.name}</span>
                      <span className="text-muted-foreground text-[11px] md:hidden">
                        {invoice.point_of_sale ? formatPosShort(invoice.point_of_sale) : "—"}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs hidden md:table-cell">
                    <span className="inline-flex items-center gap-1.5">
                      <span className={`size-1.5 rounded-full ${invoice.point_of_sale?.cuis ? "bg-success" : "bg-warning"}`} />
                      {invoice.point_of_sale ? formatPosShort(invoice.point_of_sale) : "—"}
                    </span>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatCurrency(invoice.total)}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">
                    {formatDateTime(invoice.issue_date)}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={invoice.status} />
                  </TableCell>
                  <TableCell>
                    <RowActions invoice={invoice} onOpenDetail={setSelectedId} />
                  </TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>

        {!listQuery.isPending &&
          !listQuery.isError &&
          invoices.length === 0 && (
            <Empty className="border-t">
              <EmptyHeader>
                <EmptyTitle>Sin facturas este período</EmptyTitle>
                <EmptyDescription>
                  Ajustá los filtros. Las facturas aparecen aquí cuando tu ERP/POS emite vía API (<span className="font-mono">POST /v1/invoices/emit</span>).
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button variant="outline" onClick={() => navigate("/company/api-keys")}>
                  <KeyRound data-icon="inline-start" />
                  Ver API keys para integrar
                </Button>
              </EmptyContent>
            </Empty>
          )}
      </div>

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {listQuery.isPending ? "…" : `${total} facturas`}
          {hasTransient && " · actualización automática activa"}
        </span>
        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="icon-sm"
            disabled={page === 0 || listQuery.isPending}
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            aria-label="Página anterior"
          >
            <ChevronLeft />
          </Button>
          <span className="px-2 tabular-nums">{page + 1} / {totalPages}</span>
          <Button
            variant="outline"
            size="icon-sm"
            disabled={(page + 1) * PAGE_SIZE >= total || listQuery.isPending}
            onClick={() => setPage((p) => p + 1)}
            aria-label="Página siguiente"
          >
            <ChevronRight />
          </Button>
        </div>
      </div>

      <InvoiceDetailSheet
        invoiceId={selectedId}
        onOpenChange={(open) => !open && setSelectedId(null)}
      />
    </div>
  )
}

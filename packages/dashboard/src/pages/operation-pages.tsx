import { useMemo, useState } from "react"
import { useDashboardHost } from "../host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useNavigate, useSearchParams } from "react-router-dom"
import { toast } from "sonner"

import { ApiError } from "../host"
import { useAuth } from "../auth-context"
import { formatCurrency, formatDateTime } from "../lib/format"
import { formatInvoiceTitle, formatPosShort } from "../lib/display-names"
import { PageHeader, QueryErrorState } from "../components/shared/page-parts"
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert"
import { Button } from "../components/ui/button"
import { Input } from "../components/ui/input"
import { Textarea } from "../components/ui/textarea"
import { Label } from "../components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select"
import { Spinner } from "../components/ui/spinner"
import { Skeleton } from "../components/ui/skeleton"

/**
 * Lotes: envío y validación de paquetes/masiva por nombres, sin pegar UUIDs.
 * Sector 30 solo admite masiva.
 */
export function LotesPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const queryClient = useQueryClient()
  const [searchParams] = useSearchParams()
  const preselectedPos = searchParams.get("pos") ?? ""
  const [posId, setPosId] = useState(preselectedPos)
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [batchId, setBatchId] = useState("")
  const [lastReception, setLastReception] = useState<string | null>(null)
  const [mode, setMode] = useState<"paquete" | "masiva">("paquete")

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    enabled: companyId !== "",
    staleTime: 60_000,
  })
  const posList = posQuery.data?.items ?? []
  const effectivePos = posId || posList[0]?.id || ""
  const effectivePosObj = posList.find((p) => p.id === effectivePos)

  const offlineQuery = useQuery({
    queryKey: ["invoices", companyId, "offline-50", effectivePos],
    queryFn: () =>
      host.listInvoices({
        point_of_sale_id: effectivePos || undefined,
        status: ["OFFLINE"],
        limit: 50,
      }),
    enabled: companyId !== "" && effectivePos !== "",
    staleTime: 8_000,
    retry: false,
  })
  const offline = useMemo(() => offlineQuery.data?.items ?? [], [offlineQuery.data])
  const ids = useMemo(() => offline.filter((i) => selected[i.id]).map((i) => i.id), [offline, selected])
  const selectedTotal = useMemo(
    () => offline.filter((i) => selected[i.id]).reduce((acc, i) => acc + i.total, 0),
    [offline, selected],
  )

  const sendMutation = useMutation({
    mutationFn: async () => {
      if (mode === "paquete") {
        if (!host.enviarPaquete) throw new ApiError(405, "NOT_SUPPORTED", "Host sin enviarPaquete")
        return host.enviarPaquete(effectivePos, { point_of_sale_id: effectivePos, invoice_ids: ids })
      }
      if (!host.enviarMasiva) throw new ApiError(405, "NOT_SUPPORTED", "Host sin enviarMasiva")
      return host.enviarMasiva(effectivePos, { point_of_sale_id: effectivePos, invoice_ids: ids })
    },
    onSuccess: async (res) => {
      setLastReception(res.reception_code ?? null)
      if (res.reception_code) {
        setBatchId(res.reception_code)
        try {
          await navigator.clipboard.writeText(res.reception_code)
        } catch {
          /* portapapeles no disponible */
        }
      }
      toast.success(res.reception_code ? `Lote enviado: ${res.reception_code}` : "Lote enviado", {
        action: res.reception_code
          ? { label: "Ir a validar", onClick: () => document.getElementById("lote-batch")?.focus() }
          : undefined,
      })
      setSelected({})
      queryClient.invalidateQueries({ queryKey: ["invoices", companyId] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "No se pudo enviar el lote"),
  })

  const validateMutation = useMutation({
    mutationFn: async () => {
      if (!batchId.trim()) throw new ApiError(400, "VALIDATION_ERROR", "Indicá el código de recepción")
      if (mode === "paquete") {
        if (!host.validarPaquete) throw new ApiError(405, "NOT_SUPPORTED", "Host sin validarPaquete")
        return host.validarPaquete(batchId.trim())
      }
      if (!host.validarMasiva) throw new ApiError(405, "NOT_SUPPORTED", "Host sin validarMasiva")
      return host.validarMasiva(batchId.trim())
    },
    onSuccess: (res) => {
      toast.success(res.message ?? "Validación completada")
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "No se pudo validar"),
  })

  function toggle(id: string, v: boolean) {
    setSelected((s) => ({ ...s, [id]: v }))
  }
  function selectAll(v: boolean) {
    const next: Record<string, boolean> = {}
    if (v) offline.forEach((i) => { next[i.id] = true })
    setSelected(next)
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <PageHeader
        title="Lotes (envío masivo)"
        description="Elegí facturas en contingencia por su número y cliente. Hasta 500 por envío."
      />
      {offline.length > 0 && (
        <Alert>
          <AlertTitle>{offline.length} en contingencia {effectivePosObj ? `en ${formatPosShort(effectivePosObj)}` : ""}</AlertTitle>
          <AlertDescription>
            Tildá las que quieras enviar. No necesitás copiar ningún código.
          </AlertDescription>
        </Alert>
      )}
      <div className="grid gap-4 rounded-lg border p-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-2">
            <Label>Canal</Label>
            <Select value={mode} onValueChange={(v) => setMode(v as "paquete" | "masiva")}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="paquete">Paquete contingencia</SelectItem>
                <SelectItem value="masiva">Masiva (única vía para sector 30)</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-muted-foreground text-xs">
              {mode === "masiva"
                ? "Masiva acepta todos los sectores, incluido el 30."
                : "Paquete reenvía lo emitido fuera de línea con su evento significativo."}
            </p>
          </div>
          <div className="flex flex-col gap-2">
            <Label>Punto de venta</Label>
            <Select value={effectivePos || undefined} onValueChange={(v) => { setPosId(v ?? ""); setSelected({}) }}>
              <SelectTrigger><SelectValue placeholder="Seleccioná PDV" /></SelectTrigger>
              <SelectContent>
                {posList.map((p) => (
                  <SelectItem key={p.id} value={p.id}>{formatPosShort(p)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="flex items-center justify-between">
          <Label>Facturas en contingencia ({offline.length})</Label>
          {offline.length > 0 && (
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onClick={() => selectAll(true)}>Todas</Button>
              <Button variant="ghost" size="sm" onClick={() => selectAll(false)}>Ninguna</Button>
            </div>
          )}
        </div>

        {posQuery.isError && (
          <QueryErrorState error={posQuery.error} onRetry={() => posQuery.refetch()} />
        )}
        {posList.length === 0 && !posQuery.isPending && !posQuery.isError && (
          <p className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
            Todavía no hay puntos de venta. Creá uno en Mi empresa → Puntos de venta.
          </p>
        )}
        {offlineQuery.isError ? (
          <QueryErrorState error={offlineQuery.error} onRetry={() => offlineQuery.refetch()} />
        ) : offlineQuery.isPending ? (
          <div className="flex flex-col gap-2" aria-busy="true" aria-label="Cargando pendientes">
            {[...Array(4)].map((_, i) => (
              <Skeleton key={i} className="h-10 w-full rounded-md" />
            ))}
          </div>
        ) : offline.length === 0 ? (
          <p className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
            Sin facturas en contingencia para este punto de venta.
          </p>
        ) : (
          <ul className="flex max-h-80 flex-col gap-1 overflow-y-auto rounded-md border p-2">
            {offline.map((i) => (
              <li key={i.id}>
                <label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-2 hover:bg-muted/50">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={!!selected[i.id]}
                    onChange={(e) => toggle(i.id, e.target.checked)}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">
                      {formatInvoiceTitle(i)} · {i.customer.name} · {formatCurrency(i.total)}
                    </span>
                    <span className="text-muted-foreground text-xs">{formatDateTime(i.issue_date)}</span>
                  </span>
                </label>
              </li>
            ))}
          </ul>
        )}

        <div className="flex flex-wrap items-center gap-3">
          <Button
            disabled={!effectivePos || ids.length === 0 || sendMutation.isPending}
            onClick={() => sendMutation.mutate()}
          >
            {sendMutation.isPending && <Spinner data-icon="inline-start" />}
            Enviar {mode} ({ids.length}){selectedTotal > 0 ? ` · ${formatCurrency(selectedTotal)}` : ""}
          </Button>
          {lastReception && (
            <span className="font-mono text-xs text-muted-foreground">Recepción: {lastReception}</span>
          )}
        </div>
      </div>
      <div className="flex flex-col gap-2 rounded-lg border p-4">
        <Label htmlFor="lote-batch">Validar recepción (código devuelto por el envío)</Label>
        <div className="flex gap-2">
          <Input
            id="lote-batch"
            placeholder="Código de recepción del lote"
            value={batchId}
            onChange={(e) => setBatchId(e.target.value)}
          />
          <Button
            variant="outline"
            disabled={validateMutation.isPending || !batchId.trim()}
            onClick={() => validateMutation.mutate()}
          >
            {validateMutation.isPending && <Spinner data-icon="inline-start" />}
            Validar
          </Button>
        </div>
      </div>
    </div>
  )
}

/**
 * Compras: recepción de compras por punto de venta.
 */
export function ComprasPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const navigate = useNavigate()
  const [posId, setPosId] = useState("")
  const [codes, setCodes] = useState("")

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    enabled: companyId !== "",
    staleTime: 60_000,
  })
  const posList = posQuery.data?.items ?? []
  const effectivePos = posId || posList[0]?.id || ""
  const ids = codes.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean)

  const sendMutation = useMutation({
    mutationFn: async () => {
      if (!host.enviarCompras) throw new ApiError(405, "NOT_SUPPORTED", "Host sin enviarCompras")
      return host.enviarCompras(effectivePos, { point_of_sale_id: effectivePos, invoice_ids: ids })
    },
    onSuccess: (res) => {
      toast.success(res.reception_code ? `Compras enviadas: ${res.reception_code}` : "Compras enviadas")
      setCodes("")
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "No se pudo enviar"),
  })

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4">
      <PageHeader
        title="Compras"
        description="Registrá tus compras ante el SIAT por punto de venta. No son facturas de venta."
      />
      <Alert>
        <AlertTitle>Libro de compras, no ventas</AlertTitle>
        <AlertDescription>
          Pegá aquí los CUF o códigos de tus documentos de compra (no son N° de tus facturas emitidas).
        </AlertDescription>
      </Alert>
      <div className="flex flex-col gap-4 rounded-lg border p-4">
        {posQuery.isPending && (
          <div className="flex flex-col gap-2" aria-busy="true" aria-label="Cargando puntos de venta">
            <Skeleton className="h-9 w-full rounded-md" />
          </div>
        )}
        {posQuery.isError && (
          <QueryErrorState error={posQuery.error} onRetry={() => posQuery.refetch()} />
        )}
        {posList.length === 0 && !posQuery.isPending && !posQuery.isError && (
          <p className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
            Todavía no hay puntos de venta. Creá uno en Mi empresa → Puntos de venta.
          </p>
        )}
        <div className="flex flex-col gap-2">
          <Label>Punto de venta</Label>
          <Select value={effectivePos || undefined} onValueChange={(v) => setPosId(v ?? "")}>
            <SelectTrigger><SelectValue placeholder="Seleccioná PDV" /></SelectTrigger>
            <SelectContent>
              {posList.map((p) => (
                <SelectItem key={p.id} value={p.id}>{formatPosShort(p)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="compras-ids">Códigos de compra (CUF o recepción, uno por línea)</Label>
          <Textarea
            id="compras-ids"
            rows={4}
            placeholder="Pegá un código por línea o separados por coma…"
            aria-describedby="compras-ids-hint"
            value={codes}
            onChange={(e) => setCodes(e.target.value)}
          />
          <p id="compras-ids-hint" className="text-muted-foreground text-xs">{ids.length} código(s) listos para enviar. Se separan por coma, espacio o salto de línea.</p>
        </div>
        <div className="flex gap-2">
          <Button
            disabled={!effectivePos || ids.length === 0 || sendMutation.isPending}
            onClick={() => sendMutation.mutate()}
          >
            {sendMutation.isPending && <Spinner data-icon="inline-start" />}
            Enviar compras ({ids.length})
          </Button>
          <Button variant="ghost" onClick={() => navigate("/operation/contingencia")}>
            Ver contingencia
          </Button>
        </div>
      </div>
    </div>
  )
}

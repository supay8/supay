import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"

import { ApiError } from "../host"
import { useAuth } from "../auth-context"
import { PageHeader } from "../components/shared/page-parts"
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert"
import { Button } from "../components/ui/button"
import { Input } from "../components/ui/input"
import { Label } from "../components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select"
import { Spinner } from "../components/ui/spinner"

/**
 * Operación por lotes: envío y validación de paquetes/masiva.
 * Usa endpoints reales: POST /v1/siat/paquete|masiva/{posId} + /{batchId}/validate.
 * El sector 30 solo admite vía masiva.
 */
export function LotesPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const [posId, setPosId] = useState("")
  const [invoiceIds, setInvoiceIds] = useState("")
  const [batchId, setBatchId] = useState("")
  const [mode, setMode] = useState<"paquete" | "masiva">("masiva")

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    enabled: companyId !== "",
    staleTime: 60_000,
  })
  const offlineQuery = useQuery({
    queryKey: ["invoices", "offline-50"],
    queryFn: () => host.listInvoices({ status: ["OFFLINE"], limit: 50 }),
    staleTime: 8_000,
  })

  const effectivePos = posId || posQuery.data?.items[0]?.id || ""
  const ids = invoiceIds.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean)

  const sendMutation = useMutation({
    mutationFn: async () => {
      if (mode === "paquete") {
        if (!host.enviarPaquete) throw new ApiError(405, "NOT_SUPPORTED", "Host sin enviarPaquete")
        return host.enviarPaquete(effectivePos, { point_of_sale_id: effectivePos, invoice_ids: ids })
      }
      if (!host.enviarMasiva) throw new ApiError(405, "NOT_SUPPORTED", "Host sin enviarMasiva")
      return host.enviarMasiva(effectivePos, { point_of_sale_id: effectivePos, invoice_ids: ids })
    },
    onSuccess: (res) => {
      toast.success(res.reception_code ? `Lote enviado: ${res.reception_code}` : "Lote enviado")
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "No se pudo enviar el lote"),
  })

  const validateMutation = useMutation({
    mutationFn: async () => {
      if (!batchId.trim()) throw new ApiError(400, "VALIDATION_ERROR", "Indicá el batchId")
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

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4">
      <PageHeader
        title="Lotes (envío masivo)"
        description="Paquetes de contingencia y masiva. Hasta 500 facturas por envío."
      />
      {(offlineQuery.data?.items ?? []).length > 0 && (
        <Alert>
          <AlertTitle>{(offlineQuery.data?.items ?? []).length} en contingencia</AlertTitle>
          <AlertDescription>
            IDs sugeridos: {(offlineQuery.data?.items ?? []).slice(0, 5).map((i) => i.id).join(", ")}
            {offlineQuery.data && offlineQuery.data.items.length > 5 ? "…" : ""}
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
                <SelectItem value="masiva">Masiva (incluye sector 30)</SelectItem>
                <SelectItem value="paquete">Paquete contingencia</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-2">
            <Label>Punto de venta</Label>
            <Select value={effectivePos || undefined} onValueChange={(v) => setPosId(v ?? "")}>
              <SelectTrigger><SelectValue placeholder="Seleccioná PDV" /></SelectTrigger>
              <SelectContent>
                {(posQuery.data?.items ?? []).map((p) => (
                  <SelectItem key={p.id} value={p.id}>{p.description}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="lote-ids">IDs de factura (coma o salto de línea)</Label>
          <Input
            id="lote-ids"
            placeholder="uuid-1, uuid-2…"
            value={invoiceIds}
            onChange={(e) => setInvoiceIds(e.target.value)}
          />
        </div>
        <div>
          <Button
            disabled={!effectivePos || ids.length === 0 || sendMutation.isPending}
            onClick={() => sendMutation.mutate()}
          >
            {sendMutation.isPending && <Spinner data-icon="inline-start" />}
            Enviar {mode} ({ids.length})
          </Button>
        </div>
      </div>
      <div className="flex flex-col gap-2 rounded-lg border p-4">
        <Label htmlFor="lote-batch">Validar recepción (batchId)</Label>
        <div className="flex gap-2">
          <Input
            id="lote-batch"
            placeholder="batchId devuelto por el envío"
            value={batchId}
            onChange={(e) => setBatchId(e.target.value)}
          />
          <Button
            variant="outline"
            disabled={validateMutation.isPending}
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
 * Compras: recepción de paquete de compras (libro de compras SIAT).
 * Endpoint real: POST /v1/siat/compras/{posId}.
 */
export function ComprasPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const navigate = useNavigate()
  const [posId, setPosId] = useState("")
  const [invoiceIds, setInvoiceIds] = useState("")

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    enabled: companyId !== "",
    staleTime: 60_000,
  })
  const effectivePos = posId || posQuery.data?.items[0]?.id || ""
  const ids = invoiceIds.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean)

  const sendMutation = useMutation({
    mutationFn: async () => {
      if (!host.enviarCompras) throw new ApiError(405, "NOT_SUPPORTED", "Host sin enviarCompras")
      return host.enviarCompras(effectivePos, { point_of_sale_id: effectivePos, invoice_ids: ids })
    },
    onSuccess: (res) => {
      toast.success(res.reception_code ? `Compras enviadas: ${res.reception_code}` : "Compras enviadas")
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "No se pudo enviar"),
  })

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4">
      <PageHeader
        title="Compras"
        description="Recepción de paquete de compras ante el SIAT."
      />
      <div className="flex flex-col gap-4 rounded-lg border p-4">
        <div className="flex flex-col gap-2">
          <Label>Punto de venta</Label>
          <Select value={effectivePos || undefined} onValueChange={(v) => setPosId(v ?? "")}>
            <SelectTrigger><SelectValue placeholder="Seleccioná PDV" /></SelectTrigger>
            <SelectContent>
              {(posQuery.data?.items ?? []).map((p) => (
                <SelectItem key={p.id} value={p.id}>{p.description}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="compras-ids">IDs de documentos de compra</Label>
          <Input
            id="compras-ids"
            placeholder="uuid-1, uuid-2…"
            value={invoiceIds}
            onChange={(e) => setInvoiceIds(e.target.value)}
          />
        </div>
        <div className="flex gap-2">
          <Button
            disabled={!effectivePos || ids.length === 0 || sendMutation.isPending}
            onClick={() => sendMutation.mutate()}
          >
            {sendMutation.isPending && <Spinner data-icon="inline-start" />}
            Enviar compras ({ids.length})
          </Button>
          <Button variant="ghost" onClick={() => navigate("/invoices")}>
            Ir a facturas
          </Button>
        </div>
      </div>
    </div>
  )
}

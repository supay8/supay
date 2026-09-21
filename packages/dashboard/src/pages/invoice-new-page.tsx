import { useMemo, useState } from "react"
import { useDashboardHost } from "../host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, useNavigate } from "react-router-dom"
import {
  ArrowLeft,
  Check,
  ChevronDown,
  CircleCheck,
  Copy,
  Download,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "../host"
import { useAuth } from "../auth-context"
import { formatCurrency } from "../lib/format"
import type {
  Customer,
  DraftItemInput,
  Invoice,
  InvoicePreview,
  Product,
  SectorFieldInfo,
  V1InvoiceInput,
} from "../lib/types"
import { CustomerCombobox } from "../components/invoices/customer-combobox"
import { ProductCombobox } from "../components/invoices/product-combobox"
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../components/ui/alert"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../components/ui/alert-dialog"
import { Button } from "../components/ui/button"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../components/ui/collapsible"
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table"
import { cn } from "../lib/utils"

interface ItemRow {
  key: string
  productId?: string
  code?: string
  description: string
  quantity: string
  unitPrice: string
  discount: string
}

type Phase = "idle" | "submitting" | "accepted" | "rejected"

const parseNum = (value: string): number => {
  const parsed = Number(value.replace(",", "."))
  return Number.isFinite(parsed) ? parsed : 0
}

const METODO_LABELS: Record<string, string> = {
  "1": "Efectivo",
  "2": "Tarjeta",
  "4": "Transferencia",
  "8": "QR",
}

const MONEDA_LABELS: Record<string, string> = {
  "1": "Bolivianos",
  "2": "Dólares",
}

function newKey(): string {
  return crypto.randomUUID()
}

function focusFirstError(errors: Record<string, string[]>) {
  const fields = Array.from(document.querySelectorAll<HTMLElement>("[data-field]"))
  for (const key of Object.keys(errors)) {
    const match =
      fields.find((el) => el.dataset.field === key) ??
      fields.find(
        (el) =>
          el.dataset.field !== undefined &&
          (key.includes(el.dataset.field) || el.dataset.field.includes(key))
      )
    if (match) {
      match.scrollIntoView({ behavior: "smooth", block: "center" })
      match.focus()
      return
    }
  }
}

function SectorInput({
  campo,
  value,
  error,
  onChange,
}: {
  campo: SectorFieldInfo
  value: string
  error?: string[]
  onChange: (value: string) => void
}) {
  const inputType =
    campo.tipo === "fecha"
      ? "date"
      : campo.tipo === "int" || campo.tipo === "float"
        ? "number"
        : "text"
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={`sector-${campo.clave}`}>
        {campo.label}
        {campo.requerido && <span className="text-destructive"> *</span>}
      </Label>
      <Input
        id={`sector-${campo.clave}`}
        data-field={`sector_data.${campo.clave}`}
        type={inputType}
        step={campo.tipo === "float" ? "any" : undefined}
        placeholder={campo.ejemplo}
        value={value}
        aria-invalid={error !== undefined}
        onChange={(e) => onChange(e.target.value)}
      />
      {campo.ejemplo && campo.tipo === "string" && (
        <p className="text-muted-foreground text-xs">ej.: {campo.ejemplo}</p>
      )}
      {error && <p className="text-destructive text-xs">{error.join(", ")}</p>}
    </div>
  )
}

export function InvoiceNewPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [customer, setCustomer] = useState<Customer | null>(null)
  const [posId, setPosId] = useState<string | null>(null)
  const [sectorCodigo, setSectorCodigo] = useState<number | null>(null)
  const [sectorValues, setSectorValues] = useState<Record<string, string>>({})
  const [items, setItems] = useState<ItemRow[]>([])
  const [metodoPago, setMetodoPago] = useState("1")
  const [moneda, setMoneda] = useState("1")
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({})
  const [phase, setPhase] = useState<Phase>("idle")
  const [resultInvoice, setResultInvoice] = useState<Invoice | null>(null)
  const [rejectedMessages, setRejectedMessages] = useState<string>("")
  const [unavailableOpen, setUnavailableOpen] = useState(false)
  const [preview, setPreview] = useState<InvoicePreview | null>(null)

  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    staleTime: 60_000,
    enabled: companyId !== "",
  })

  const sectoresQuery = useQuery({
    queryKey: ["sectores", companyId],
    queryFn: () => host.listSectores(companyId || undefined),
    staleTime: 5 * 60_000,
  })

  const posOptions = posQuery.data?.items ?? []
  const effectivePos = posId ?? posOptions[0]?.id ?? ""

  const sectores = sectoresQuery.data ?? []
  const habilitados = sectores.filter((s) => s.habilitado)
  const effectiveSector =
    sectores.find((s) => s.codigo === sectorCodigo) ??
    habilitados[0] ??
    null
  const sectorCampos = useMemo(
    () => effectiveSector?.campos ?? [],
    [effectiveSector]
  )

  const totals = useMemo(() => {
    let base = 0
    let discount = 0
    for (const item of items) {
      const lineBase = parseNum(item.quantity) * parseNum(item.unitPrice)
      const lineDiscount = parseNum(item.discount)
      base += lineBase
      discount += Math.min(lineDiscount, lineBase)
    }
    return { base, discount, total: base - discount }
  }, [items])

  const itemsValid =
    items.length > 0 &&
    items.every(
      (i) =>
        i.description.trim() !== "" &&
        parseNum(i.quantity) > 0 &&
        parseNum(i.unitPrice) > 0
    )
  const canEmit =
    customer !== null &&
    effectivePos !== "" &&
    effectiveSector !== null &&
    itemsValid

  function addProductLine(product: Product) {
    const index = items.length
    setItems((current) => [
      ...current,
      {
        key: newKey(),
        productId: product.id,
        code: product.sku,
        description: product.name,
        quantity: "1",
        unitPrice: "",
        discount: "0",
      },
    ])
    setTimeout(() => {
      document
        .querySelector<HTMLElement>(`[data-field="items.${index}.unit_price"]`)
        ?.focus()
    }, 50)
  }

  function addFreeLine() {
    setItems((current) => [
      ...current,
      {
        key: newKey(),
        description: "",
        quantity: "1",
        unitPrice: "",
        discount: "0",
      },
    ])
    setTimeout(() => {
      document
        .querySelector<HTMLElement>(`[data-field="items.${items.length}.description"]`)
        ?.focus()
    }, 50)
  }

  function updateItem(key: string, patch: Partial<ItemRow>) {
    setItems((current) =>
      current.map((item) => (item.key === key ? { ...item, ...patch } : item))
    )
  }

  function removeItem(key: string) {
    setItems((current) => current.filter((item) => item.key !== key))
  }

  function resetKeepingContext() {
    setItems([])
    setSectorValues({})
    setFieldErrors({})
    setRejectedMessages("")
    setPreview(null)
    setResultInvoice(null)
    setPhase("idle")
  }

  /** Payload v1 simplificado (POST /v1/invoices/preview|emit). */
  function buildV1Payload(): V1InvoiceInput {
    const sectorData = Object.fromEntries(
      Object.entries(sectorValues).filter(([, v]) => v.trim() !== "")
    )
    return {
      point_of_sale_id: effectivePos,
      customer: customer!.id.startsWith("manual-")
        ? {
            document_type: customer!.document_type,
            document_number: customer!.document_number,
            name: customer!.name,
          }
        : { id: customer!.id },
      items: items.map((i) => ({
        sku: i.code,
        quantity: parseNum(i.quantity),
        price: parseNum(i.unitPrice),
        discount: parseNum(i.discount),
        data: undefined,
      })),
      sector: effectiveSector ? String(effectiveSector.codigo) : "auto",
      data: sectorData,
      payment: { method_code: Number(metodoPago), currency_code: Number(moneda) },
    }
  }

  const previewMutation = useMutation({
    mutationFn: async () => {
      if (!host.previewInvoice) {
        throw new ApiError(405, "NOT_SUPPORTED", "Este backend no expone POST /v1/invoices/preview")
      }
      return host.previewInvoice(buildV1Payload())
    },
    onSuccess: (data) => {
      setPreview(data)
      setFieldErrors({})
      toast.success("Validación OK: podés emitir")
    },
    onError: (error) => {
      setPreview(null)
      if (error instanceof ApiError && error.code === "VALIDATION_ERROR") {
        const details = error.details ?? {}
        setFieldErrors(details)
        focusFirstError(details)
        return
      }
      toast.error(error instanceof ApiError ? error.message : "Falló la validación previa")
    },
  })

  function handleResult(result: {
    status: Phase
    invoice: Invoice
    error?: ApiError
  }) {
    queryClient.invalidateQueries({ queryKey: ["invoices"] })
    setResultInvoice(result.invoice)
    setPhase(result.status)
    if (result.status === "accepted") {
      toast.success(`Factura emitida: ${result.invoice.invoice_number}`)
    }
    if (result.status === "rejected" && result.error) {
      setRejectedMessages(result.error.message)
    }
    if (result.status === "idle") setUnavailableOpen(true)
  }

  const emitMutation = useMutation({
    mutationFn: async () => {
      // Clientes transitorios (manual-*) o payload v1: emisión atómica.
      if (customer!.id.startsWith("manual-") && host.emitInvoiceDirect) {
        const emitted = await host.emitInvoiceDirect(buildV1Payload())
        return { status: "accepted" as const, invoice: emitted }
      }
      const payload = {
        customer_id: customer!.id,
        point_of_sale_id: effectivePos,
        codigo_documento_sector: effectiveSector!.codigo,
        codigo_metodo_pago: Number(metodoPago),
        codigo_moneda: Number(moneda),
        items: items.map<DraftItemInput>((i) => ({
          product_id: i.productId,
          code: i.code,
          description: i.description.trim(),
          quantity: parseNum(i.quantity),
          unit_price: parseNum(i.unitPrice),
          discount: parseNum(i.discount),
        })),
        sector_data: Object.fromEntries(
          Object.entries(sectorValues).filter(([, v]) => v.trim() !== "")
        ),
      }
      const draft = await host.createDraft(payload)
      try {
        const emitted = await host.emitInvoice(draft.id)
        return { status: "accepted" as const, invoice: emitted }
      } catch (error) {
        if (
          error instanceof ApiError &&
          (error.code === "SIAT_REJECTED" || error.code === "SIAT_UNAVAILABLE")
        ) {
          return {
            status: error.code === "SIAT_REJECTED" ? ("rejected" as const) : ("idle" as const),
            invoice: draft,
            error,
          }
        }
        throw error
      }
    },
    onSuccess: handleResult,
    onError: (error) => {
      if (error instanceof ApiError && error.code === "VALIDATION_ERROR") {
        const details = error.details ?? {}
        setFieldErrors(details)
        focusFirstError(details)
        return
      }
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo emitir la factura"
      )
    },
  })

  const retryMutation = useMutation({
    mutationFn: () => host.emitInvoice(resultInvoice!.id),
    onSuccess: (emitted) => {
      setUnavailableOpen(false)
      handleResult({ status: "accepted", invoice: emitted })
    },
    onError: (error) => {
      setUnavailableOpen(false)
      if (error instanceof ApiError && error.code === "SIAT_REJECTED") {
        handleResult({ status: "rejected", invoice: resultInvoice!, error })
        return
      }
      if (error instanceof ApiError && error.code === "SIAT_UNAVAILABLE") {
        setUnavailableOpen(true)
        return
      }
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo reintentar el envío"
      )
    },
  })

  if (phase === "accepted" && resultInvoice) {
    return (
      <div className="mx-auto flex max-w-lg flex-col items-center gap-6 rounded-lg border p-10 text-center">
        <CircleCheck className="text-success size-12" />
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold tracking-tight">
            Factura emitida
          </h1>
          <p className="text-muted-foreground text-sm">
            N° {String(resultInvoice.invoice_number).padStart(6, "0")} ·{" "}
            {formatCurrency(resultInvoice.total)}
          </p>
        </div>
        {resultInvoice.cuf && (
          <div className="flex w-full items-center justify-between gap-3 rounded-md border p-3">
            <div className="min-w-0 text-left">
              <Label className="text-xs text-muted-foreground">CUF</Label>
              <p className="truncate font-mono text-xs">{resultInvoice.cuf}</p>
            </div>
            <CopyCufButton cuf={resultInvoice.cuf} />
          </div>
        )}
        <div className="flex flex-wrap justify-center gap-2">
          <Button
            variant="outline"
            render={<a href={host.getInvoicePdfUrl(resultInvoice.id)} download />}
          >
            <Download data-icon="inline-start" />
            Descargar PDF
          </Button>
          <Button variant="outline" onClick={() => navigate("/invoices")}>
            Ver en listado
          </Button>
          <Button onClick={resetKeepingContext}>Emitir otra</Button>
        </div>
      </div>
    )
  }

  const knownKeys = new Set([
    "customer_id",
    "point_of_sale_id",
    "codigo_documento_sector",
    "codigo_metodo_pago",
    "codigo_moneda",
    "items",
  ])
  const unmatchedErrors = Object.entries(fieldErrors).filter(
    ([key]) =>
      !knownKeys.has(key) && !key.startsWith("sector_data.")
  )

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-5 pb-28">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon-sm" render={<Link to="/invoices" />}>
          <ArrowLeft />
        </Button>
        <h1 className="text-xl font-semibold tracking-tight">Nueva factura</h1>
      </div>

      {phase === "rejected" && resultInvoice && (
        <Alert variant="destructive">
          <AlertTitle>
            El SIAT rechazó la factura{" "}
            {String(resultInvoice.invoice_number).padStart(6, "0")}
          </AlertTitle>
          <AlertDescription>
            {rejectedMessages || "Mensaje del SIAT no disponible."}
            <span className="mt-1 block">
              La factura quedó registrada como rechazada; al emitir de nuevo se
              creará un borrador corregido.
            </span>
          </AlertDescription>
          <div className="flex gap-2 pt-1">
            <Button size="sm" variant="outline" onClick={resetKeepingContext}>
              Crear corrección
            </Button>
            <Button size="sm" variant="ghost" onClick={() => navigate("/invoices")}>
              Ver en listado
            </Button>
          </div>
        </Alert>
      )}

      {unmatchedErrors.length > 0 && (
        <Alert variant="destructive">
          <AlertTitle>Revisá estos datos</AlertTitle>
          <AlertDescription>
            <ul className="list-inside list-disc">
              {unmatchedErrors.map(([key, messages]) => (
                <li key={key}>
                  {key}: {messages.join(", ")}
                </li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}

      <section
        className="flex flex-col gap-2 rounded-lg border p-4"
        data-field="customer_id"
      >
        <p className="text-sm font-medium">Cliente</p>
        <CustomerCombobox value={customer} onChange={setCustomer} />
        {fieldErrors["customer_id"] && (
          <p className="text-destructive text-xs">
            {fieldErrors["customer_id"].join(", ")}
          </p>
        )}
      </section>

      <section className="grid gap-4 rounded-lg border p-4 md:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label>Punto de venta</Label>
          <Select
            value={effectivePos || undefined}
            onValueChange={(v) => setPosId(v ?? null)}
          >
            <SelectTrigger data-field="point_of_sale_id" aria-label="Punto de venta">
              <SelectValue placeholder="Seleccioná punto de venta" />
            </SelectTrigger>
            <SelectContent>
              {posOptions.map((pos) => (
                <SelectItem key={pos.id} value={pos.id}>
                  {pos.description}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-2">
          <Label>Actividad (sector)</Label>
          <Select
            value={effectiveSector ? String(effectiveSector.codigo) : undefined}
            onValueChange={(v) => {
              setSectorCodigo(Number(v))
              setSectorValues({})
            }}
          >
            <SelectTrigger
              data-field="codigo_documento_sector"
              aria-label="Sector"
            >
              <SelectValue placeholder="Seleccioná actividad" />
            </SelectTrigger>
            <SelectContent>
              {sectores.map((sector) => (
                <SelectItem
                  key={sector.codigo}
                  value={String(sector.codigo)}
                  disabled={!sector.habilitado}
                >
                  {sector.label}
                  {!sector.habilitado && " (no disponible)"}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {sectorCampos.length > 0 && (
          <div className="flex flex-col gap-3 md:col-span-2">
            <p className="text-muted-foreground -mb-1 text-xs">
              Campos de la actividad:{" "}
              <span className="text-foreground font-medium">
                {effectiveSector?.label}
              </span>
            </p>
            <div className="grid gap-4 sm:grid-cols-2">
              {sectorCampos.map((campo) => (
                <SectorInput
                  key={campo.clave}
                  campo={campo}
                  value={sectorValues[campo.clave] ?? ""}
                  error={
                    fieldErrors[`sector_data.${campo.clave}`] ??
                    fieldErrors[campo.clave]
                  }
                  onChange={(value) =>
                    setSectorValues((current) => ({
                      ...current,
                      [campo.clave]: value,
                    }))
                  }
                />
              ))}
            </div>
          </div>
        )}
      </section>

      <Collapsible
        open={paymentOpen}
        onOpenChange={setPaymentOpen}
        className="rounded-lg border p-4"
      >
        <div className="flex items-center justify-between">
          <p className="text-sm font-medium">
            Pago · {METODO_LABELS[metodoPago]} en {MONEDA_LABELS[moneda]}
          </p>
          <CollapsibleTrigger render={<Button variant="ghost" size="sm" />}>
            {paymentOpen ? "Ocultar" : "Cambiar"}
            <ChevronDown
              className={cn(
                "size-4 transition-transform",
                paymentOpen && "rotate-180"
              )}
            />
          </CollapsibleTrigger>
        </div>
        <CollapsibleContent>
          <div className="mt-3 grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Método de pago</Label>
              <Select value={metodoPago} onValueChange={(v) => setMetodoPago(v ?? "1")}>
                <SelectTrigger data-field="codigo_metodo_pago">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">Efectivo</SelectItem>
                  <SelectItem value="2">Tarjeta débito/credito</SelectItem>
                  <SelectItem value="4">Transferencia</SelectItem>
                  <SelectItem value="8">QR</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Moneda</Label>
              <Select value={moneda} onValueChange={(v) => setMoneda(v ?? "1")}>
                <SelectTrigger data-field="codigo_moneda">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">Bolivianos (BOB)</SelectItem>
                  <SelectItem value="2">Dólares (USD)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CollapsibleContent>
      </Collapsible>

      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <Label>Ítems</Label>
          <div className="flex items-center gap-2">
            {preview != null && preview.total !== undefined && (
              <span className="text-muted-foreground text-xs tabular-nums">
                Preview: {formatCurrency(preview.total)} ✓
              </span>
            )}
            <ProductCombobox onSelectProduct={addProductLine} onFreeLine={addFreeLine} />
          </div>
        </div>

        {fieldErrors["items"] && (
          <p className="text-destructive text-xs">
            {fieldErrors["items"].join(", ")}
          </p>
        )}

        {items.length === 0 ? (
          <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
            Agregá productos del catálogo o una línea libre.
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/50">
                  <TableHead>Descripción</TableHead>
                  <TableHead className="w-20 text-right">Cant.</TableHead>
                  <TableHead className="w-28 text-right">Precio</TableHead>
                  <TableHead className="w-24 text-right">Dto.</TableHead>
                  <TableHead className="w-28 text-right">Subtotal</TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item, index) => {
                  const lineSubtotal =
                    parseNum(item.quantity) * parseNum(item.unitPrice) -
                    Math.min(parseNum(item.discount), parseNum(item.quantity) * parseNum(item.unitPrice))
                  return (
                    <TableRow key={item.key}>
                      <TableCell className="p-1.5">
                        <Input
                          data-field={`items.${index}.description`}
                          className="h-8 border-transparent bg-transparent shadow-none"
                          placeholder="Descripción…"
                          value={item.description}
                          onChange={(e) =>
                            updateItem(item.key, { description: e.target.value })
                          }
                        />
                      </TableCell>
                      <TableCell className="p-1.5">
                        <Input
                          data-field={`items.${index}.quantity`}
                          className="h-8 border-transparent bg-transparent text-right shadow-none"
                          type="number"
                          min="0"
                          step="any"
                          value={item.quantity}
                          onChange={(e) =>
                            updateItem(item.key, { quantity: e.target.value })
                          }
                        />
                      </TableCell>
                      <TableCell className="p-1.5">
                        <Input
                          data-field={`items.${index}.unit_price`}
                          className="h-8 border-transparent bg-transparent text-right shadow-none"
                          type="number"
                          min="0"
                          step="any"
                          placeholder="0,00"
                          value={item.unitPrice}
                          onChange={(e) =>
                            updateItem(item.key, { unitPrice: e.target.value })
                          }
                        />
                      </TableCell>
                      <TableCell className="p-1.5">
                        <Input
                          data-field={`items.${index}.discount`}
                          className="h-8 border-transparent bg-transparent text-right shadow-none"
                          type="number"
                          min="0"
                          step="any"
                          value={item.discount}
                          onChange={(e) =>
                            updateItem(item.key, { discount: e.target.value })
                          }
                        />
                      </TableCell>
                      <TableCell className="p-1.5 text-right tabular-nums">
                        {formatCurrency(lineSubtotal)}
                      </TableCell>
                      <TableCell className="p-1.5">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="Quitar ítem"
                          onClick={() => removeItem(item.key)}
                        >
                          <Trash2 />
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </section>

      <div className="bg-background/80 sticky bottom-0 -mx-4 mt-auto flex items-end justify-between gap-4 border-t px-4 py-3 backdrop-blur md:-mx-6 md:px-6">
        <Button variant="ghost" onClick={() => navigate("/invoices")}>
          Cancelar
        </Button>
        <div className="flex items-end gap-6">
          <div className="flex flex-col items-end text-sm">
            <span className="text-muted-foreground">
              Subtotal {formatCurrency(totals.base)}
            </span>
            {totals.discount > 0 && (
              <span className="text-muted-foreground">
                Descuento −{formatCurrency(totals.discount)}
              </span>
            )}
            <span className="text-base font-semibold tabular-nums">
              Total {formatCurrency(totals.total)}
            </span>
          </div>
          <Button
            variant="outline"
            size="lg"
            disabled={!canEmit || previewMutation.isPending}
            onClick={() => previewMutation.mutate()}
          >
            {previewMutation.isPending && <Spinner data-icon="inline-start" />}
            Validar
          </Button>
          <Button
            size="lg"
            disabled={!canEmit || emitMutation.isPending}
            onClick={() => emitMutation.mutate()}
          >
            {emitMutation.isPending && <Spinner data-icon="inline-start" />}
            Emitir factura
          </Button>
        </div>
      </div>

      <AlertDialog open={unavailableOpen} onOpenChange={setUnavailableOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>El SIAT no responde ahora</AlertDialogTitle>
            <AlertDialogDescription>
              Tu factura{" "}
              {resultInvoice
                ? `N° ${String(resultInvoice.invoice_number).padStart(6, "0")}`
                : ""}{" "}
              quedó guardada y NO se perdió. Podés reintentar el envío ahora o
              dejarla pendiente; se enviará sola cuando el servicio vuelva.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={resetKeepingContext}>
              Dejar pendiente
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={retryMutation.isPending}
              onClick={(event) => {
                event.preventDefault()
                retryMutation.mutate()
              }}
            >
              Reintentar ahora
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function CopyCufButton({ cuf }: { cuf: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label="Copiar CUF"
      onClick={() => {
        navigator.clipboard.writeText(cuf)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }}
    >
      {copied ? <Check /> : <Copy />}
    </Button>
  )
}

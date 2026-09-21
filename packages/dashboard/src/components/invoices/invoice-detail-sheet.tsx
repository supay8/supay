import { useState } from "react"
import { useDashboardHost } from "../../host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { Check, Copy, Download } from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "../../host"
import { formatCurrency, formatDateTime } from "../../lib/format"
import type { Invoice } from "../../lib/types"
import { StatusBadge } from "../../components/invoices/status-badge"
import { AnnulDialog } from "../../components/invoices/annul-dialog"
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../components/ui/alert"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../../components/ui/alert-dialog"
import { Button } from "../../components/ui/button"
import { Label } from "../../components/ui/label"
import { ScrollArea } from "../../components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "../../components/ui/sheet"
import { Skeleton } from "../../components/ui/skeleton"
import { Spinner } from "../../components/ui/spinner"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../../components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/tabs"

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 text-[13px]">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium">{value}</span>
    </div>
  )
}

interface InvoiceDetailSheetProps {
  invoiceId: string | null
  onOpenChange: (open: boolean) => void
}

export function InvoiceDetailSheet({
  invoiceId,
  onOpenChange,
}: InvoiceDetailSheetProps) {
  const host = useDashboardHost()
  const [annulOpen, setAnnulOpen] = useState(false)
  const [revertOpen, setRevertOpen] = useState(false)
  const [copied, setCopied] = useState(false)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const detail = useQuery({
    queryKey: ["invoices", invoiceId],
    queryFn: () => host.getInvoice(invoiceId!),
    enabled: invoiceId !== null,
    refetchInterval: (query) =>
      query.state.data &&
      ["PENDING", "SENDING"].includes(query.state.data.status)
        ? 4000
        : false,
  })

  const revertMutation = useMutation({
    mutationFn: () => host.revertAnnul(invoiceId!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["invoices"] })
      toast.success("Anulación revertida")
      setRevertOpen(false)
    },
    onError: (error) => {
      toast.error(
        error instanceof ApiError
          ? error.message
          : "No se pudo revertir la anulación"
      )
    },
  })

  const invoice: Invoice | undefined = detail.data

  function copyCuf(cuf: string) {
    navigator.clipboard.writeText(cuf)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <>
      <Sheet open={invoiceId !== null} onOpenChange={onOpenChange}>
        <SheetContent className="flex flex-col gap-0 p-0 sm:max-w-md">
          <SheetHeader className="border-b px-5 py-4">
            <div className="flex items-center justify-between gap-3">
              <SheetTitle className="text-sm font-medium">
                Factura{" "}
                {invoice
                  ? String(invoice.invoice_number).padStart(6, "0")
                  : ""}
              </SheetTitle>
              {invoice && <StatusBadge status={invoice.status} />}
            </div>
          </SheetHeader>

          {detail.isPending && (
            <div className="flex flex-col gap-3 p-5">
              {[...Array(6)].map((_, i) => (
                <Skeleton key={i} className="h-5 w-full" />
              ))}
            </div>
          )}

          {detail.isError && (
            <div className="p-5">
              <Alert variant="destructive">
                <AlertTitle>No se pudo cargar el detalle</AlertTitle>
                <AlertDescription>
                  {detail.error instanceof ApiError
                    ? detail.error.message
                    : String(detail.error)}
                </AlertDescription>
              </Alert>
            </div>
          )}

          {invoice && (
            <>
              <Tabs defaultValue="resumen" className="flex min-h-0 flex-1 flex-col gap-0">
                <div className="border-b px-5 pt-3">
                  <TabsList className="h-8 text-xs">
                    <TabsTrigger value="resumen">Resumen</TabsTrigger>
                    <TabsTrigger value="items">
                      Ítems ({invoice.items.length})
                    </TabsTrigger>
                    <TabsTrigger value="siat">SIAT</TabsTrigger>
                  </TabsList>
                </div>

                <ScrollArea className="min-h-0 flex-1">
                  <TabsContent value="resumen" className="mt-0 flex flex-col gap-3 p-5">
                    <DetailRow label="Cliente" value={invoice.customer.name} />
                    <DetailRow
                      label="Documento"
                      value={`${invoice.customer.document_type} ${invoice.customer.document_number}`}
                    />
                    <DetailRow
                      label="Punto de venta"
                      value={invoice.point_of_sale.description}
                    />
                    <DetailRow label="Emisión" value={formatDateTime(invoice.issue_date)} />
                    {invoice.fecha_anulacion && (
                      <DetailRow
                        label="Anulada"
                        value={formatDateTime(invoice.fecha_anulacion)}
                      />
                    )}
                    <div className="my-2 border-t" />
                    <DetailRow label="Subtotal" value={formatCurrency(invoice.subtotal)} />
                    {invoice.discount > 0 && (
                      <DetailRow
                        label="Descuento"
                        value={`−${formatCurrency(invoice.discount)}`}
                      />
                    )}
                    <DetailRow
                      label="Total"
                      value={
                        <span className="text-base">{formatCurrency(invoice.total)}</span>
                      }
                    />
                  </TabsContent>

                  <TabsContent value="items" className="mt-0 p-4">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Descripción</TableHead>
                          <TableHead className="text-right">Cant.</TableHead>
                          <TableHead className="text-right">Subtotal</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {invoice.items.map((item) => (
                          <TableRow key={item.id}>
                            <TableCell>{item.description}</TableCell>
                            <TableCell className="text-right">{item.quantity}</TableCell>
                            <TableCell className="text-right">
                              {formatCurrency(item.subtotal)}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TabsContent>

                  <TabsContent value="siat" className="mt-0 flex flex-col gap-3 p-5">
                    {invoice.cuf ? (
                      <div className="flex items-center justify-between gap-3 rounded-md border p-3">
                        <div className="min-w-0">
                          <Label className="text-xs text-muted-foreground">CUF</Label>
                          <p className="truncate font-mono text-xs">{invoice.cuf}</p>
                        </div>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => copyCuf(invoice.cuf!)}
                          aria-label="Copiar CUF"
                        >
                          {copied ? <Check /> : <Copy />}
                        </Button>
                      </div>
                    ) : (
                      <p className="text-sm text-muted-foreground">
                        Aún sin CUF: la factura no fue enviada al SIAT.
                      </p>
                    )}
                    {invoice.siat_reception_code && (
                      <DetailRow label="Código de recepción" value={invoice.siat_reception_code} />
                    )}
                    {invoice.siat_mensajes && (
                      <Alert variant="destructive">
                        <AlertTitle>Mensajes del SIAT</AlertTitle>
                        <AlertDescription>{invoice.siat_mensajes}</AlertDescription>
                      </Alert>
                    )}
                    {invoice.motivo_anulacion !== null &&
                      invoice.motivo_anulacion !== undefined && (
                        <p className="text-sm text-muted-foreground">
                          Motivo de anulación registrado ante el SIAT.
                        </p>
                      )}
                  </TabsContent>
                </ScrollArea>
              </Tabs>

              <div className="flex items-center gap-2 border-t bg-background p-3">
                {(invoice.status === "ACCEPTED" || invoice.status === "SENT") && (
                  <Button size="sm" variant="outline" render={<a href={host.getInvoicePdfUrl(invoice.id)} download />}>
                    <Download data-icon="inline-start" />
                    Descargar PDF
                  </Button>
                )}
                {(invoice.status === "ACCEPTED" || invoice.status === "OBSERVED") && (
                  <Button size="sm" variant="destructive" onClick={() => setAnnulOpen(true)}>
                    Anular…
                  </Button>
                )}
                {invoice.status === "REJECTED" && (
                  <Button size="sm" onClick={() => navigate("/invoices/new")}>
                    Crear corrección
                  </Button>
                )}
                {invoice.status === "CANCELLED" && (
                  <Button size="sm" variant="outline" onClick={() => setRevertOpen(true)}>
                    Revertir anulación
                  </Button>
                )}
                {(invoice.status === "PENDING" || invoice.status === "SENDING") && (
                  <p className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Spinner className="size-4" />
                    Enviando al SIAT…
                  </p>
                )}
                {invoice.status === "OFFLINE" && (
                  <p className="text-sm text-muted-foreground">
                    Se enviará automáticamente cuando el SIAT vuelva a estar disponible.
                  </p>
                )}
              </div>
            </>
          )}
        </SheetContent>
      </Sheet>

      <AnnulDialog
        invoiceId={invoice?.id ?? null}
        invoiceNumber={invoice?.invoice_number ?? null}
        open={annulOpen}
        onOpenChange={setAnnulOpen}
      />

      <AlertDialog open={revertOpen} onOpenChange={setRevertOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>¿Revertir la anulación?</AlertDialogTitle>
            <AlertDialogDescription>
              La factura {invoice?.invoice_number} volverá a tener validez oficial
              ante el SIAT.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancelar</AlertDialogCancel>
            <AlertDialogAction
              disabled={revertMutation.isPending}
              onClick={(event) => {
                event.preventDefault()
                revertMutation.mutate()
              }}
            >
              Revertir anulación
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

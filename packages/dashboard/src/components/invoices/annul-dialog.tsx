import { useState } from "react"
import { useDashboardHost } from "../../host-context"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { ApiError } from "../../host"
import { MOTIVOS_ANULACION } from "../../lib/invoice-status"
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
import { Label } from "../../components/ui/label"
import { RadioGroup, RadioGroupItem } from "../../components/ui/radio-group"

interface AnnulDialogProps {
  invoiceId: string | null
  invoiceNumber: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AnnulDialog({
  invoiceId,
  invoiceNumber,
  open,
  onOpenChange,
}: AnnulDialogProps) {
  const host = useDashboardHost()
  const [motivo, setMotivo] = useState<string>("90")
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: () => host.annulInvoice(invoiceId!, Number(motivo)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["invoices"] })
      toast.success(`Factura ${invoiceNumber} anulada ante el SIAT`)
      onOpenChange(false)
    },
    onError: (error) => {
      toast.error(
        error instanceof ApiError
          ? error.message
          : "No se pudo anular la factura"
      )
    },
  })

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>¿Anular la factura {invoiceNumber}?</AlertDialogTitle>
          <AlertDialogDescription>
            La factura quedará anulada oficialmente ante el SIAT. Esta acción se
            informa a Impuestos.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <RadioGroup value={motivo} onValueChange={setMotivo} className="gap-2">
          {MOTIVOS_ANULACION.map((m) => (
            <Label
              key={m.codigo}
              className="flex cursor-pointer items-start gap-2 rounded-md border p-3 font-normal has-[button[data-state=checked]]:border-ring"
            >
              <RadioGroupItem value={String(m.codigo)} className="mt-0.5" />
              <span>{m.label}</span>
            </Label>
          ))}
        </RadioGroup>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancelar</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={mutation.isPending}
            onClick={(event) => {
              event.preventDefault()
              mutation.mutate()
            }}
          >
            Anular definitivamente
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

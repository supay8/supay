import type { InvoiceStatus } from "@/lib/types"

export const TRANSIENT_STATUSES: InvoiceStatus[] = ["PENDING", "SENDING"]

export const INVOICE_STATUS_LABELS: Record<InvoiceStatus, string> = {
  PENDING: "Borrador",
  SENDING: "Enviando",
  SENT: "Enviada",
  ACCEPTED: "Aceptada",
  REJECTED: "Rechazada",
  OBSERVED: "Observada",
  OFFLINE: "En contingencia",
  CANCELLED: "Anulada",
}

export const MOTIVOS_ANULACION = [
  { codigo: 90, label: "Error en la emisión (digitación)" },
  { codigo: 91, label: "Interrupción o falla técnica" },
  { codigo: 92, label: "Factura emitida por error" },
  { codigo: 93, label: "Devolución total / nota de crédito" },
] as const

export const PLACEHOLDER_COMPANY_ID = "00000000-0000-0000-0000-000000000000"

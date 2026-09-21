import type { ReactNode } from "react"
import { Ban } from "lucide-react"
import { Spinner } from "../../components/ui/spinner"
import { INVOICE_STATUS_LABELS } from "../../lib/invoice-status"
import type { InvoiceStatus } from "../../lib/types"
import { cn } from "../../lib/utils"

const DOT_COLORS: Record<InvoiceStatus, string> = {
  ACCEPTED: "bg-success",
  SENDING: "bg-blue-500",
  SENT: "bg-blue-400",
  PENDING: "bg-neutral-300 dark:bg-neutral-600",
  REJECTED: "bg-destructive",
  OBSERVED: "bg-warning/60",
  OFFLINE: "bg-warning",
  CANCELLED: "bg-neutral-400 dark:bg-neutral-600",
}

interface StatusConfig {
  label: string
  icon?: ReactNode
}

const STATUS_CONFIG: Record<InvoiceStatus, StatusConfig> = {
  ACCEPTED: { label: INVOICE_STATUS_LABELS.ACCEPTED },
  SENDING: { label: INVOICE_STATUS_LABELS.SENDING },
  SENT: { label: INVOICE_STATUS_LABELS.SENT },
  PENDING: { label: INVOICE_STATUS_LABELS.PENDING },
  REJECTED: { label: INVOICE_STATUS_LABELS.REJECTED },
  OBSERVED: { label: INVOICE_STATUS_LABELS.OBSERVED },
  OFFLINE: { label: INVOICE_STATUS_LABELS.OFFLINE },
  CANCELLED: {
    label: INVOICE_STATUS_LABELS.CANCELLED,
    icon: <Ban data-icon="inline-start" />,
  },
}

export function StatusBadge({
  status,
  className,
}: {
  status: InvoiceStatus
  className?: string
}) {
  const config = STATUS_CONFIG[status]
  return (
    <span
      className={cn(
        "text-muted-foreground inline-flex items-center gap-1.5 whitespace-nowrap text-[13px] font-medium",
        status === "CANCELLED" && "line-through decoration-neutral-400",
        className
      )}
    >
      {status === "SENDING" ? (
        <Spinner className="size-3" />
      ) : (
        <span
          className={cn("size-1.5 shrink-0 rounded-full", DOT_COLORS[status])}
        />
      )}
      {config.label}
    </span>
  )
}

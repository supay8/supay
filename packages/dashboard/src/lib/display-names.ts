import type { Branch, Customer, Invoice, PointOfSale } from "./types"

export function padCode(n: number | null | undefined, len = 2): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "—"
  return String(n).padStart(len, "0")
}

export function formatBranch(branch: Pick<Branch, "name" | "codigo_sucursal">): string {
  return `${branch.name} (Suc ${padCode(branch.codigo_sucursal)})`
}

export function formatPos(
  pos: Pick<PointOfSale, "description" | "codigo_sucursal" | "codigo_punto_venta" | "branch_id">,
  branches: Pick<Branch, "id" | "name" | "codigo_sucursal">[] = [],
): string {
  const branch = branches.find((b) => b.id === pos.branch_id)
  const branchPart = branch
    ? branch.name
    : pos.branch_id
      ? "Sucursal desconocida"
      : `Suc ${padCode(pos.codigo_sucursal)}`
  return `${pos.description} · ${branchPart} · Pos ${padCode(pos.codigo_punto_venta)}`
}

export function formatPosShort(
  pos: Pick<PointOfSale, "description" | "codigo_sucursal" | "codigo_punto_venta">,
): string {
  return `${pos.description} · Suc ${padCode(pos.codigo_sucursal)} · Pos ${padCode(pos.codigo_punto_venta)}`
}

export function formatCustomer(
  customer: Pick<Customer, "name" | "document_type" | "document_number">,
): string {
  return `${customer.name} · ${customer.document_type?.toUpperCase()} ${customer.document_number}`
}

export function formatInvoiceTitle(invoice: Pick<Invoice, "invoice_number" | "total">): string {
  return `N° ${String(invoice.invoice_number).padStart(6, "0")}`
}

export function formatCompanyTitle(businessName: string, nit: string): string {
  return `${businessName} · NIT ${nit}`
}

export function certDisplayName(cert: { name?: string | null; subject?: string | null; created_at?: string }): string {
  if (cert.name?.trim()) return cert.name.trim()
  if (cert.subject?.trim()) return cert.subject.trim()
  return `Certificado del ${cert.created_at ? new Date(cert.created_at).toLocaleDateString("es-BO") : "NIT"}`
}

export function certStatusLabel(status: string, isActive: boolean): string {
  if (isActive) return "Activo"
  const s = (status || "").toLowerCase()
  if (s.includes("revok")) return "Revocado"
  if (s.includes("expir")) return "Expirado"
  return status || "Inactivo"
}

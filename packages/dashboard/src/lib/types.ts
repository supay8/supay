export const INVOICE_STATUSES = [
  "PENDING",
  "SENDING",
  "SENT",
  "ACCEPTED",
  "REJECTED",
  "OBSERVED",
  "OFFLINE",
  "CANCELLED",
] as const

export type InvoiceStatus = (typeof INVOICE_STATUSES)[number]

export type SiatEnvironment = "PILOTO" | "PRODUCCION"

export interface Company {
  id: string
  nit: string
  business_name: string
  codigo_sistema: string
  ambiente: SiatEnvironment
  municipio?: string
  direccion?: string
  telefono?: string
  codigo_actividad?: string | null
  created_at: string
  updated_at: string
}

export interface Branch {
  id: string
  company_id: string
  codigo_sucursal: number
  name: string
  address: string
  active: boolean
  created_at: string
}

export interface PointOfSale {
  id: string
  company_id: string
  branch_id?: string | null
  codigo_sucursal: number
  codigo_punto_venta: number
  name?: string | null
  description: string
  cuis?: string | null
  cuis_created_at?: string | null
  is_active: boolean
  siat_code?: number | null
  status?: string
  tipo_punto_venta?: number | null
  siat_transaccion: boolean
  siat_registered_at?: string | null
  siat_error?: string | null
  created_at: string
}

export interface Customer {
  id: string
  company_id: string
  document_type: string
  document_number: string
  complement?: string | null
  name: string
  created_at: string
}

export interface ProductMapping {
  id: string
  product_id: string
  codigo_producto_sin: number
  codigo_actividad: string
  codigo_documento_sector: number
  unidad_medida: number
  is_default: boolean
  active: boolean
  synced_at: string
}

export interface Product {
  id: string
  company_id: string
  sku: string
  name: string
  active: boolean
  mappings?: ProductMapping[]
  created_at: string
  updated_at: string
}

export interface InvoiceItem {
  id: string
  invoice_id: string
  product_id?: string | null
  code: string
  description: string
  codigo_actividad?: string | null
  codigo_producto_sin?: string | null
  unit_code?: number | null
  quantity: number
  unit_price: number
  discount: number
  subtotal: number
}

export interface Invoice {
  id: string
  company_id: string
  customer_id: string
  point_of_sale_id: string
  invoice_number: number
  cuf?: string | null
  emission_type: string
  issue_date: string
  subtotal: number
  discount: number
  total: number
  status: InvoiceStatus
  motivo_anulacion?: number | null
  fecha_anulacion?: string | null
  siat_reception_code?: string | null
  siat_mensajes?: string | null
  contingency_event_id?: string | null
  customer: Customer
  point_of_sale: PointOfSale
  items: InvoiceItem[]
}

export interface SectorFieldInfo {
  clave: string
  label: string
  tipo: "string" | "int" | "float" | "fecha"
  requerido: boolean
  ejemplo?: string
}

export interface SectorInfo {
  codigo: number
  label: string
  soportado: boolean
  habilitado: boolean
  ejemplo?: string
  campos?: SectorFieldInfo[]
}

export interface Paginated<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface FieldErrors {
  [field: string]: string[]
}

export type ApiErrorCode =
  | "VALIDATION_ERROR"
  | "SIAT_REJECTED"
  | "SIAT_UNAVAILABLE"
  | "NOT_FOUND"
  | "CONFLICT"
  | "BAD_REQUEST"
  | "INTERNAL"

export interface InvoiceListParams {
  point_of_sale_id?: string
  status?: InvoiceStatus[]
  from?: string
  to?: string
  q?: string
  limit?: number
  offset?: number
}

export interface DraftItemInput {
  product_id?: string
  code?: string
  description: string
  quantity: number
  unit_price: number
  discount?: number
}

export interface DraftInput {
  customer_id: string
  point_of_sale_id: string
  codigo_documento_sector: number
  codigo_metodo_pago?: number
  codigo_moneda?: number
  items: DraftItemInput[]
  sector_data?: Record<string, unknown>
}

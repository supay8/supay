import type {
  Branch,
  Company,
  Customer,
  DraftInput,
  Invoice,
  InvoiceListParams,
  Paginated,
  PointOfSale,
  Product,
  SectorInfo,
} from "./lib/types"

export interface ProductMappingInput {
  codigo_producto_sin: number
  codigo_actividad: string
  codigo_documento_sector: number
  unidad_medida: number
  is_default?: boolean
}

export class HostError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, string[]>

  constructor(
    status: number,
    code: string,
    message: string,
    details?: Record<string, string[]>
  ) {
    super(message)
    this.name = "HostError"
    this.status = status
    this.code = code
    this.details = details
  }
}

// Compat alias para que código migrado que esperaba ApiError siga funcionando
export const ApiError = HostError
export type ApiError = HostError

export interface DashboardHost {
  // Company
  getCompany(id: string): Promise<Company>
  getCompanyByNit(nit: string): Promise<Company>
  createCompany(payload: Partial<Company>): Promise<Company>
  updateCompany(id: string, payload: Partial<Company>): Promise<Company>
  setupCompany(companyId: string, payload?: Record<string, unknown>): Promise<Record<string, unknown>>

  // Branches
  listBranches(companyId: string): Promise<Paginated<Branch>>
  getBranch(id: string): Promise<Branch>
  createBranch(payload: Partial<Branch>): Promise<Branch>
  updateBranch(id: string, payload: Partial<Branch>): Promise<Branch>
  deleteBranch(id: string): Promise<void>

  // Points of sale
  listPointsOfSale(companyId: string): Promise<Paginated<PointOfSale>>
  getPointOfSale(id: string): Promise<PointOfSale>
  createPointOfSale(payload: Partial<PointOfSale>): Promise<PointOfSale>
  updatePointOfSale(id: string, payload: Partial<PointOfSale>): Promise<PointOfSale>
  deletePointOfSale(id: string): Promise<void>

  // Customers
  listCustomers(companyId: string): Promise<Paginated<Customer>>
  getCustomer(id: string): Promise<Customer>
  createCustomer(payload: Partial<Customer>): Promise<Customer>

  // Products
  listProducts(companyId: string): Promise<Paginated<Product>>
  createProduct(payload: Partial<Product>): Promise<Product>
  addProductMapping(productId: string, payload: Partial<ProductMappingInput>): Promise<void>

  // Invoices
  listInvoices(params: InvoiceListParams): Promise<Paginated<Invoice>>
  getInvoice(id: string): Promise<Invoice>
  listSectores(): Promise<SectorInfo[]>
  createDraft(payload: DraftInput): Promise<Invoice>
  emitInvoice(id: string): Promise<Invoice>
  annulInvoice(id: string, motivoAnulacion: number): Promise<Invoice>
  revertAnnul(id: string): Promise<Invoice>
  getSiatStatus(id: string): Promise<Record<string, unknown>>
  getInvoicePdfUrl(id: string): string

  // Cloud optional capabilities - extensible
  getUsage?(): Promise<unknown>
  getBilling?(): Promise<unknown>

  // Allow host to expose arbitrary extra capabilities without changing core
  capabilities?: Record<string, unknown>
}

export type HostCapabilities = Record<string, unknown>

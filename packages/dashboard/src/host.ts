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
  // Auth (sesión humana, JWT Bearer)
  signup(name: string, email: string, password: string): Promise<AuthResult>
  login(email: string, password: string): Promise<AuthResult>
  getCurrentUser(): Promise<AuthUser>
  listMyCompanies(): Promise<UserCompany[]>
  createMyCompany(payload: CreateMyCompanyInput): Promise<Company>
  logout(): void

  // Company
  getCompanyByNit(nit: string): Promise<Company>
  createCompany(payload: Partial<Company>): Promise<Company>
  updateCompany(id: string, payload: Partial<Company>): Promise<Company>
  setupPointOfSale(pointOfSaleId: string): Promise<Record<string, unknown>>

  // API Keys
  bootstrapCompany(payload: Partial<Company>): Promise<BootstrapCompanyResponse>
  listApiKeys(companyId: string): Promise<ApiKeyItem[]>
  createApiKey(companyId: string, name: string): Promise<CreateApiKeyResponse>
  revokeApiKey(companyId: string, keyId: string): Promise<void>

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

  // Gestión de credenciales del host (self-hosted las implementa)
  setApiKey?(key: string): void
  getApiKey?(): string
  setStoredCompanyId?(id: string): void
  getStoredCompanyId?(): string

  // Empresa activa para X-Company-ID (sesión JWT sin X-API-Key)
  setActiveCompanyId?(id: string): void
  getActiveCompanyId?(): string

  // Allow host to expose arbitrary extra capabilities without changing core
  capabilities?: Record<string, unknown>
}

export interface AuthUser {
  id: string
  email: string
  name: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface AuthResult {
  user: AuthUser
  access_token: string
  token_type: string
  expires_at: string
}

export interface UserCompany {
  company: Company
  role: string
}

export interface CreateMyCompanyInput {
  nit: string
  business_name: string
  municipio?: string
  direccion?: string
  telefono?: string
}

export interface BootstrapCompanyResponse {  company: Company
  api_key: string
  key_prefix: string
  key_id: string
}

export interface ApiKeyItem {
  id: string
  key_prefix: string
  name: string
  is_active: boolean
  last_used_at: string | null
  created_at: string
}

export interface CreateApiKeyResponse {
  api_key: string
  key_prefix: string
  id: string
  name: string
  created_at: string
}

export type HostCapabilities = Record<string, unknown>

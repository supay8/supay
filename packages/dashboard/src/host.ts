import type {
  BatchSendInput,
  Branch,
  CatalogReadiness,
  Company,
  Customer,
  DraftInput,
  EmissionBootstrap,
  Invoice,
  InvoiceListParams,
  InvoicePreview,
  Paginated,
  PointOfSale,
  Product,
  SectorInfo,
  SectorPerfil,
  SiatOperationResult,
  SinProduct,
  V1InvoiceInput,
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

export interface AuthHost {
  signup(name: string, email: string, password: string): Promise<AuthResult>
  login(email: string, password: string): Promise<AuthResult>
  getCurrentUser(): Promise<AuthUser>
  listMyCompanies(): Promise<UserCompany[]>
  createMyCompany(payload: CreateMyCompanyInput): Promise<Company>
  logout(): void
}

export interface CompanyAdminHost {
  getCompanyByNit(nit: string): Promise<Company>
  createCompany(payload: Partial<Company>): Promise<Company>
  updateCompany(id: string, payload: Partial<Company>): Promise<Company>
  setupPointOfSale(pointOfSaleId: string): Promise<Record<string, unknown>>
}

export interface InvoiceHost {
  listInvoices(params: InvoiceListParams): Promise<Paginated<Invoice>>
  getInvoice(id: string): Promise<Invoice>
  listSectores(companyId?: string): Promise<SectorInfo[]>
  createDraft(payload: DraftInput): Promise<Invoice>
  emitInvoice(id: string): Promise<Invoice>
  annulInvoice(id: string, motivoAnulacion: number): Promise<Invoice>
  revertAnnul(id: string): Promise<Invoice>
  getSiatStatus(id: string): Promise<Record<string, unknown>>
  getInvoicePdfUrl(id: string): string
  /** Contrato v1: validación sin efectos (POST /v1/invoices/preview). */
  previewInvoice?(payload: V1InvoiceInput): Promise<InvoicePreview>
  /** Contrato v1: crear + emitir atómico (POST /v1/invoices/emit). */
  emitInvoiceDirect?(payload: V1InvoiceInput): Promise<Invoice>
  getInvoiceXmlUrl?(id: string): string
}

export interface CatalogHost {
  listSinProducts?(companyId: string, query?: string, limit?: number): Promise<Paginated<SinProduct>>
  getCatalogReadiness?(companyId: string, pointOfSaleId?: string): Promise<CatalogReadiness>
  getEmissionBootstrap?(companyId: string, codigoActividad?: string): Promise<EmissionBootstrap>
  listSectorPerfiles?(): Promise<SectorPerfil[]>
}

export interface SiatOpsHost {
  requestCuis?(pointOfSaleId: string): Promise<SiatOperationResult>
  requestCufd?(pointOfSaleId: string): Promise<SiatOperationResult>
  sincronizar?(pointOfSaleId: string, operation?: string): Promise<SiatOperationResult>
  registrarEventoSignificativo?(pointOfSaleId: string, payload?: Record<string, unknown>): Promise<SiatOperationResult>
  enviarPaquete?(pointOfSaleId: string, input: BatchSendInput): Promise<SiatOperationResult>
  validarPaquete?(batchId: string): Promise<SiatOperationResult>
  enviarMasiva?(pointOfSaleId: string, input: BatchSendInput): Promise<SiatOperationResult>
  validarMasiva?(batchId: string): Promise<SiatOperationResult>
  enviarCompras?(pointOfSaleId: string, input: BatchSendInput): Promise<SiatOperationResult>
}

export interface DashboardHost
  extends AuthHost,
    CompanyAdminHost,
    InvoiceHost,
    CatalogHost,
    SiatOpsHost {
  // Auth (sesión humana, JWT Bearer) — heredado de AuthHost, se redeclara
  // por compatibilidad con hosts existentes; no duplicar en código nuevo.

  // API Keys
  bootstrapCompany(payload: Partial<Company>): Promise<BootstrapCompanyResponse>
  listApiKeys(companyId: string): Promise<ApiKeyItem[]>
  createApiKey(companyId: string, name: string): Promise<CreateApiKeyResponse>
  revokeApiKey(companyId: string, keyId: string): Promise<void>

  // Certificates (firma digital .p12 por empresa/NIT)
  listCertificates(companyId: string): Promise<CertificateItem[]>
  getActiveCertificate(companyId: string): Promise<CertificateItem | null>
  uploadCertificate(companyId: string, input: UploadCertificateInput): Promise<CertificateItem>
  revokeCertificate(companyId: string, certId: string): Promise<void>

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

  // Customers (solo lectura en backend: se crean implícito vía facturas)
  listCustomers(companyId: string): Promise<Paginated<Customer>>
  getCustomer(id: string): Promise<Customer>
  /** @deprecated El backend no expone POST /customers; crear vía facturación. */
  createCustomer(payload: Partial<Customer>): Promise<Customer>

  // Products (backend expone catálogo SIN, no CRUD local)
  listProducts(companyId: string): Promise<Paginated<Product>>
  /** @deprecated Sin endpoint backend; usar catálogo SIN. */
  createProduct(payload: Partial<Product>): Promise<Product>
  /** @deprecated Sin endpoint backend. */
  addProductMapping(productId: string, payload: Partial<ProductMappingInput>): Promise<void>

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

export interface CertificateItem {
  id: string
  name: string
  type: string
  status: string
  is_active: boolean
  not_before?: string | null
  not_after?: string | null
  issuer?: string | null
  subject?: string | null
  thumbprint?: string | null
  created_at: string
}

export interface UploadCertificateInput {
  file: File
  name?: string
  password: string
}

export type HostCapabilities = Record<string, unknown>

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
} from "../lib/types"
import { HostError, type DashboardHost, type ProductMappingInput } from "../host"

type QueryParams = Record<string, string | number | undefined>

function buildUrl(baseUrl: string, path: string, query?: QueryParams): string {
  const url = new URL(path, baseUrl)
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined) {
        url.searchParams.set(key, String(value))
      }
    }
  }
  return url.toString()
}

async function toHostError(response: Response): Promise<HostError> {
  let code = "UNKNOWN"
  let message = response.statusText
  let details: Record<string, string[]> | undefined
  try {
    const body = await response.json()
    if (body?.error?.code) code = body.error.code
    if (body?.error?.message) message = body.error.message
    if (body?.error?.details) details = body.error.details
  } catch {
    /* cuerpo no JSON */
  }
  return new HostError(response.status, code, message, details)
}

export interface SelfHostedHostOptions {
  baseUrl?: string
  apiKey?: string
  fetchImpl?: typeof fetch
}

export function createSelfHostedHost(options: SelfHostedHostOptions = {}): DashboardHost {
  const baseUrl = options.baseUrl ?? "http://localhost:8080"
  const apiKey = options.apiKey ?? ""
  const fetchImpl = options.fetchImpl ?? fetch

  async function request<T>(
    path: string,
    opts: { method?: string; body?: unknown; query?: QueryParams } = {}
  ): Promise<T> {
    const headers: Record<string, string> = {}
    if (opts.body !== undefined) headers["Content-Type"] = "application/json"
    if (apiKey) headers["X-API-Key"] = apiKey

    const response = await fetchImpl(buildUrl(baseUrl, path, opts.query), {
      method: opts.method ?? "GET",
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    })

    if (!response.ok) throw await toHostError(response)
    if (response.status === 204) return undefined as T
    return (await response.json()) as T
  }

  return {
    // Company
    getCompany: (id) => request<Company>(`/companies/${id}`),
    getCompanyByNit: (nit) => request<Company>("/companies", { query: { nit } }),
    createCompany: (payload) => request<Company>("/companies", { method: "POST", body: payload }),
    updateCompany: (id, payload) => request<Company>(`/companies/${id}`, { method: "PATCH", body: payload }),
    setupCompany: (companyId, payload) =>
      request<Record<string, unknown>>(`/companies/${companyId}/setup`, {
        method: "POST",
        body: payload,
      }),

    // Branches
    listBranches: (companyId) => request<Paginated<Branch>>("/branches", { query: { company_id: companyId } }),
    getBranch: (id) => request<Branch>(`/branches/${id}`),
    createBranch: (payload) => request<Branch>("/branches", { method: "POST", body: payload }),
    updateBranch: (id, payload) => request<Branch>(`/branches/${id}`, { method: "PUT", body: payload }),
    deleteBranch: (id) => request<void>(`/branches/${id}`, { method: "DELETE" }),

    // Points of sale
    listPointsOfSale: (companyId) =>
      request<Paginated<PointOfSale>>("/point-of-sale", { query: { company_id: companyId } }),
    getPointOfSale: (id) => request<PointOfSale>(`/point-of-sale/${id}`),
    createPointOfSale: (payload) => request<PointOfSale>("/point-of-sale", { method: "POST", body: payload }),
    updatePointOfSale: (id, payload) =>
      request<PointOfSale>(`/point-of-sale/${id}`, { method: "PATCH", body: payload }),
    deletePointOfSale: (id) => request<void>(`/point-of-sale/${id}`, { method: "DELETE" }),

    // Customers
    listCustomers: (companyId) =>
      request<Paginated<Customer>>("/customers", { query: { company_id: companyId } }),
    getCustomer: (id) => request<Customer>(`/customers/${id}`),
    createCustomer: (payload) => request<Customer>("/customers", { method: "POST", body: payload }),

    // Products
    listProducts: (companyId) =>
      request<Paginated<Product>>("/products", { query: { company_id: companyId } }),
    createProduct: (payload) => request<Product>("/products", { method: "POST", body: payload }),
    addProductMapping: (productId, payload: Partial<ProductMappingInput>) =>
      request<void>(`/products/${productId}/mappings`, { method: "POST", body: payload }),

    // Invoices
    listInvoices: (params: InvoiceListParams) =>
      request<Paginated<Invoice>>("/invoices", {
        query: {
          point_of_sale_id: params.point_of_sale_id,
          status: params.status?.join(","),
          from: params.from,
          to: params.to,
          q: params.q,
          limit: params.limit,
          offset: params.offset,
        },
      }),
    getInvoice: (id) => request<Invoice>(`/invoices/${id}`),
    listSectores: () => request<SectorInfo[]>("/invoices/sectores"),
    createDraft: (payload: DraftInput) => request<Invoice>("/invoices", { method: "POST", body: payload }),
    emitInvoice: (id) => request<Invoice>(`/invoices/${id}/emit`, { method: "POST" }),
    annulInvoice: (id, motivoAnulacion) =>
      request<Invoice>(`/invoices/${id}/annul`, {
        method: "POST",
        body: { motivo_anulacion: motivoAnulacion },
      }),
    revertAnnul: (id) => request<Invoice>(`/invoices/${id}/annul/revert`, { method: "POST" }),
    getSiatStatus: (id) => request<Record<string, unknown>>(`/invoices/${id}/siat-status`),
    getInvoicePdfUrl: (id) => buildUrl(baseUrl, `/invoices/${id}/pdf`),
  }
}

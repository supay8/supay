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
} from "./types"

const baseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080"
const apiKey = import.meta.env.VITE_API_KEY ?? ""

export class ApiError extends Error {
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
    this.name = "ApiError"
    this.status = status
    this.code = code
    this.details = details
  }
}

type QueryParams = Record<string, string | number | undefined>

function buildUrl(path: string, query?: QueryParams): string {
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

async function toApiError(response: Response): Promise<ApiError> {
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
  return new ApiError(response.status, code, message, details)
}

async function request<T>(
  path: string,
  options: { method?: string; body?: unknown; query?: QueryParams } = {}
): Promise<T> {
  const headers: Record<string, string> = {}
  if (options.body !== undefined) headers["Content-Type"] = "application/json"
  if (apiKey) headers["X-API-Key"] = apiKey

  const response = await fetch(buildUrl(path, options.query), {
    method: options.method ?? "GET",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  })

  if (!response.ok) throw await toApiError(response)
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

export const api = {
  companies: {
    getById: (id: string) => request<Company>(`/companies/${id}`),
    getByNit: (nit: string) =>
      request<Company>("/companies", { query: { nit } }),
    create: (payload: Partial<Company>) =>
      request<Company>("/companies", { method: "POST", body: payload }),
    update: (id: string, payload: Partial<Company>) =>
      request<Company>(`/companies/${id}`, { method: "PATCH", body: payload }),
    setup: (companyId: string, payload?: Record<string, unknown>) =>
      request<Record<string, unknown>>(`/companies/${companyId}/setup`, {
        method: "POST",
        body: payload,
      }),
  },
  branches: {
    list: (companyId: string) =>
      request<Paginated<Branch>>("/branches", { query: { company_id: companyId } }),
    get: (id: string) => request<Branch>(`/branches/${id}`),
    create: (payload: Partial<Branch>) =>
      request<Branch>("/branches", { method: "POST", body: payload }),
    update: (id: string, payload: Partial<Branch>) =>
      request<Branch>(`/branches/${id}`, { method: "PUT", body: payload }),
    remove: (id: string) =>
      request<void>(`/branches/${id}`, { method: "DELETE" }),
  },
  pointOfSale: {
    list: (companyId: string) =>
      request<Paginated<PointOfSale>>("/point-of-sale", {
        query: { company_id: companyId },
      }),
    get: (id: string) => request<PointOfSale>(`/point-of-sale/${id}`),
    create: (payload: Partial<PointOfSale>) =>
      request<PointOfSale>("/point-of-sale", { method: "POST", body: payload }),
    update: (id: string, payload: Partial<PointOfSale>) =>
      request<PointOfSale>(`/point-of-sale/${id}`, {
        method: "PATCH",
        body: payload,
      }),
    remove: (id: string) =>
      request<void>(`/point-of-sale/${id}`, { method: "DELETE" }),
  },
  customers: {
    list: (companyId: string) =>
      request<Paginated<Customer>>("/customers", {
        query: { company_id: companyId },
      }),
    get: (id: string) => request<Customer>(`/customers/${id}`),
    create: (payload: Partial<Customer>) =>
      request<Customer>("/customers", { method: "POST", body: payload }),
  },
  products: {
    list: (companyId: string) =>
      request<Paginated<Product>>("/products", {
        query: { company_id: companyId },
      }),
    create: (payload: Partial<Product>) =>
      request<Product>("/products", { method: "POST", body: payload }),
    addMapping: (productId: string, payload: Partial<ProductMappingInput>) =>
      request<void>(`/products/${productId}/mappings`, {
        method: "POST",
        body: payload,
      }),
  },
  invoices: {
    list: (params: InvoiceListParams) =>
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
    get: (id: string) => request<Invoice>(`/invoices/${id}`),
    sectores: () => request<SectorInfo[]>("/invoices/sectores"),
    createDraft: (payload: DraftInput) =>
      request<Invoice>("/invoices", { method: "POST", body: payload }),
    emit: (id: string) =>
      request<Invoice>(`/invoices/${id}/emit`, { method: "POST" }),
    annul: (id: string, motivoAnulacion: number) =>
      request<Invoice>(`/invoices/${id}/annul`, {
        method: "POST",
        body: { motivo_anulacion: motivoAnulacion },
      }),
    revertAnnul: (id: string) =>
      request<Invoice>(`/invoices/${id}/annul/revert`, { method: "POST" }),
    siatStatus: (id: string) =>
      request<Record<string, unknown>>(`/invoices/${id}/siat-status`),
    pdfUrl: (id: string) => buildUrl(`/invoices/${id}/pdf`),
  },
}

interface ProductMappingInput {
  codigo_producto_sin: number
  codigo_actividad: string
  codigo_documento_sector: number
  unidad_medida: number
  is_default?: boolean
}

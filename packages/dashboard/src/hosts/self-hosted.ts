import type {
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
} from "../lib/types"
import { HostError, type AuthResult, type AuthUser, type CreateMyCompanyInput, type DashboardHost, type UserCompany } from "../host"

type QueryParams = Record<string, string | number | boolean | undefined>

const STORAGE_KEYS = {
    API_KEY: "supay_api_key",
    COMPANY_ID: "supay_company_id",
    ACCESS_TOKEN: "supay_access_token",
} as const

/** Prefijo del contrato público estable. Todo el dashboard debe usar /v1. */
const API_V1 = "/v1"
/** Puerto por defecto del backend (config PORT=8081). */
const DEFAULT_BASE_URL = "http://localhost:8081"

const isBrowser = typeof window !== "undefined"

function getStorage(key: string): string {
    return isBrowser ? localStorage.getItem(key) ?? "" : ""
}

function setStorage(key: string, value: string): void {
    if (!isBrowser) return
    if (value) {
        localStorage.setItem(key, value)
    } else {
        localStorage.removeItem(key)
    }
}

function buildUrl(baseUrl: string, path: string, query?: QueryParams): string {
    const url = new URL(path, baseUrl)
    if (query) {
        for (const [key, value] of Object.entries(query)) {
            if (value !== undefined && value !== "") {
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

interface RequestOptions {
    method?: string
    body?: unknown
    form?: FormData
    query?: QueryParams
    requireAuth?: boolean
    idempotencyKey?: string
}

/**
 * ApiClient — Adapter sobre fetch con una sola responsabilidad: transporte.
 * Centraliza baseUrl, headers de tenant (X-API-Key / Bearer + X-Company-ID),
 * prefijo /v1 y mapeo de errores al envelope {error:{code,message}}.
 * Patrón Builder: los callers componen path + query + body sin concatenar strings.
 */
class ApiClient {
    constructor(
        private baseUrl: string,
        private getAuth: () => { apiKey: string; accessToken: string; companyId: string },
        private fetchImpl: typeof fetch,
    ) {}

    async request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
        const { apiKey, accessToken, companyId } = this.getAuth()
        const headers: Record<string, string> = {}

        if (opts.body !== undefined) {
            headers["Content-Type"] = "application/json"
        }
        if (opts.idempotencyKey) {
            headers["Idempotency-Key"] = opts.idempotencyKey.slice(0, 100)
        }
        if (opts.requireAuth !== false) {
            if (apiKey) headers["X-API-Key"] = apiKey
            if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`
            if (companyId) headers["X-Company-ID"] = companyId
        }

        const response = await this.fetchImpl(buildUrl(this.baseUrl, path, opts.query), {
            method: opts.method ?? (opts.form ? "POST" : "GET"),
            headers,
            body: opts.form ?? (opts.body !== undefined ? JSON.stringify(opts.body) : undefined),
        })

        if (!response.ok) throw await toHostError(response)
        if (response.status === 204) return undefined as T
        const text = await response.text()
        if (!text) return undefined as T
        return JSON.parse(text) as T
    }

    url(path: string, query?: QueryParams): string {
        return buildUrl(this.baseUrl, path, query)
    }
}

function unwrapItems<T>(res: Paginated<T> | { items?: T[] } | T[]): T[] {
    if (Array.isArray(res)) return res
    if (res && typeof res === "object" && "items" in res && Array.isArray((res as { items?: T[] }).items)) {
        return (res as { items: T[] }).items
    }
    return []
}

function toPaginated<T>(res: Paginated<T> | { items?: T[] } | T[] | { data?: T[] }): Paginated<T> {
    if (res && typeof res === "object" && "items" in res && Array.isArray((res as { items: T[] }).items)) {
        return res as Paginated<T>
    }
    if (res && typeof res === "object" && "data" in res && Array.isArray((res as { data: T[] }).data)) {
        const items = (res as { data: T[] }).data ?? []
        return { items, total: items.length, limit: items.length, offset: 0 }
    }
    const items = unwrapItems(res as Paginated<T>)
    return { items, total: items.length, limit: items.length, offset: 0 }
}

function notSupported(feature: string): HostError {
    return new HostError(
        405,
        "NOT_SUPPORTED",
        `${feature} no existe en el backend: el contrato real usa catálogos SIAT sincronizados.`,
    )
}

export interface SelfHostedHostOptions {
    baseUrl?: string
    apiKey?: string
    fetchImpl?: typeof fetch
}

/**
 * Factory del host self-hosted (patrón Factory + Strategy vía DashboardHost).
 * Todos los paths usan el contrato estable /v1.
 */
export function createSelfHostedHost(options: SelfHostedHostOptions = {}): DashboardHost {
    const baseUrl = options.baseUrl ?? DEFAULT_BASE_URL
    // `fetch` nativo exige el receptor `window`; al extraerlo sin bindear el
    // navegador lanza "Illegal invocation". Se envuelve en arrow function.
    const fetchImpl: typeof fetch =
        options.fetchImpl ?? ((url, init) => fetch(url, init))

    let apiKey = options.apiKey ?? getStorage(STORAGE_KEYS.API_KEY)
    let accessToken = getStorage(STORAGE_KEYS.ACCESS_TOKEN)
    let activeCompanyId = getStorage(STORAGE_KEYS.COMPANY_ID)

    const setApiKey = (key: string) => { apiKey = key; setStorage(STORAGE_KEYS.API_KEY, key) }
    const setAccessToken = (token: string) => { accessToken = token; setStorage(STORAGE_KEYS.ACCESS_TOKEN, token) }
    const setActiveCompanyId = (id: string) => { activeCompanyId = id; setStorage(STORAGE_KEYS.COMPANY_ID, id) }

    const client = new ApiClient(
        baseUrl,
        () => ({ apiKey, accessToken, companyId: activeCompanyId }),
        fetchImpl,
    )
    const v1 = (path: string) => `${API_V1}${path}`
    const request = <T>(path: string, opts?: RequestOptions) => client.request<T>(v1(path), { ...opts, requireAuth: true })
    const requestPublic = <T>(path: string, opts?: RequestOptions) => client.request<T>(v1(path), { ...opts, requireAuth: false })
    const requestForm = <T>(path: string, form: FormData) => client.request<T>(v1(path), { form, requireAuth: true })

    const siatResult = (raw: unknown): SiatOperationResult => {
        if (raw && typeof raw === "object") {
            const r = raw as Record<string, unknown>
            return {
                ok: true,
                reception_code: (r["reception_code"] ?? r["codigo_recepcion"] ?? null) as string | null,
                message: (r["message"] as string) ?? undefined,
                raw: r,
            }
        }
        return { ok: true, raw: {} }
    }

    return {
        // Auth (sesión humana, JWT Bearer)
        signup: async (name: string, email: string, password: string) => {
            const result = await requestPublic<AuthResult>("/auth/signup", {
                method: "POST",
                body: { name, email, password },
            })
            setAccessToken(result.access_token)
            return result
        },
        login: async (email: string, password: string) => {
            const result = await requestPublic<AuthResult>("/auth/login", {
                method: "POST",
                body: { email, password },
            })
            setAccessToken(result.access_token)
            return result
        },
        getCurrentUser: () => request<AuthUser>("/auth/me"),
        listMyCompanies: async () => {
            const res = await request<Paginated<UserCompany>>("/auth/companies")
            return res.items ?? []
        },
        createMyCompany: (payload: CreateMyCompanyInput) =>
            request<Company>("/auth/companies", { method: "POST", body: payload }),
        logout: () => {
            setAccessToken("")
        },

        // Company
        getCompanyByNit: (nit) => request<Company>("/companies", { query: { nit } }),
        createCompany: (payload) => request<Company>("/auth/companies", { method: "POST", body: payload }),
        updateCompany: (id, payload) => request<Company>(`/companies/${id}`, { method: "PATCH", body: payload }),
        setupPointOfSale: (pointOfSaleId) =>
            request<Record<string, unknown>>(`/siat/setup/${pointOfSaleId}`, {
                method: "POST",
                body: { point_of_sale_id: pointOfSaleId },
            }),

        // API Keys
        bootstrapCompany: (payload) => requestPublic<import("../host").BootstrapCompanyResponse>("/internal/companies", { method: "POST", body: payload }),
        listApiKeys: async (companyId) => {
            const res = await request<import("../host").ApiKeyItem[] | { items: import("../host").ApiKeyItem[] }>(`/companies/${companyId}/api-keys`)
            return Array.isArray(res) ? res : (res?.items ?? [])
        },
        createApiKey: (companyId, name) => request<import("../host").CreateApiKeyResponse>(`/companies/${companyId}/api-keys`, { method: "POST", body: { name } }),
        revokeApiKey: (companyId, keyId) => request<void>(`/companies/${companyId}/api-keys/${keyId}`, { method: "DELETE" }),

        // Certificates (firma digital .p12 por empresa)
        listCertificates: async (companyId) => {
            const res = await request<import("../host").CertificateItem[] | { data: import("../host").CertificateItem[] }>(`/companies/${companyId}/certificates`)
            return Array.isArray(res) ? res : (res?.data ?? [])
        },
        getActiveCertificate: async (companyId) => {
            try {
                const res = await request<import("../host").CertificateItem | { data: import("../host").CertificateItem }>(`/companies/${companyId}/certificates/active`)
                return (res && typeof res === "object" && "data" in res) ? res.data : res
            } catch (e) {
                if (e instanceof HostError && e.status === 404) return null
                throw e
            }
        },
        uploadCertificate: async (companyId, input) => {
            const form = new FormData()
            form.append("p12_file", input.file, input.file.name)
            if (input.name?.trim()) form.append("name", input.name.trim())
            form.append("p12_password", input.password)
            const res = await requestForm<import("../host").CertificateItem | { data: import("../host").CertificateItem }>(`/companies/${companyId}/certificates`, form)
            return (res && typeof res === "object" && "data" in res) ? res.data : res
        },
        revokeCertificate: (companyId, certId) => request<void>(`/companies/${companyId}/certificates/${certId}`, { method: "DELETE" }),

        // Branches
        listBranches: (companyId) => request<Paginated<Branch>>("/branches", { query: { company_id: companyId } }),
        getBranch: (id) => request<Branch>(`/branches/${id}`),
        createBranch: (payload) => request<Branch>("/branches", { method: "POST", body: payload }),
        updateBranch: (id, payload) => request<Branch>(`/branches/${id}`, { method: "PUT", body: payload }),
        deleteBranch: (id) => request<void>(`/branches/${id}`, { method: "DELETE" }),

        // Points of sale
        listPointsOfSale: (companyId) => request<Paginated<PointOfSale>>("/point-of-sales", { query: { company_id: companyId } }),
        getPointOfSale: (id) => request<PointOfSale>(`/point-of-sales/${id}`),
        createPointOfSale: (payload) => request<PointOfSale>("/point-of-sales", { method: "POST", body: payload }),
        updatePointOfSale: (id, payload) => request<PointOfSale>(`/point-of-sales/${id}`, { method: "PATCH", body: payload }),
        deletePointOfSale: (id) => request<void>(`/point-of-sales/${id}`, { method: "DELETE" }),

        // Customers — solo lectura (el backend crea al facturar)
        listCustomers: (companyId) => request<Paginated<Customer>>("/customers", { query: { company_id: companyId } }),
        getCustomer: (id) => request<Customer>(`/customers/${id}`),
        createCustomer: () => { throw notSupported("POST /customers") },

        // Products — catálogo SIN sincronizado, no CRUD local.
        // listProducts actúa como Adapter: expone SIN con forma Product para
        // no romper combobox existentes; usar listSinProducts para el contrato real.
        listProducts: async (companyId) => {
            const sin = await request<Paginated<SinProduct> | { data?: SinProduct[] } | SinProduct[]>(
                `/companies/${companyId}/catalogs/productos-sin`,
                { query: { limit: 50 } },
            ).catch((): Paginated<SinProduct> => ({ items: [], total: 0, limit: 0, offset: 0 }))
            const items = toPaginated<SinProduct>(sin).items
            const adapted: Product[] = items.map((s, i) => ({
                id: s.id ?? `sin-${s.codigo}-${i}`,
                company_id: companyId,
                sku: String(s.codigo),
                name: s.descripcion,
                active: true,
                mappings: [],
                created_at: new Date(0).toISOString(),
                updated_at: new Date(0).toISOString(),
            }))
            return { items: adapted, total: adapted.length, limit: 50, offset: 0 }
        },
        createProduct: () => { throw notSupported("POST /products") },
        addProductMapping: () => { throw notSupported("POST /products/{id}/mappings") },

        // Invoices (contrato estable /v1)
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
        listSectores: async (companyId?: string) => {
            const res = await request<SectorInfo[] | { data?: SectorInfo[] }>("/invoices/sectores", {
                query: { company_id: companyId },
            })
            if (Array.isArray(res)) return res
            if (res && typeof res === "object" && "data" in res && Array.isArray(res.data)) return res.data
            return []
        },
        createDraft: (payload: DraftInput) => request<Invoice>("/invoices", { method: "POST", body: payload }),
        emitInvoice: (id) => request<Invoice>(`/invoices/${id}/emit`, { method: "POST" }),
        previewInvoice: (payload: V1InvoiceInput) =>
            request<InvoicePreview>("/invoices/preview", { method: "POST", body: payload }),
        emitInvoiceDirect: (payload: V1InvoiceInput, idempotencyKey?: string) =>
            request<Invoice>("/invoices/emit", { method: "POST", body: payload, idempotencyKey }),
        annulInvoice: (id, motivoAnulacion) =>
            request<Invoice>(`/invoices/${id}/annul`, {
                method: "POST",
                body: { motivo_anulacion: motivoAnulacion },
            }),
        revertAnnul: (id) => request<Invoice>(`/invoices/${id}/annul/revert`, { method: "POST" }),
        getSiatStatus: (id) => request<Record<string, unknown>>(`/invoices/${id}/siat-status`),
        getInvoicePdfUrl: (id) => client.url(v1(`/invoices/${id}/pdf`)),
        getInvoiceXmlUrl: (id) => client.url(v1(`/invoices/${id}/xml`)),

        // Catálogos sincronizados
        listSinProducts: async (companyId, query?: string, limit = 50) => {
            const res = await request<Paginated<SinProduct> | { data?: SinProduct[] } | SinProduct[]>(
                `/companies/${companyId}/catalogs/productos-sin`,
                { query: { q: query, query, limit } },
            )
            return toPaginated<SinProduct>(res)
        },
        getCatalogReadiness: async (companyId, pointOfSaleId?: string) => {
            const res = await request<CatalogReadiness | { data?: CatalogReadiness }>(
                `/companies/${companyId}/catalogs/readiness`,
                { query: { point_of_sale_id: pointOfSaleId } },
            )
            if (res && typeof res === "object" && "data" in res && res.data) return res.data as CatalogReadiness
            return res as CatalogReadiness
        },
        getEmissionBootstrap: async (companyId, codigoActividad?: string) => {
            const res = await request<EmissionBootstrap | { data?: EmissionBootstrap }>(
                `/companies/${companyId}/catalogs/emision-bootstrap`,
                { query: { codigo_actividad: codigoActividad } },
            )
            if (res && typeof res === "object" && "data" in res && res.data) return res.data as EmissionBootstrap
            return res as EmissionBootstrap
        },
        listSectorPerfiles: async () => {
            const res = await request<SectorPerfil[] | { data?: SectorPerfil[] }>("/catalogs/perfiles-documento-sector")
            if (Array.isArray(res)) return res
            return res?.data ?? []
        },

        // Operación SIAT granular
        requestCuis: async (pointOfSaleId) =>
            siatResult(await request<unknown>(`/siat/cuis/${pointOfSaleId}`, { method: "POST" })),
        requestCufd: async (pointOfSaleId) =>
            siatResult(await request<unknown>(`/siat/cufd/${pointOfSaleId}`, { method: "POST" })),
        sincronizar: async (pointOfSaleId, operation?: string) =>
            siatResult(await request<unknown>(`/siat/sincronizar/${pointOfSaleId}`, {
                method: "POST",
                query: { operation },
            })),
        registrarEventoSignificativo: async (pointOfSaleId, payload?: Record<string, unknown>) =>
            siatResult(await request<unknown>(`/siat/evento-significativo/${pointOfSaleId}`, {
                method: "POST",
                body: payload ?? {},
            })),
        enviarPaquete: async (pointOfSaleId, input) =>
            siatResult(await request<unknown>(`/siat/paquete/${pointOfSaleId}`, {
                method: "POST",
                body: { invoice_ids: input.invoice_ids },
            })),
        validarPaquete: async (batchId) =>
            siatResult(await request<unknown>(`/siat/paquete/${batchId}/validate`, { method: "POST" })),
        enviarMasiva: async (pointOfSaleId, input) =>
            siatResult(await request<unknown>(`/siat/masiva/${pointOfSaleId}`, {
                method: "POST",
                body: { invoice_ids: input.invoice_ids },
            })),
        validarMasiva: async (batchId) =>
            siatResult(await request<unknown>(`/siat/masiva/${batchId}/validate`, { method: "POST" })),
        enviarCompras: async (pointOfSaleId, input) =>
            siatResult(await request<unknown>(`/siat/compras/${pointOfSaleId}`, {
                method: "POST",
                body: { invoice_ids: input.invoice_ids },
            })),

        // Exposure state methods
        setApiKey,
        getApiKey: () => apiKey,
        setStoredCompanyId: setActiveCompanyId,
        getStoredCompanyId: () => activeCompanyId,
        setActiveCompanyId,
        getActiveCompanyId: () => activeCompanyId,
    }
}

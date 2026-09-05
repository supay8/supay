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
const API_KEY_STORAGE_KEY = "supay_api_key"

function getStoredApiKey(): string {
	if (typeof window !== "undefined") {
		return localStorage.getItem(API_KEY_STORAGE_KEY) ?? ""
	}
	return ""
}

function setStoredApiKey(key: string): void {
	if (typeof window !== "undefined") {
		if (key) {
			localStorage.setItem(API_KEY_STORAGE_KEY, key)
		} else {
			localStorage.removeItem(API_KEY_STORAGE_KEY)
		}
	}
}

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
	const apiKey = getStoredApiKey()
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

async function requestWithoutAuth<T>(
	path: string,
	options: { method?: string; body?: unknown; query?: QueryParams } = {}
): Promise<T> {
	const headers: Record<string, string> = {}
	if (options.body !== undefined) headers["Content-Type"] = "application/json"

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
		bootstrap: (payload: Partial<Company>) =>
			requestWithoutAuth<{ company: Company; api_key: string; key_prefix: string; key_id: string }>("/companies", {
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
	apiKeys: {
		list: (companyId: string) =>
			request<{ id: string; key_prefix: string; name: string; is_active: boolean; last_used_at: string | null; created_at: string }[]>(`/companies/${companyId}/api-keys`),
		create: (companyId: string, name: string) =>
			request<{ api_key: string; key_prefix: string; id: string; name: string; created_at: string }>(`/companies/${companyId}/api-keys`, { method: "POST", body: { name } }),
		revoke: (companyId: string, keyId: string) =>
			request<void>(`/companies/${companyId}/api-keys/${keyId}`, { method: "DELETE" }),
	},
}

export const apiKeyStorage = {
	get: getStoredApiKey,
	set: setStoredApiKey,
	clear: () => setStoredApiKey(""),
}

interface ProductMappingInput {
	codigo_producto_sin: number
	codigo_actividad: string
	codigo_documento_sector: number
	unidad_medida: number
	is_default?: boolean
}
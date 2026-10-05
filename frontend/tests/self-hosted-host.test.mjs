import assert from "node:assert/strict"
import { after, before, test } from "node:test"
import { createServer } from "vite"

let server
let createSelfHostedHost

before(async () => {
  server = await createServer({
    configFile: new URL("../vite.config.ts", import.meta.url).pathname,
    server: { middlewareMode: true },
    appType: "custom",
  })
  const module = await server.ssrLoadModule(
    new URL("../../packages/dashboard/src/hosts/self-hosted.ts", import.meta.url).pathname,
  )
  createSelfHostedHost = module.createSelfHostedHost
})

after(async () => {
  await server?.close()
})

test("self-hosted host sends session and company headers to the backend", async () => {
  const calls = []
  const host = createSelfHostedHost({
    baseUrl: "http://backend.test:8081",
    fetchImpl: async (url, options) => {
      calls.push({ url: String(url), options })
      const body = calls.length === 1
        ? { access_token: "session-token", user: { id: "user-1" } }
        : { items: [] }
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    },
  })

  await host.login("user@example.test", "password")
  host.setActiveCompanyId("company-1")
  await host.listApiKeys("company-1")

  assert.equal(calls[0].url, "http://backend.test:8081/v1/auth/login")
  assert.equal(calls[0].options.method, "POST")
  assert.equal(calls[1].url, "http://backend.test:8081/v1/companies/company-1/api-keys")
  assert.equal(calls[1].options.headers.Authorization, "Bearer session-token")
  assert.equal(calls[1].options.headers["X-Company-ID"], "company-1")
})

test("self-hosted host exposes v1 preview and SIN catalog without legacy 404s", async () => {
  const calls = []
  const host = createSelfHostedHost({
    baseUrl: "http://backend.test:8081",
    fetchImpl: async (url, options) => {
      calls.push({ url: String(url), options })
      const u = String(url)
      const body = u.includes("/catalogs/productos-sin")
        ? { items: [{ codigo_producto_sin: 1, codigo_actividad: 101010, descripcion: "Producto SIN" }], total: 1, limit: 50, offset: 0 }
        : { total: 100 }
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    },
  })

  await host.previewInvoice({
    point_of_sale_id: "pos-1",
    customer: { document_type: "CI", document_number: "1", name: "Test" },
    items: [{
      sku: "SKU-1",
      description: "Producto SIN",
      codigo_producto_sin: 1,
      unidad_medida: 58,
      quantity: 1,
      price: 10,
    }],
  })
  assert.ok(calls[0].url.endsWith("/v1/invoices/preview"))

  const sin = await host.listSinProducts("company-1", "test")
  assert.equal(sin.items.length, 1)
  assert.ok(calls[1].url.includes("/v1/companies/company-1/catalogs/productos-sin"))

  assert.throws(() => host.createCustomer({ name: "x" }), /POST \/customers/)
  assert.throws(() => host.createProduct({ name: "x" }), /POST \/products/)
})

test("self-hosted host authenticates downloads and emits without an auxiliary key", async () => {
  const calls = []
  const host = createSelfHostedHost({
    baseUrl: "http://backend.test:8081",
    apiKey: "sup_test_secret",
    fetchImpl: async (url, options) => {
      calls.push({ url: String(url), options })
      if (String(url).endsWith("/pdf")) {
        return new Response(new Uint8Array([37, 80, 68, 70]), {
          status: 200,
          headers: { "Content-Type": "application/pdf" },
        })
      }
      return new Response(JSON.stringify({ id: "invoice-1" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    },
  })

  const blob = await host.downloadInvoicePdf("invoice-1")
  assert.equal(blob.type, "application/pdf")
  assert.equal(calls[0].options.headers["X-API-Key"], "sup_test_secret")

  await host.emitInvoiceDirect({
    point_of_sale_id: "pos-1",
    customer: { document_type: "CI", document_number: "1", name: "Test" },
    items: [{
      sku: "SKU-1",
      description: "Producto SIN",
      codigo_producto_sin: 1,
      unidad_medida: 58,
      quantity: 1,
      price: 10,
    }],
  })

  assert.equal(calls[1].options.headers["Idempotency-Key"], undefined)
})

test("dashboard critical flow emits, checks, downloads, and annuls with the v1 contract", async () => {
  const calls = []
  const host = createSelfHostedHost({
    baseUrl: "https://api.example.test",
    apiKey: "sup_pilot_secret",
    fetchImpl: async (url, options = {}) => {
      calls.push({ url: String(url), options })
      const path = new URL(String(url)).pathname
      if (path.endsWith("/pdf")) {
        return new Response(new Uint8Array([37, 80, 68, 70, 45]), {
          status: 200,
          headers: { "Content-Type": "application/pdf" },
        })
      }
      if (path.endsWith("/xml")) {
        return new Response("<facturaComputarizadaCompraVenta/>", {
          status: 200,
          headers: { "Content-Type": "application/xml" },
        })
      }
      const body = path.endsWith("/siat-status")
        ? { estado: "VALIDADA" }
        : path.endsWith("/annul")
          ? { id: "invoice-1", status: "CANCELLED" }
          : { id: "invoice-1", status: "ACCEPTED" }
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    },
  })

  await host.emitInvoiceDirect({
    point_of_sale_id: "pos-1",
    customer: { document_type: "CI", document_number: "1", name: "Piloto" },
    items: [{
      sku: "SKU-1",
      description: "Producto piloto",
      codigo_producto_sin: 1,
      unidad_medida: 58,
      quantity: 1,
      price: 10.25,
    }],
  })
  await host.getSiatStatus("invoice-1")
  await host.downloadInvoiceXml("invoice-1")
  await host.downloadInvoicePdf("invoice-1")
  await host.annulInvoice("invoice-1", 90)

  assert.deepEqual(calls.map((call) => new URL(call.url).pathname), [
    "/v1/invoices/emit",
    "/v1/invoices/invoice-1/siat-status",
    "/v1/invoices/invoice-1/xml",
    "/v1/invoices/invoice-1/pdf",
    "/v1/invoices/invoice-1/annul",
  ])
  assert.equal(calls[0].options.headers["Idempotency-Key"], undefined)
  assert.equal(calls[4].options.body, JSON.stringify({ codigo_motivo: 90 }))
  for (const call of calls) {
    assert.equal(call.options.headers["X-API-Key"], "sup_pilot_secret")
  }
})

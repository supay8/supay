import test from "node:test"
import assert from "node:assert/strict"
import {
  certDisplayName,
  certStatusLabel,
  formatBranch,
  formatCompanyTitle,
  formatCustomer,
  formatInvoiceTitle,
  formatPosShort,
} from "../src/lib/display-names.ts"

test("formatPosShort nunca expone UUIDs", () => {
  const s = formatPosShort({ description: "Caja 1", codigo_sucursal: 0, codigo_punto_venta: 1 })
  assert.ok(s.includes("Caja 1"))
  assert.ok(s.includes("Suc 00"))
  assert.ok(s.includes("Pos 01"))
})

test("formatBranch muestra nombre y código", () => {
  assert.equal(formatBranch({ name: "Casa Matriz", codigo_sucursal: 0 }), "Casa Matriz (Suc 00)")
})

test("formatCustomer e invoice/company usan nombres", () => {
  assert.equal(formatCustomer({ name: "Ada", document_type: "ci", document_number: "123" }), "Ada · CI 123")
  assert.equal(formatInvoiceTitle({ invoice_number: 7, total: 0 }), "N° 000007")
  assert.equal(formatCompanyTitle("Mi Empresa", "123"), "Mi Empresa · NIT 123")
})

test("certificado sin metadatos no filtra UUID", () => {
  assert.equal(certDisplayName({}), "Certificado del NIT")
  assert.equal(certStatusLabel("REVOKED", false), "Revocado")
  assert.equal(certStatusLabel("EXPIRED", false), "Expirado")
  assert.equal(certStatusLabel("x", true), "Activo")
})

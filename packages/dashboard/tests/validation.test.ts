import test from "node:test"
import assert from "node:assert/strict"
import { NIT_ERROR, isValidEmail, isValidNit } from "../src/lib/validation.ts"

test("NIT acepta solo dígitos de 5 a 15", () => {
  assert.equal(isValidNit("123456789"), true)
  assert.equal(isValidNit("  1020304015  "), true)
  assert.equal(isValidNit("1234"), false)
  assert.equal(isValidNit("1234567890123456"), false)
  assert.equal(isValidNit("12A456"), false)
  assert.equal(isValidNit(""), false)
  assert.ok(NIT_ERROR.length > 0)
})

test("email opcional pero válido si se informa", () => {
  assert.equal(isValidEmail(""), true)
  assert.equal(isValidEmail("  "), true)
  assert.equal(isValidEmail("cliente@mail.com"), true)
  assert.equal(isValidEmail("sin-arroba"), false)
})

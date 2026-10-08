export const NIT_PATTERN = /^\d{5,15}$/

export const NIT_ERROR = "NIT inválido: solo dígitos, de 5 a 15 caracteres."

export function isValidNit(nit: string): boolean {
  return NIT_PATTERN.test(nit.trim())
}

export function isValidEmail(email: string): boolean {
  const v = email.trim()
  if (v === "") return true
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)
}

export function toastApiError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message
  return fallback
}

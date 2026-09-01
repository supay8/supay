import { createSelfHostedHost } from "@supay/dashboard"

export const selfHostedHost = createSelfHostedHost({
  baseUrl: import.meta.env.VITE_API_URL,
  apiKey: import.meta.env.VITE_API_KEY,
})

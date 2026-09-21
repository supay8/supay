import { createSelfHostedHost } from "@supay/dashboard"

export const selfHostedHost = createSelfHostedHost({
	baseUrl: import.meta.env.VITE_API_URL ?? "http://localhost:8081",
	apiKey: import.meta.env.VITE_API_KEY,
})

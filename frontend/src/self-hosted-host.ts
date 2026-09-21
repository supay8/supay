import { createSelfHostedHost } from "@supay/dashboard"

export const selfHostedHost = createSelfHostedHost({
	baseUrl: "http://localhost:8081",
	apiKey: import.meta.env.VITE_API_KEY,
})
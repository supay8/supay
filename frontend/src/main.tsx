import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "@fontsource-variable/geist"
import "@fontsource-variable/geist-mono"
import "./index.css"
import { SupayDashboard } from "@supay/dashboard"
import "@supay/dashboard/styles.css"
import { selfHostedHost } from "./self-hosted-host"

// Fuente de verdad del dashboard: packages/dashboard/src/*
// Si editas frontend/src/App.tsx o frontend/src/pages/* NO verás cambios:
// este entry solo monta SupayDashboard (ver vite.config.ts alias @ -> dashboard).
// Para cambios visibles edita packages/dashboard/src/* o este main.tsx/self-hosted-host.ts

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <SupayDashboard mode="self-hosted" host={selfHostedHost} />
  </StrictMode>
)

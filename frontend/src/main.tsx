import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "@fontsource-variable/geist"
import "@fontsource-variable/geist-mono"
import "./index.css"
import { SupayDashboard } from "@supay/dashboard"
import "@supay/dashboard/styles.css"
import { selfHostedHost } from "./self-hosted-host"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <SupayDashboard mode="self-hosted" host={selfHostedHost} />
  </StrictMode>
)

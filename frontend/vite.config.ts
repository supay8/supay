import path from "node:path"

import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

const dashboardSrc = path.resolve(
  import.meta.dirname,
  "../packages/dashboard/src"
)

const frontendSrc = path.resolve(
  import.meta.dirname,
  "./src"
)

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],

  resolve: {
    alias: {
      // frontend
      "@": frontendSrc,

      // dashboard package
      "@supay/dashboard": dashboardSrc,
    },

    // Muy recomendable en monorepos con React
    dedupe: ["react", "react-dom"],
  },

  server: {
    fs: {
      allow: [
        path.resolve(import.meta.dirname, ".."),
      ],
    },

    hmr: {
      overlay: true,
    },
  },

  optimizeDeps: {
    exclude: ["@supay/dashboard"],
  },
})
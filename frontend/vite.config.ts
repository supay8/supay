import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "../packages/dashboard/src"),
      "@frontend": path.resolve(__dirname, "./src"),
      "@supay/dashboard": path.resolve(__dirname, "../packages/dashboard/src"),
      "@supay/dashboard/styles.css": path.resolve(
        __dirname,
        "../packages/dashboard/src/styles.css"
      ),
    },
  },
})

/**
 * @deprecated ESTE ARCHIVO NO SE USA EN DEV.
 * El frontend real renderiza `SupayDashboard` desde `@supay/dashboard`
 * (alias Vite `@` -> `../packages/dashboard/src`, ver `frontend/vite.config.ts:11`).
 * `main.tsx` importa `SupayDashboard` y nunca importa este `App`.
 * Si editas este archivo NO verás cambios en `pnpm dev`.
 * Edita `packages/dashboard/src/routing.tsx` y `packages/dashboard/src/pages/*` en su lugar.
 * Se mantiene solo como referencia histórica / fallback si se quiere modo standalone.
 */
import { lazy, Suspense } from "react"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { AppShell } from "@/components/layout/app-shell"
import { PlaceholderPage } from "@/pages/placeholder-page"
import { Spinner } from "@/components/ui/spinner"

const InvoicesPage = lazy(() =>
  import("@/pages/invoices-page").then((m) => ({ default: m.InvoicesPage }))
)
const InvoiceNewPage = lazy(() =>
  import("@/pages/invoice-new-page").then((m) => ({ default: m.InvoiceNewPage }))
)
const SetupPage = lazy(() =>
  import("@/pages/setup-page").then((m) => ({ default: m.SetupPage }))
)
const CompanyPage = lazy(() =>
  import("@/pages/company-page").then((m) => ({ default: m.CompanyPage }))
)
const BranchesPage = lazy(() =>
  import("@/pages/branches-page").then((m) => ({ default: m.BranchesPage }))
)
const PointsOfSalePage = lazy(() =>
  import("@/pages/points-of-sale-page").then((m) => ({
    default: m.PointsOfSalePage,
  }))
)
const SectorsPage = lazy(() =>
  import("@/pages/sectors-page").then((m) => ({ default: m.SectorsPage }))
)
const SiatConnectionPage = lazy(() =>
  import("@/pages/siat-page").then((m) => ({ default: m.SiatConnectionPage }))
)
const CustomersPage = lazy(() =>
  import("@/pages/customers-page").then((m) => ({ default: m.CustomersPage }))
)
const ProductsPage = lazy(() =>
  import("@/pages/products-page").then((m) => ({ default: m.ProductsPage }))
)
const ContingenciaPage = lazy(() =>
  import("@/pages/contingencia-page").then((m) => ({
    default: m.ContingenciaPage,
  }))
)
const LotesPage = lazy(() =>
  import("@/pages/operation-pages").then((m) => ({ default: m.LotesPage }))
)
const ComprasPage = lazy(() =>
  import("@/pages/operation-pages").then((m) => ({ default: m.ComprasPage }))
)
const LoginPage = lazy(() =>
  import("@/pages/login-page").then((m) => ({ default: m.default }))
)
const SignupPage = lazy(() =>
  import("@/pages/signup-page").then((m) => ({ default: m.default }))
)
function RouteLoader() {
  return (
    <div className="flex min-h-[50vh] items-center justify-center">
      <Spinner className="size-5" />
    </div>
  )
}

function page(node: React.ReactNode) {
  return <Suspense fallback={<RouteLoader />}>{node}</Suspense>
}

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={page(<LoginPage />)} />
        <Route path="/signup" element={page(<SignupPage />)} />
        <Route element={<AppShell />}>
          <Route path="/" element={<Navigate to="/invoices" replace />} />
          <Route path="/setup" element={page(<SetupPage />)} />
          <Route path="/invoices" element={page(<InvoicesPage />)} />
          <Route path="/invoices/new" element={page(<InvoiceNewPage />)} />
          <Route path="/customers" element={page(<CustomersPage />)} />
          <Route path="/products" element={page(<ProductsPage />)} />
          <Route path="/company" element={page(<CompanyPage />)} />
          <Route path="/company/branches" element={page(<BranchesPage />)} />
         
          <Route
            path="/company/points-of-sale"
            element={page(<PointsOfSalePage />)}
          />
          <Route path="/company/sectors" element={page(<SectorsPage />)} />
          <Route
            path="/company/siat"
            element={page(<SiatConnectionPage />)}
          />
          <Route
            path="/operation/contingencia"
            element={page(<ContingenciaPage />)}
          />
          <Route path="/operation/lotes" element={page(<LotesPage />)} />
          <Route path="/operation/compras" element={page(<ComprasPage />)} />
          <Route
            path="*"
            element={
              page(
                <PlaceholderPage
                  title="Página no encontrada"
                  description="La ruta solicitada no existe."
                />
              ) as React.ReactNode
            }
          />
        </Route>
    
      </Routes>
    </BrowserRouter>
  )
}

export default App

import * as React from "react"
import { Navigate, Route, Routes } from "react-router-dom"
import { AppShell } from "@/components/layout/app-shell"
import { PlaceholderPage } from "@/pages/placeholder-page"
import { Spinner } from "@/components/ui/spinner"

export interface DashboardRoute {
  path: string
  element: React.ReactNode
  hidden?: boolean
}

// Lazy pages - same as frontend/src/App.tsx but imported from dashboard
const InvoicesPage = React.lazy(() =>
  import("@/pages/invoices-page").then((m) => ({ default: m.InvoicesPage }))
)
const InvoiceNewPage = React.lazy(() =>
  import("@/pages/invoice-new-page").then((m) => ({ default: m.InvoiceNewPage }))
)
const SetupPage = React.lazy(() =>
  import("@/pages/setup-page").then((m) => ({ default: m.SetupPage }))
)
const CompanyPage = React.lazy(() =>
  import("@/pages/company-page").then((m) => ({ default: m.CompanyPage }))
)
const BranchesPage = React.lazy(() =>
  import("@/pages/branches-page").then((m) => ({ default: m.BranchesPage }))
)
const PointsOfSalePage = React.lazy(() =>
  import("@/pages/points-of-sale-page").then((m) => ({ default: m.PointsOfSalePage }))
)
const SectorsPage = React.lazy(() =>
  import("@/pages/sectors-page").then((m) => ({ default: m.SectorsPage }))
)
const SiatConnectionPage = React.lazy(() =>
  import("@/pages/siat-page").then((m) => ({ default: m.SiatConnectionPage }))
)
const CustomersPage = React.lazy(() =>
  import("@/pages/customers-page").then((m) => ({ default: m.CustomersPage }))
)
const ProductsPage = React.lazy(() =>
  import("@/pages/products-page").then((m) => ({ default: m.ProductsPage }))
)
const ContingenciaPage = React.lazy(() =>
  import("@/pages/contingencia-page").then((m) => ({ default: m.ContingenciaPage }))
)
const LotesPage = React.lazy(() =>
  import("@/pages/operation-pages").then((m) => ({ default: m.LotesPage }))
)
const ComprasPage = React.lazy(() =>
  import("@/pages/operation-pages").then((m) => ({ default: m.ComprasPage }))
)

function RouteLoader() {
  return (
    <div className="flex min-h-[50vh] items-center justify-center">
      <Spinner className="size-5" />
    </div>
  )
}

function page(node: React.ReactNode) {
  return <React.Suspense fallback={<RouteLoader />}>{node}</React.Suspense>
}

export function DashboardRoutes({ extraRoutes }: { extraRoutes?: DashboardRoute[] }) {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route path="/" element={<Navigate to="/invoices" replace />} />
        <Route path="/setup" element={page(<SetupPage />)} />
        <Route path="/invoices" element={page(<InvoicesPage />)} />
        <Route path="/invoices/new" element={page(<InvoiceNewPage />)} />
        <Route path="/customers" element={page(<CustomersPage />)} />
        <Route path="/products" element={page(<ProductsPage />)} />
        <Route path="/company" element={page(<CompanyPage />)} />
        <Route path="/company/branches" element={page(<BranchesPage />)} />
        <Route path="/company/points-of-sale" element={page(<PointsOfSalePage />)} />
        <Route path="/company/sectors" element={page(<SectorsPage />)} />
        <Route path="/company/siat" element={page(<SiatConnectionPage />)} />
        <Route path="/operation/contingencia" element={page(<ContingenciaPage />)} />
        <Route path="/operation/lotes" element={page(<LotesPage />)} />
        <Route path="/operation/compras" element={page(<ComprasPage />)} />

        {/* Extra routes injected by host (Cloud) */}
        {extraRoutes?.map((r) => (
          <Route key={r.path} path={r.path} element={page(r.element)} />
        ))}

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
  )
}

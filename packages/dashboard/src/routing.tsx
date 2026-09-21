import * as React from "react"
import { Navigate, Route, Routes, Outlet } from "react-router-dom"
import { AppShell } from "./components/layout/app-shell"
import { PlaceholderPage } from "./pages/placeholder-page"
import { Spinner } from "./components/ui/spinner"
import { useAuth } from "./auth-context"

export interface DashboardRoute {
  path: string
  element: React.ReactNode
  hidden?: boolean
}

// Lazy loading centralizado (se mantiene igual, es la mejor forma para bundlers)
const InvoicesPage = React.lazy(() => import("./pages/invoices-page").then((m) => ({ default: m.InvoicesPage })))
const InvoiceNewPage = React.lazy(() => import("./pages/invoice-new-page").then((m) => ({ default: m.InvoiceNewPage })))
const DashboardHomePage = React.lazy(() => import("./pages/dashboard-home-page").then((m) => ({ default: m.DashboardHomePage })))
const SetupPage = React.lazy(() => import("./pages/setup-page").then((m) => ({ default: m.SetupPage })))
const CompanyPage = React.lazy(() => import("./pages/company-page").then((m) => ({ default: m.CompanyPage })))
const BranchesPage = React.lazy(() => import("./pages/branches-page").then((m) => ({ default: m.BranchesPage })))
const PointsOfSalePage = React.lazy(() => import("./pages/points-of-sale-page").then((m) => ({ default: m.PointsOfSalePage })))
const SectorsPage = React.lazy(() => import("./pages/sectors-page").then((m) => ({ default: m.SectorsPage })))
const SiatConnectionPage = React.lazy(() => import("./pages/siat-page").then((m) => ({ default: m.SiatConnectionPage })))
const CustomersPage = React.lazy(() => import("./pages/customers-page").then((m) => ({ default: m.CustomersPage })))
const ProductsPage = React.lazy(() => import("./pages/products-page").then((m) => ({ default: m.ProductsPage })))
const ContingenciaPage = React.lazy(() => import("./pages/contingencia-page").then((m) => ({ default: m.ContingenciaPage })))
const LotesPage = React.lazy(() => import("./pages/operation-pages").then((m) => ({ default: m.LotesPage })))
const ComprasPage = React.lazy(() => import("./pages/operation-pages").then((m) => ({ default: m.ComprasPage })))
const SignupPage = React.lazy(() => import("./pages/signup-page").then((m) => ({ default: m.SignupPage })))
const LoginPage = React.lazy(() => import("./pages/login-page").then((m) => ({ default: m.default })))
const CompaniesPage = React.lazy(() => import("./pages/companies-page").then((m) => ({ default: m.CompaniesPage })))
const ApiKeysPage = React.lazy(() => import("./pages/api-keys-page").then((m) => ({ default: m.ApiKeysPage })))
const CertificatePage = React.lazy(() => import("./pages/certificate-page").then((m) => ({ default: m.CertificatePage })))

function RouteLoader() {
  return (
    <div className="flex min-h-[50vh] items-center justify-center">
      <Spinner className="size-5" />
    </div>
  )
}

// NUEVO: Componente Layout para manejar Suspense de forma global
function SuspenseLayout() {
  return (
    <React.Suspense fallback={<RouteLoader />}>
      <Outlet />
    </React.Suspense>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()
  if (isLoading) return <RouteLoader />
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

function PublicOnlyRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()
  if (isLoading) return <RouteLoader />
  if (isAuthenticated) return <Navigate to="/" replace />
  return <>{children}</>
}

function CompanyGate({ children }: { children: React.ReactNode }) {
  const { hasCompany, companiesLoading, isLoading } = useAuth()
  // Ya no hace falta evaluar isLoading si ProtectedRoute ya lo hizo, pero es más seguro dejarlo.
  if (isLoading || companiesLoading) return <RouteLoader />
  if (!hasCompany) return <Navigate to="/companies" replace />
  return <>{children}</>
}

export function DashboardRoutes({ extraRoutes = [] }: { extraRoutes?: DashboardRoute[] }) {
  return (
    <Routes>
      {/* Rutas Públicas (Envueltas en un único Suspense) */}
      <Route element={<SuspenseLayout />}>
        <Route path="/login" element={<PublicOnlyRoute><LoginPage /></PublicOnlyRoute>} />
        <Route path="/signup" element={<PublicOnlyRoute><SignupPage /></PublicOnlyRoute>} />
      </Route>

      {/* Selector de Compañías (Protegida, sin AppShell aún) */}
      <Route element={<SuspenseLayout />}>
        <Route path="/companies" element={<ProtectedRoute><CompaniesPage /></ProtectedRoute>} />
      </Route>

      {/* Rutas del Dashboard Principal (Protegidas, con AppShell y CompanyGate) */}
      <Route 
        element={
          <ProtectedRoute>
            <CompanyGate>
              <AppShell />
            </CompanyGate>
          </ProtectedRoute>
        }
      >
        {/* Usamos un SuspenseLayout anidado dentro de AppShell. 
            Esto hace que la barra lateral (AppShell) cargue de inmediato 
            y solo muestre el spinner en el contenido principal. */}
        <Route element={<SuspenseLayout />}>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<DashboardHomePage />} />
          <Route path="/setup" element={<SetupPage />} />
          <Route path="/invoices" element={<InvoicesPage />} />
          <Route path="/invoices/new" element={<InvoiceNewPage />} />
          <Route path="/customers" element={<CustomersPage />} />
          <Route path="/products" element={<ProductsPage />} />
          <Route path="/company" element={<CompanyPage />} />
          <Route path="/company/api-keys" element={<ApiKeysPage />} />
          <Route path="/company/certificates" element={<CertificatePage />} />
          <Route path="/company/branches" element={<BranchesPage />} />
          <Route path="/company/points-of-sale" element={<PointsOfSalePage />} />
          <Route path="/company/sectors" element={<SectorsPage />} />
          <Route path="/company/siat" element={<SiatConnectionPage />} />
          <Route path="/operation/contingencia" element={<ContingenciaPage />} />
          <Route path="/operation/lotes" element={<LotesPage />} />
          <Route path="/operation/compras" element={<ComprasPage />} />

          {/* Extra routes injected by host (Cloud) */}
          {extraRoutes.map((r) => (
            <Route key={r.path} path={r.path} element={r.element} />
          ))}

          <Route 
            path="*" 
            element={<PlaceholderPage title="Página no encontrada" description="La ruta solicitada no existe." />} 
          />
        </Route>
      </Route>
    </Routes>
  )
}
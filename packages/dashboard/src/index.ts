// Styles - host debe importar "@supay/dashboard/styles.css" si quiere tokens
// export public API
export { SupayDashboard } from "./dashboard"
export type { SupayDashboardProps, DashboardRoute } from "./dashboard"

export type { DashboardHost, HostError, ApiError, ProductMappingInput, AuthUser, AuthResult, UserCompany, CreateMyCompanyInput, ApiKeyItem, CreateApiKeyResponse, CertificateItem, UploadCertificateInput, AuthHost, CompanyAdminHost, InvoiceHost, CatalogHost, SiatOpsHost } from "./host"
export { ApiError as HostErrorAlias } from "./host"

export type { DashboardConfig, DashboardMode } from "./config"

export type { NavSection, NavItem, ActiveNav } from "./lib/nav-config"
export { NAV_SECTIONS, mergeNavSections, findActiveNav } from "./lib/nav-config"

export { DashboardHostProvider, useDashboardHost } from "./host-context"
export { AuthProvider, useAuth } from "./auth-context"
export { DashboardExtensionsProvider, useDashboardExtensions } from "./dashboard-context"

export { createSelfHostedHost } from "./hosts/self-hosted"
export type { SelfHostedHostOptions } from "./hosts/self-hosted"

export { DashboardRoutes } from "./routing"
export type { DashboardRoute as DashboardRouteType } from "./routing"

// Re-export types útiles
export type {
  Company,
  Branch,
  PointOfSale,
  Customer,
  Product,
  ProductMapping,
  Invoice,
  InvoiceItem,
  SectorInfo,
  SectorFieldInfo,
  SectorPerfil,
  SinProduct,
  CatalogReadiness,
  EmissionBootstrap,
  InvoicePreview,
  V1InvoiceInput,
  V1CustomerInput,
  V1ItemInput,
  SiatOperationResult,
  BatchSendInput,
  Paginated,
  InvoiceListParams,
  DraftInput,
  DraftItemInput,
  InvoiceStatus,
  SiatEnvironment,
} from "./lib/types"

export { INVOICE_STATUSES } from "./lib/types"
export * from "./lib/invoice-status"
export * from "./lib/format"

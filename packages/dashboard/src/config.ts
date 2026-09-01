export type DashboardMode = "self-hosted" | "cloud"

export interface DashboardBranding {
  title?: string
  logo?: React.ReactNode
}

export interface DashboardConfig {
  mode: DashboardMode
  branding?: DashboardBranding
  /** companyId por defecto para single-tenant self-hosted */
  defaultCompanyId?: string
}

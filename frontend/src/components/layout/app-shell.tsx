/**
 * @deprecated ESTE ARCHIVO NO SE USA EN DEV — CÓDIGO MUERTO.
 * El frontend real renderiza `AppShell` desde `packages/dashboard/src/components/layout/app-shell.tsx`
 * vía alias Vite `@` -> `../packages/dashboard/src` (ver `frontend/vite.config.ts:11`).
 * `frontend/src/main.tsx` monta `SupayDashboard` que importa `@/components/layout/app-shell` (dashboard).
 * Si editas ESTE archivo NO verás cambios en `pnpm dev`.
 * Edita `packages/dashboard/src/components/layout/app-shell.tsx` en su lugar.
 */
import { useEffect } from "react"
import { NavLink, useLocation, useNavigate, Outlet } from "react-router-dom"

import { NAV_SECTIONS } from "@/lib/nav-config"
import { AppHeader } from "@/components/layout/header"
import { SidebarUserMenu } from "@/components/layout/user-menu"
import { LogoSupay } from "@/components/logo"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
} from "@/components/ui/sidebar"

function NavItems({ sectionIndex }: { sectionIndex: number }) {
  const location = useLocation()
  const section = NAV_SECTIONS[sectionIndex]
  return (
    <SidebarMenu>
      {section.items.map((item) => (
        <SidebarMenuItem key={item.to}>
          <SidebarMenuButton
            render={<NavLink to={item.to} />}
            isActive={
              item.to === "/invoices"
                ? location.pathname === "/invoices" ||
                  (location.pathname.startsWith("/invoices") &&
                    location.pathname !== "/invoices/new")
                : location.pathname.startsWith(item.to)
            }
            tooltip={item.label}
          >
            <item.icon />
            <span>{item.label}</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
      ))}
    </SidebarMenu>
  )
}

export function AppShell() {
  const navigate = useNavigate()

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.repeat || event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      if (
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.tagName === "SELECT" ||
          target.isContentEditable)
      ) {
        return
      }
      if (event.key.toLowerCase() !== "n") return
      event.preventDefault()
      navigate("/invoices/new")
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [navigate])

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon">
        <SidebarHeader className="h-14 justify-center border-b border-border/60">
          <div className="flex items-center gap-2 px-2 group-data-[collapsible=icon]:hidden">
            <LogoSupay size={35} className="shrink-0 text-primary" color="currentColor" />
            <span className="font-bold tracking-widest ">Supay</span>
          </div>
          <div className="hidden group-data-[collapsible=icon]:flex justify-center">
            <LogoSupay size={28} className="text-primary" color="currentColor" />
          </div>
        </SidebarHeader>
        <SidebarContent>
          {NAV_SECTIONS.map((section, index) => (
            <SidebarGroup key={section.label}>
              <SidebarGroupLabel>{section.label}</SidebarGroupLabel>
              <SidebarGroupContent>
                <NavItems sectionIndex={index} />
              </SidebarGroupContent>
            </SidebarGroup>
          ))}
        </SidebarContent>
        <SidebarFooter className="border-t border-border/60">
          <SidebarUserMenu />
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
      <SidebarInset>
        <AppHeader />
        <main className="flex-1 p-4 md:p-6">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}

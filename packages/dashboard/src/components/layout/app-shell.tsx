import { useEffect } from "react"
import { NavLink, useLocation, useNavigate, Outlet } from "react-router-dom"

import { useDashboardExtensions } from "../../dashboard-context"
import { AppHeader } from "../../components/layout/header"
import { SidebarUserMenu } from "../../components/layout/user-menu"
import { LogoSupay } from "../../components/logo"
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
} from "../../components/ui/sidebar"

function NavItems({ sectionIndex }: { sectionIndex: number }) {
  const location = useLocation()
  const { navSections } = useDashboardExtensions()
  const section = navSections[sectionIndex]
  if (!section) return null
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
  const { navSections } = useDashboardExtensions()
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
            <LogoSupay size={28} className="shrink-0 text-primary" color="currentColor" />
            <span className="text-sm font-semibold tracking-tight">Supay</span>
          </div>
          <div className="hidden group-data-[collapsible=icon]:flex justify-center">
            <LogoSupay size={28} className="text-primary" color="currentColor" />
          </div>
        </SidebarHeader>
        <SidebarContent>
          {navSections.map((section, index) => (
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

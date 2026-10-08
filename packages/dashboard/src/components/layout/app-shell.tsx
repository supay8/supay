import { NavLink, useLocation, Outlet } from "react-router-dom"

import { useDashboardExtensions } from "../../dashboard-context"
import { AppHeader } from "../../components/layout/header"
import { SidebarUserMenu } from "../../components/layout/user-menu"
import {  SupayIcon } from "../../components/logo"
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
import { useTheme } from "../theme-provider"

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
            isActive={location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)}
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
  const {theme} = useTheme();
  const currentTheme = theme === "dark" ? "dark" : "light";
  

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon">
        <SidebarHeader className="h-14 justify-center border-b border-border/60">
          <div className="flex items-center gap-2 px-2 group-data-[collapsible=icon]:hidden">
            <SupayIcon size={28} theme={currentTheme} className="shrink-0 text-primary" color="currentColor" />
            <span className="text-sm font-semibold tracking-tight">Supay</span>
          </div>
          <div className="hidden group-data-[collapsible=icon]:flex justify-center">
            <SupayIcon size={28} theme={currentTheme} className="text-primary" color="currentColor" />
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

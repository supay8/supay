import { useNavigate } from "react-router-dom"
import { Check, LogOut, Monitor, Moon, Settings, Sun, User } from "lucide-react"
import { toast } from "sonner"

import { useTheme } from "@/components/theme-provider"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

const THEME_OPTIONS = [
  { value: "light", label: "Claro", icon: Sun },
  { value: "dark", label: "Oscuro", icon: Moon },
  { value: "system", label: "Sistema", icon: Monitor },
] as const

export function SidebarUserMenu() {
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            className="hover:bg-sidebar-accent focus-visible:ring-sidebar-ring flex w-full items-center gap-2 overflow-hidden rounded-md p-2 text-left outline-none focus-visible:ring-2"
            aria-label="Menú de usuario"
          />
        }
      >
        <span className="bg-primary text-primary-foreground grid size-7 shrink-0 place-items-center rounded-full text-[11px] font-semibold">
          CA
        </span>
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-[13px] font-medium">Mi cuenta</span>
          <span className="text-muted-foreground truncate text-[11px]">
            Administrador
          </span>
        </span>
      </DropdownMenuTrigger>

      <DropdownMenuContent side="top" align="start" className="w-56">
        <DropdownMenuLabel>Sesión</DropdownMenuLabel>
        <DropdownMenuItem disabled className="gap-2.5">
          <User className="size-4" />
          Perfil
        </DropdownMenuItem>
        <DropdownMenuItem
          className="gap-2.5"
          onClick={() => navigate("/company")}
        >
          <Settings className="size-4" />
          Configuraciones
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuLabel>Apariencia</DropdownMenuLabel>
        {THEME_OPTIONS.map((option) => (
          <DropdownMenuItem
            key={option.value}
            className="gap-2.5"
            onClick={() => setTheme(option.value)}
          >
            <option.icon className="size-4" />
            {option.label}
            {theme === option.value && (
              <Check className="text-success ml-auto size-4" />
            )}
          </DropdownMenuItem>
        ))}

        <DropdownMenuSeparator />

        <DropdownMenuItem
          disabled
          className="gap-2.5"
          onClick={() => toast.info("La sesión llega con la autenticación")}
        >
          <LogOut className="size-4" />
          Cerrar sesión
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

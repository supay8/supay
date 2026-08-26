import { useNavigate } from "react-router-dom"
import { Building2, Check, ChevronsUpDown, Plus } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

export function OrganizationSwitcher() {
  const navigate = useNavigate()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            className="hover:bg-accent focus-visible:ring-ring/40 flex h-8 shrink-0 items-center gap-2 rounded-md px-1.5 outline-none focus-visible:ring-2"
            aria-label="Cambiar organización"
          />
        }
      >
        <span className="bg-primary text-primary-foreground grid size-5 shrink-0 place-items-center rounded text-[9px] font-semibold">
          CA
        </span>
        <span className="hidden truncate text-[13px] font-medium tracking-tight sm:block">
          Comercial Andina SRL
        </span>
        <ChevronsUpDown className="text-muted-foreground size-3 shrink-0 opacity-60" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuLabel>Organización</DropdownMenuLabel>
        <DropdownMenuItem className="gap-2.5">
          <span className="bg-primary text-primary-foreground grid size-6 place-items-center rounded text-[10px] font-semibold">
            CA
          </span>
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-[13px] font-medium">
              Comercial Andina SRL
            </span>
            <span className="text-muted-foreground flex items-center gap-1.5 font-mono text-[11px]">
              NIT 1020304015
              <Badge variant="outline" className="h-3.5 px-1 text-[9px]">
                Piloto
              </Badge>
            </span>
          </span>
          <Check className="ml-auto size-4" />
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem
            onClick={() => navigate("/company")}
            className="gap-2.5"
          >
            <Building2 className="size-4" />
            Configuración de la empresa
          </DropdownMenuItem>
          <DropdownMenuItem disabled className="gap-2.5">
            <Plus className="size-4" />
            Nueva organización
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

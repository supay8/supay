import { useState } from "react"
import { useDashboardHost } from "../../host-context"
import { useAuth } from "../../auth-context"
import { useQuery } from "@tanstack/react-query"
import { ChevronsUpDown, PackageOpen, Plus } from "lucide-react"


import type { Product } from "../../lib/types"
import { Button } from "../../components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "../../components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "../../components/ui/popover"

export function ProductCombobox({
  onSelectProduct,
  onFreeLine,
}: {
  onSelectProduct: (product: Product) => void
  onFreeLine: () => void
}) {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")

  const productsQuery = useQuery({
    queryKey: ["products", companyId],
    queryFn: () => host.listProducts(companyId),
    staleTime: 60_000,
    enabled: companyId !== "",
  })

  const products = productsQuery.data?.items ?? []
  const needle = search.trim().toLowerCase()
  const filtered = needle
    ? products.filter(
        (p) =>
          p.name.toLowerCase().includes(needle) ||
          p.sku.toLowerCase().includes(needle)
      )
    : products.slice(0, 20)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button variant="outline" size="sm" />}>
        <Plus data-icon="inline-start" />
        Agregar producto
        <ChevronsUpDown className="ml-1 size-3 opacity-50" />
      </PopoverTrigger>
      <PopoverContent className="w-[360px] p-0" align="end">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="Buscar por nombre o código…"
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            <CommandEmpty>Sin resultados en tu catálogo.</CommandEmpty>
            <CommandGroup>
              {filtered.map((product) => (
                <CommandItem
                  key={product.id}
                  value={product.id}
                  onSelect={() => {
                    onSelectProduct(product)
                    setOpen(false)
                    setSearch("")
                  }}
                >
                  <span className="truncate">{product.name}</span>
                  <span className="text-muted-foreground ml-auto font-mono text-xs">
                    {product.sku}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandGroup>
              <CommandItem
                value="__libre"
                onSelect={() => {
                  onFreeLine()
                  setOpen(false)
                }}
              >
                <PackageOpen className="mr-1 size-4" />
                Línea libre (sin catálogo)…
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

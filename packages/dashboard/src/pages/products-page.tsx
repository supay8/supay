import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useQuery } from "@tanstack/react-query"

import { useAuth } from "../auth-context"
import { PageHeader, QueryErrorState } from "../components/shared/page-parts"
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "../components/ui/empty"
import { Input } from "../components/ui/input"
import { Label } from "../components/ui/label"
import { Skeleton } from "../components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table"

/**
 * Catálogo SIN sincronizado (solo lectura).
 * El backend no expone CRUD local de productos: la fuente de verdad es
 * GET /v1/companies/{id}/catalogs/productos-sin (sincronizado desde SIAT).
 * Patrón Adapter: listSinProducts devuelve el contrato real; la tabla lo muestra.
 */
export function ProductsPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const [search, setSearch] = useState("")
  const [debounced, setDebounced] = useState("")

  function handleSearch(value: string) {
    setSearch(value)
    window.clearTimeout((handleSearch as unknown as { t?: number }).t)
    ;(handleSearch as unknown as { t?: number }).t = window.setTimeout(() => {
      setDebounced(value.trim())
    }, 350)
  }

  const productsQuery = useQuery({
    queryKey: ["sin-products", companyId, debounced],
    queryFn: () =>
      host.listSinProducts
        ? host.listSinProducts(companyId, debounced || undefined, 50)
        : Promise.resolve({ items: [], total: 0, limit: 50, offset: 0 }),
    retry: false,
    enabled: companyId !== "",
    staleTime: 60_000,
  })

  const products = productsQuery.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Productos SIN"
        description="Catálogo oficial sincronizado desde el SIAT. Sin alta manual."
      />

      <Alert>
        <AlertTitle>Fuente SIAT, no inventario local</AlertTitle>
        <AlertDescription>
          Los códigos, actividades y unidades vienen de la sincronización
          (<code>POST /v1/siat/sincronizar</code>). Al facturar se resuelven por SKU.
        </AlertDescription>
      </Alert>

      <div className="flex max-w-md flex-col gap-1.5">
        <Label htmlFor="sin-search">Buscar en catálogo</Label>
        <Input
          id="sin-search"
          placeholder="Descripción o código…"
          value={search}
          onChange={(e) => handleSearch(e.target.value)}
        />
      </div>

      {productsQuery.isPending && (
        <div className="flex flex-col gap-2 rounded-lg border p-4">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {productsQuery.isError && (
        <QueryErrorState
          error={productsQuery.error}
          onRetry={() => productsQuery.refetch()}
        />
      )}

      {!productsQuery.isPending &&
        !productsQuery.isError &&
        (products.length === 0 ? (
          <Empty className="rounded-lg border border-dashed">
            <EmptyHeader>
              <EmptyTitle>Sin resultados</EmptyTitle>
              <EmptyDescription>
                Sincronizá catálogos en Conexión SIAT y volvé a buscar.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Descripción
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Código SIN
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Actividad
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {products.map((p, i) => (
                  <TableRow key={p.id ?? `${p.codigo}-${i}`}>
                    <TableCell className="font-medium">{p.descripcion}</TableCell>
                    <TableCell className="text-muted-foreground font-mono text-xs">
                      {p.codigo}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {p.codigo_actividad ?? "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        ))}
    </div>
  )
}

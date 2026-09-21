import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus } from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "../host"
import { useAuth } from "../auth-context"
import type { Product } from "../lib/types"
import { PageHeader, QueryErrorState } from "../components/shared/page-parts"
import { Button } from "../components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog"
import {
  Empty,
  EmptyContent,
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

export function ProductsPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState("")
  const [sku, setSku] = useState("")

  const productsQuery = useQuery({
    queryKey: ["products", companyId],
    queryFn: () => host.listProducts(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  const createMutation = useMutation({
    mutationFn: () =>
      host.createProduct({
        company_id: companyId,
        name: name.trim(),
        sku: sku.trim(),
        active: true,
      }),
    onSuccess: (product) => {
      queryClient.invalidateQueries({ queryKey: ["products"] })
      toast.success(`Producto ${product.name} creado`)
      setCreateOpen(false)
      setName("")
      setSku("")
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo crear el producto"
      ),
  })

  const products = productsQuery.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Productos"
        description="Tu catálogo local; la correspondencia SIAT evita rechazos por codificación."
        actions={
          <Button
            size="sm"
            onClick={() => {
              setName("")
              setSku("")
              setCreateOpen(true)
            }}
          >
            <Plus data-icon="inline-start" />
            Nuevo producto
          </Button>
        }
      />

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
              <EmptyTitle>Catálogo vacío</EmptyTitle>
              <EmptyDescription>
                Podés facturar igual con líneas libres, pero el catálogo acelera
                la emisión.
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                Crear producto
              </Button>
            </EmptyContent>
          </Empty>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Producto
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Código
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Correspondencia SIAT
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {products.map((p: Product) => {
                  const mapping = p.mappings?.find((m) => m.is_default) ?? p.mappings?.[0]
                  return (
                    <TableRow key={p.id}>
                      <TableCell className="font-medium">{p.name}</TableCell>
                      <TableCell className="text-muted-foreground font-mono text-xs">
                        {p.sku}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-xs">
                        {mapping
                          ? `SIN ${mapping.codigo_producto_sin} · act. ${mapping.codigo_actividad}`
                          : "sin mapeo — se asigna al sincronizar"}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        ))}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Nuevo producto</DialogTitle>
            <DialogDescription>
              La correspondencia de códigos SIAT se completa al sincronizar
              catálogos.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="prod-name">Nombre *</Label>
              <Input
                id="prod-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="prod-sku">Código interno *</Label>
              <Input
                id="prod-sku"
                value={sku}
                onChange={(e) => setSku(e.target.value)}
                placeholder="PROD-001"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancelar
            </Button>
            <Button
              disabled={
                name.trim() === "" || sku.trim() === "" || createMutation.isPending
              }
              onClick={() => createMutation.mutate()}
            >
              Crear producto
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

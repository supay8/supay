import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus } from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "../host"
import { useAuth } from "../auth-context"
import { formatDateTime } from "../lib/format"
import type { PointOfSale } from "../lib/types"
import { PageHeader, QueryErrorState } from "../components/shared/page-parts"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../components/ui/alert-dialog"
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu"
import { Input } from "../components/ui/input"
import { Label } from "../components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "../components/ui/sheet"
import { Skeleton } from "../components/ui/skeleton"
import { Spinner } from "../components/ui/spinner"
import { Switch } from "../components/ui/switch"

function HealthBadge({ pos }: { pos: PointOfSale }) {
  return pos.cuis ? (
    <span className="text-success inline-flex items-center gap-1.5 text-xs font-medium">
      <span className="bg-success size-1.5 rounded-full" />
      Lista para facturar
    </span>
  ) : (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium text-warning">
      <span className="bg-warning size-1.5 rounded-full" />
      Sin conexión SIAT
    </span>
  )
}

export function PointsOfSalePage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [branchIdForNew, setBranchIdForNew] = useState<string>("")
  const [description, setDescription] = useState("")
  const [detailPos, setDetailPos] = useState<PointOfSale | null>(null)
  const [reconnectingId, setReconnectingId] = useState<string | null>(null)

  const branchesQuery = useQuery({
    queryKey: ["branches", companyId],
    queryFn: () => host.listBranches(companyId),
    retry: false,
    enabled: companyId !== "",
  })
  const posQuery = useQuery({
    queryKey: ["point-of-sales", companyId],
    queryFn: () => host.listPointsOfSale(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["point-of-sales"] })
    queryClient.invalidateQueries({ queryKey: ["invoices"] })
  }

  const toggleMutation = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      host.updatePointOfSale(id, { is_active: active }),
    onSuccess: invalidate,
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo actualizar"
      ),
  })

  const reconnectMutation = useMutation({
    mutationFn: (id: string) => host.setupPointOfSale(id),
    onSuccess: () => {
      invalidate()
      setReconnectingId(null)
      toast.success("Punto de venta conectado")
    },
    onError: (error) => {
      setReconnectingId(null)
      toast.error(
        error instanceof ApiError
          ? error.message
          : "La conexión falló. Podés reintentar: no se duplica nada."
      )
    },
  })

  const createMutation = useMutation({
    mutationFn: () =>
      host.createPointOfSale({
        branch_id: branchIdForNew || undefined,
        name: description.trim(),
        description: description.trim(),
        is_active: true,
      }),
    onSuccess: () => {
      invalidate()
      toast.success("Punto de venta creado")
      setCreateOpen(false)
      setDescription("")
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError
          ? error.message
          : "No se pudo crear el punto de venta"
      ),
  })

  const branches = branchesQuery.data?.items ?? []
  const posList = posQuery.data?.items ?? []
  const branchName = (id?: string | null) =>
    branches.find((b) => b.id === id)?.name ?? `Sucursal ${posList.find((p) => p.id === id)?.codigo_sucursal ?? 0}`

  function openCreate() {
    setBranchIdForNew(branches[0]?.id ?? "")
    setDescription("")
    setCreateOpen(true)
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <PageHeader
        title="Puntos de venta"
        description="Cada punto factura con su propia firma diaria ante el SIAT."
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus data-icon="inline-start" />
            Nuevo punto de venta
          </Button>
        }
      />

      {(posQuery.isPending || branchesQuery.isPending) && (
        <div className="flex flex-col gap-3">
          {[...Array(2)].map((_, i) => (
            <Skeleton key={i} className="h-24 w-full rounded-lg" />
          ))}
        </div>
      )}

      {posQuery.isError && (
        <QueryErrorState error={posQuery.error} onRetry={() => posQuery.refetch()} />
      )}

      {!posQuery.isPending &&
        !posQuery.isError &&
        (posList.length === 0 ? (
          <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
            Todavía no hay puntos de venta. Creá el primero para empezar a
            facturar.
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {posList.map((pos) => (
              <article
                key={pos.id}
                className="rounded-lg border p-4 transition-colors hover:bg-muted/30"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex flex-col gap-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-medium">
                        {pos.description}
                      </span>
                      <HealthBadge pos={pos} />
                    </div>
                    <span className="text-muted-foreground font-mono text-[11px]">
                      {branchName(pos.branch_id)} · código{" "}
                      {pos.codigo_sucursal}.{pos.codigo_punto_venta}
                      {pos.cuis_created_at &&
                        ` · CUIS ${formatDateTime(pos.cuis_created_at)}`}
                    </span>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Switch
                      checked={pos.is_active}
                      disabled={toggleMutation.isPending}
                      onCheckedChange={(checked) =>
                        toggleMutation.mutate({ id: pos.id, active: checked === true })
                      }
                      aria-label={`Activar ${pos.description}`}
                    />
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Acciones" />}>
                        ⋯
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setDetailPos(pos)}>
                          Ver detalle
                        </DropdownMenuItem>
                        {!pos.cuis && (
                          <DropdownMenuItem
                            onClick={() => {
                              setReconnectingId(pos.id)
                              reconnectMutation.mutate(pos.id)
                            }}
                          >
                            Conectar ahora
                          </DropdownMenuItem>
                        )}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </div>
                {!pos.cuis && (
                  <p className="text-muted-foreground mt-2 text-xs">
                    Este punto todavía no está conectado con el SIAT.
                  </p>
                )}
              </article>
            ))}
          </div>
        ))}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Nuevo punto de venta</DialogTitle>
            <DialogDescription>
              Podés conectarlo con el SIAT después desde esta misma pantalla.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label>Sucursal</Label>
              <Select value={branchIdForNew} onValueChange={(v) => setBranchIdForNew(v ?? "")}>
                <SelectTrigger>
                  <SelectValue placeholder="Elegí sucursal" />
                </SelectTrigger>
                <SelectContent>
                  {branches.map((b) => (
                    <SelectItem key={b.id} value={b.id}>
                      {b.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="pos-desc">Descripción *</Label>
              <Input
                id="pos-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Caja 2"
                autoFocus
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancelar
            </Button>
            <Button
              disabled={description.trim() === "" || createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              Crear
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet open={detailPos !== null} onOpenChange={() => setDetailPos(null)}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader className="border-b px-5 py-4">
            <SheetTitle className="text-sm font-medium">
              {detailPos?.description}
            </SheetTitle>
          </SheetHeader>
          {detailPos && (
            <dl className="flex flex-col gap-2.5 p-5 text-[13px]">
              {[
                ["Sucursal / POS", `${detailPos.codigo_sucursal}.${detailPos.codigo_punto_venta}`],
                ["CUIS", detailPos.cuis ?? "—"],
                ["CUIS desde", detailPos.cuis_created_at ? formatDateTime(detailPos.cuis_created_at) : "—"],
                ["Registrado en SIAT", detailPos.siat_registered_at ? formatDateTime(detailPos.siat_registered_at) : "—"],
                ["Estado SIAT", detailPos.siat_error ?? "sin errores"],
              ].map(([k, v]) => (
                <div key={k} className="flex items-start justify-between gap-4">
                  <dt className="text-muted-foreground">{k}</dt>
                  <dd className="max-w-56 truncate text-right font-medium">{v}</dd>
                </div>
              ))}
            </dl>
          )}
        </SheetContent>
      </Sheet>

      <AlertDialog
        open={reconnectingId !== null && reconnectMutation.isPending}
        onOpenChange={() => setReconnectingId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Conectando con el SIAT…</AlertDialogTitle>
            <AlertDialogDescription className="flex items-center gap-2">
              <Spinner className="size-4" /> Obteniendo autorización y catálogos.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setReconnectingId(null)}>
              Cerrar
            </AlertDialogCancel>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

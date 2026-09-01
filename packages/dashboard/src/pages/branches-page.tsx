import { useState } from "react"
import { useDashboardHost } from "@/host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus } from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "@/host"
import { PLACEHOLDER_COMPANY_ID } from "@/lib/invoice-status"
import type { Branch } from "@/lib/types"
import { PageHeader, QueryErrorState } from "@/components/shared/page-parts"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

export function BranchesPage() {
  const host = useDashboardHost()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Branch | null>(null)
  const [name, setName] = useState("")
  const [address, setAddress] = useState("")
  const [deleting, setDeleting] = useState<Branch | null>(null)

  const branchesQuery = useQuery({
    queryKey: ["branches"],
    queryFn: () => host.listBranches(PLACEHOLDER_COMPANY_ID),
    retry: false,
  })

  function openCreate() {
    setEditing(null)
    setName("")
    setAddress("")
    setDialogOpen(true)
  }

  function openEdit(branch: Branch) {
    setEditing(branch)
    setName(branch.name)
    setAddress(branch.address)
    setDialogOpen(true)
  }

  const saveMutation = useMutation({
    mutationFn: () => {
      const payload = {
        company_id: PLACEHOLDER_COMPANY_ID,
        name: name.trim(),
        address: address.trim(),
      }
      return editing
        ? host.updateBranch(editing.id, payload)
        : host.createBranch(payload)
    },
    onSuccess: (branch) => {
      queryClient.invalidateQueries({ queryKey: ["branches"] })
      toast.success(editing ? "Sucursal actualizada" : `Sucursal ${branch.name} creada`)
      setDialogOpen(false)
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo guardar la sucursal"
      ),
  })

  const deleteMutation = useMutation({
    mutationFn: () => host.deleteBranch(deleting!.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["branches"] })
      toast.success("Sucursal eliminada")
      setDeleting(null)
    },
    onError: (error) => {
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo eliminar"
      )
      setDeleting(null)
    },
  })

  const branches = branchesQuery.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Sucursales"
        description="Ubicaciones físicas donde operan tus puntos de venta."
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus data-icon="inline-start" />
            Nueva sucursal
          </Button>
        }
      />

      {branchesQuery.isPending && (
        <div className="flex flex-col gap-2 rounded-lg border p-4">
          {[...Array(3)].map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {branchesQuery.isError && (
        <QueryErrorState
          error={branchesQuery.error}
          onRetry={() => branchesQuery.refetch()}
        />
      )}

      {!branchesQuery.isPending &&
        !branchesQuery.isError &&
        (branches.length === 0 ? (
          <Empty className="rounded-lg border border-dashed">
            <EmptyHeader>
              <EmptyTitle>Sin sucursales</EmptyTitle>
              <EmptyDescription>
                Creá la primera para poder registrar puntos de venta.
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button size="sm" onClick={openCreate}>
                Crear la primera
              </Button>
            </EmptyContent>
          </Empty>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Código
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Nombre
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Dirección
                  </TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {branches.map((branch) => (
                  <TableRow key={branch.id}>
                    <TableCell className="font-mono text-muted-foreground text-xs">
                      {String(branch.codigo_sucursal).padStart(2, "0")}
                    </TableCell>
                    <TableCell className="font-medium">{branch.name}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {branch.address || "—"}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => openEdit(branch)}
                        >
                          Editar
                        </Button>
                        <Button
                          variant="ghost"
                          size="xs"
                          className="text-destructive hover:text-destructive"
                          onClick={() => setDeleting(branch)}
                        >
                          Eliminar
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        ))}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>
              {editing ? `Editar ${editing.name}` : "Nueva sucursal"}
            </DialogTitle>
            <DialogDescription>
              El código se asigna automáticamente.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="branch-name">Nombre *</Label>
              <Input
                id="branch-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="branch-address">Dirección</Label>
              <Input
                id="branch-address"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialogOpen(false)}>
              Cancelar
            </Button>
            <Button
              disabled={name.trim() === "" || saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              Guardar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleting !== null} onOpenChange={() => setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>¿Eliminar {deleting?.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              Si la sucursal tiene puntos de venta asociados, la eliminación
              será rechazada por el sistema.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancelar</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={(event) => {
                event.preventDefault()
                deleteMutation.mutate()
              }}
            >
              Eliminar
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus } from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "../host"
import { useAuth } from "../auth-context"
import { formatDateTime } from "../lib/format"
import type { Customer } from "../lib/types"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select"
import { Skeleton } from "../components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table"

const DOCUMENT_TYPES = ["CI", "NIT", "CE", "PASAPORTE", "OTRO"]

export function CustomersPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState("")
  const [documentType, setDocumentType] = useState("CI")
  const [documentNumber, setDocumentNumber] = useState("")

  const customersQuery = useQuery({
    queryKey: ["customers", companyId],
    queryFn: () => host.listCustomers(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  const createMutation = useMutation({
    mutationFn: () =>
      host.createCustomer({
        company_id: companyId,
        name: name.trim(),
        document_type: documentType,
        document_number: documentNumber.trim(),
      }),
    onSuccess: (customer) => {
      queryClient.invalidateQueries({ queryKey: ["customers"] })
      toast.success(`Cliente ${customer.name} creado`)
      setCreateOpen(false)
      setName("")
      setDocumentNumber("")
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo crear el cliente"
      ),
  })

  const customers = customersQuery.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Clientes"
        description="Se crean solos al facturar; acá los administrás."
        actions={
          <Button
            size="sm"
            onClick={() => {
              setName("")
              setDocumentNumber("")
              setCreateOpen(true)
            }}
          >
            <Plus data-icon="inline-start" />
            Nuevo cliente
          </Button>
        }
      />

      {customersQuery.isPending && (
        <div className="flex flex-col gap-2 rounded-lg border p-4">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {customersQuery.isError && (
        <QueryErrorState
          error={customersQuery.error}
          onRetry={() => customersQuery.refetch()}
        />
      )}

      {!customersQuery.isPending &&
        !customersQuery.isError &&
        (customers.length === 0 ? (
          <Empty className="rounded-lg border border-dashed">
            <EmptyHeader>
              <EmptyTitle>Sin clientes</EmptyTitle>
              <EmptyDescription>
                Aparecerán automáticamente cuando emitas tu primera factura.
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                Crear uno manualmente
              </Button>
            </EmptyContent>
          </Empty>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Nombre
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Documento
                  </TableHead>
                  <TableHead className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
                    Alta
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {customers.map((c: Customer) => (
                  <TableRow key={c.id}>
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {c.document_type} {c.document_number}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {formatDateTime(c.created_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        ))}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Nuevo cliente</DialogTitle>
            <DialogDescription>Nombre y documento, lo mínimo.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="cust-name">Nombre o razón social *</Label>
              <Input
                id="cust-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
              />
            </div>
            <div className="grid grid-cols-[110px_1fr] gap-2">
              <div className="flex flex-col gap-1.5">
                <Label>Tipo</Label>
                <Select value={documentType} onValueChange={(v) => setDocumentType(v ?? "CI")}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {DOCUMENT_TYPES.map((t) => (
                      <SelectItem key={t} value={t}>
                        {t}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cust-doc">N° de documento *</Label>
                <Input
                  id="cust-doc"
                  value={documentNumber}
                  onChange={(e) => setDocumentNumber(e.target.value)}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancelar
            </Button>
            <Button
              disabled={
                name.trim() === "" ||
                documentNumber.trim() === "" ||
                createMutation.isPending
              }
              onClick={() => createMutation.mutate()}
            >
              Crear cliente
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

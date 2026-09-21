import { useDashboardHost } from "../host-context"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"

import { useAuth } from "../auth-context"
import { formatDateTime } from "../lib/format"
import type { Customer } from "../lib/types"
import { PageHeader, QueryErrorState } from "../components/shared/page-parts"
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert"
import { Button } from "../components/ui/button"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "../components/ui/empty"
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
 * Historial de receptores (solo lectura).
 * El backend no expone POST /customers: los clientes se crean implícito
 * al emitir (POST /v1/invoices[/emit] con objeto `customer`).
 */
export function CustomersPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const navigate = useNavigate()

  const customersQuery = useQuery({
    queryKey: ["customers", companyId],
    queryFn: () => host.listCustomers(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  const customers = customersQuery.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Clientes"
        description="Historial de receptores. Se crean solos al facturar."
        actions={
          <Button size="sm" onClick={() => navigate("/invoices/new")}>
            Facturar a un cliente nuevo
          </Button>
        }
      />

      <Alert>
        <AlertTitle>Sin alta manual</AlertTitle>
        <AlertDescription>
          El backend crea o reutiliza al cliente desde la factura
          (por NIT/CI o por <code>customer.id</code>). No hay endpoint de creación directa.
        </AlertDescription>
      </Alert>

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
              <Button size="sm" onClick={() => navigate("/invoices/new")}>
                Emitir primera factura
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
    </div>
  )
}

import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { Input } from "../components/ui/input"

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

  const all = customersQuery.data?.items ?? []
  const [q, setQ] = useState("")
  const customers = all.filter((c) => {
    const needle = q.trim().toLowerCase()
    if (!needle) return true
    return (
      c.name.toLowerCase().includes(needle) ||
      c.document_number.toLowerCase().includes(needle)
    )
  })

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Receptores"
        description="Historial de receptores facturados. Solo lectura desde el panel."
      />

      <Alert>
        <AlertTitle>Sin alta manual</AlertTitle>
        <AlertDescription>
          Tu ERP/POS crea o reutiliza al receptor al emitir
          (<span className="font-mono">POST /v1/invoices/emit</span>). Este panel no vende ni da de alta.
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

      <Input
        placeholder="Buscar por nombre o documento…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="max-w-md"
      />

      {!customersQuery.isPending &&
        !customersQuery.isError &&
        (customers.length === 0 ? (
          <Empty className="rounded-lg border border-dashed">
            <EmptyHeader>
              <EmptyTitle>Sin receptores</EmptyTitle>
              <EmptyDescription>
                Aparecerán automáticamente cuando tu ERP/POS emita la primera factura vía API.
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button size="sm" variant="outline" onClick={() => navigate("/company/api-keys")}>
                Ver API keys para integrar
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
                  <TableRow
                    key={c.id}
                    className="cursor-pointer"
                    onClick={() => navigate(`/invoices?q=${encodeURIComponent(c.document_number)}`)}
                  >
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {c.document_type?.toUpperCase()} {c.document_number}
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

import { useState } from "react"
import { useDashboardHost } from "@/host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Check, ChevronsUpDown, UserPlus } from "lucide-react"
import { toast } from "sonner"

import { ApiError } from "@/host"
import { PLACEHOLDER_COMPANY_ID } from "@/lib/invoice-status"
import type { Customer } from "@/lib/types"
import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

const DOCUMENT_TYPES = ["CI", "NIT", "CE", "PASAPORTE", "OTRO"]

function QuickCustomerDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (customer: Customer) => void
}) {
  const host = useDashboardHost()
  const [name, setName] = useState("")
  const [documentType, setDocumentType] = useState("CI")
  const [documentNumber, setDocumentNumber] = useState("")
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: () =>
      host.createCustomer({
        company_id: PLACEHOLDER_COMPANY_ID,
        name: name.trim(),
        document_type: documentType,
        document_number: documentNumber.trim(),
      }),
    onSuccess: (customer) => {
      queryClient.invalidateQueries({ queryKey: ["customers"] })
      toast.success(`Cliente ${customer.name} creado`)
      onCreated(customer)
      onOpenChange(false)
      setName("")
      setDocumentType("CI")
      setDocumentNumber("")
    },
    onError: (error) => {
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo crear el cliente"
      )
    },
  })

  const valid = name.trim() !== "" && documentNumber.trim() !== ""

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Cliente nuevo</DialogTitle>
          <DialogDescription>
            Mínimo para facturar: nombre y documento.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="customer-name">Nombre o razón social *</Label>
            <Input
              id="customer-name"
              data-field="customer.name"
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
              <Label htmlFor="customer-document">N° de documento *</Label>
              <Input
                id="customer-document"
                data-field="customer.document_number"
                value={documentNumber}
                onChange={(e) => setDocumentNumber(e.target.value)}
              />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancelar
          </Button>
          <Button
            disabled={!valid || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            Crear cliente
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function CustomerCombobox({
  value,
  onChange,
}: {
  value: Customer | null
  onChange: (customer: Customer | null) => void
}) {
  const host = useDashboardHost()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const [quickOpen, setQuickOpen] = useState(false)

  const customersQuery = useQuery({
    queryKey: ["customers"],
    queryFn: () => host.listCustomers(PLACEHOLDER_COMPANY_ID),
    staleTime: 60_000,
  })

  const customers = customersQuery.data?.items ?? []
  const needle = search.trim().toLowerCase()
  const filtered = needle
    ? customers.filter(
        (c) =>
          c.name.toLowerCase().includes(needle) ||
          c.document_number.toLowerCase().includes(needle)
      )
    : customers.slice(0, 20)

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              variant="outline"
              role="combobox"
              aria-expanded={open}
              className="w-full max-w-md justify-between font-normal"
            />
          }
        >
          {value ? (
            <span className="flex min-w-0 items-center gap-2">
              <span className="truncate">
                {value.name} · {value.document_type} {value.document_number}
              </span>
            </span>
          ) : (
            <span className="text-muted-foreground">
              Buscar por nombre, NIT o CI…
            </span>
          )}
          <ChevronsUpDown className="ml-auto size-4 shrink-0 opacity-50" />
        </PopoverTrigger>
        <PopoverContent className="w-[420px] p-0" align="start">
          <Command shouldFilter={false}>
            <CommandInput
              placeholder="Nombre, NIT o CI…"
              value={search}
              onValueChange={setSearch}
            />
            <CommandList>
              <CommandEmpty>Sin resultados.</CommandEmpty>
              <CommandGroup>
                {filtered.map((customer) => (
                  <CommandItem
                    key={customer.id}
                    value={customer.id}
                    onSelect={() => {
                      onChange(customer.id === value?.id ? null : customer)
                      setOpen(false)
                      setSearch("")
                    }}
                  >
                    <Check
                      className={cn(
                        "mr-1 size-4",
                        value?.id === customer.id ? "opacity-100" : "opacity-0"
                      )}
                    />
                    <span className="truncate">{customer.name}</span>
                    <span className="text-muted-foreground ml-auto text-xs">
                      {customer.document_type} {customer.document_number}
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
              <CommandGroup>
                <CommandItem
                  value="__nuevo"
                  onSelect={() => {
                    setOpen(false)
                    setQuickOpen(true)
                  }}
                >
                  <UserPlus className="mr-1 size-4" />
                  Crear cliente nuevo…
                </CommandItem>
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>

      <QuickCustomerDialog
        open={quickOpen}
        onOpenChange={setQuickOpen}
        onCreated={onChange}
      />
    </>
  )
}

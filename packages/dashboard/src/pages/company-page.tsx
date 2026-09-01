import { useState } from "react"
import { useDashboardHost } from "@/host-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { ApiError } from "@/host"
import { PLACEHOLDER_COMPANY_ID } from "@/lib/invoice-status"
import type { Company } from "@/lib/types"
import { FormSection, PageHeader, QueryErrorState } from "@/components/shared/page-parts"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"

export function CompanyPage() {
  const host = useDashboardHost()
  const queryClient = useQueryClient()
  const [businessName, setBusinessName] = useState("")
  const [nit, setNit] = useState("")
  const [municipio, setMunicipio] = useState("")
  const [direccion, setDireccion] = useState("")
  const [telefono, setTelefono] = useState("")
  const [snapshot, setSnapshot] = useState<Company | null>(null)

  const companyQuery = useQuery({
    queryKey: ["company", PLACEHOLDER_COMPANY_ID],
    queryFn: () => host.getCompany(PLACEHOLDER_COMPANY_ID),
    retry: false,
    staleTime: 60_000,
  })

  const company = companyQuery.data

  if (company && snapshot !== company) {
    setSnapshot(company)
    setBusinessName(company.business_name)
    setNit(company.nit)
    setMunicipio(company.municipio ?? "")
    setDireccion(company.direccion ?? "")
    setTelefono(company.telefono ?? "")
  }

  const saveMutation = useMutation({
    mutationFn: () =>
      host.updateCompany(PLACEHOLDER_COMPANY_ID, {
        business_name: businessName.trim(),
        nit: nit.trim(),
        municipio: municipio.trim() || undefined,
        direccion: direccion.trim() || undefined,
        telefono: telefono.trim() || undefined,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["company"] })
      toast.success("Datos de la empresa actualizados")
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo guardar"
      ),
  })

  const dirty =
    company !== undefined &&
    (businessName !== company.business_name ||
      nit !== company.nit ||
      municipio !== (company.municipio ?? "") ||
      direccion !== (company.direccion ?? "") ||
      telefono !== (company.telefono ?? ""))

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <PageHeader
        title="Datos fiscales"
        description="Identidad de tu empresa ante el SIAT."
      />

      {companyQuery.isPending && (
        <div className="flex flex-col gap-3 rounded-lg border p-5">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      )}

      {companyQuery.isError && (
        <QueryErrorState
          error={companyQuery.error}
          onRetry={() => companyQuery.refetch()}
        />
      )}

      {company && (
        <>
          <FormSection title="Identificación">
            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="co-name">Razón social</Label>
                <Input
                  id="co-name"
                  value={businessName}
                  onChange={(e) => setBusinessName(e.target.value)}
                />
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="co-nit">NIT</Label>
                  <Input
                    id="co-nit"
                    value={nit}
                    onChange={(e) => setNit(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="co-municipio">Municipio</Label>
                  <Input
                    id="co-municipio"
                    value={municipio}
                    onChange={(e) => setMunicipio(e.target.value)}
                  />
                </div>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="co-direccion">Dirección</Label>
                  <Input
                    id="co-direccion"
                    value={direccion}
                    onChange={(e) => setDireccion(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="co-telefono">Teléfono</Label>
                  <Input
                    id="co-telefono"
                    value={telefono}
                    onChange={(e) => setTelefono(e.target.value)}
                  />
                </div>
              </div>
            </div>
          </FormSection>

          <FormSection
            title="Certificado digital"
            description="Se administra durante la configuración inicial."
          >
            <div className="flex items-center justify-between gap-4 text-sm">
              <span className="text-muted-foreground">
                Administrado en Conexión SIAT
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.location.assign("/company/siat")}
              >
                Ver conexión
              </Button>
            </div>
          </FormSection>

          <div className="flex justify-end">
            <Button
              disabled={!dirty || saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              Guardar cambios
            </Button>
          </div>
        </>
      )}
    </div>
  )
}

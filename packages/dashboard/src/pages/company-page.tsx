import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { useDashboardHost } from "../host-context"
import { useAuth } from "../auth-context"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { ApiError } from "../host"
import type { Company } from "../lib/types"
import { FormSection, PageHeader } from "../components/shared/page-parts"
import { Button } from "../components/ui/button"
import { Input } from "../components/ui/input"
import { Label } from "../components/ui/label"

export function CompanyPage() {
  const host = useDashboardHost()
  const navigate = useNavigate()
  const { activeCompany, refreshCompanies } = useAuth()
  const queryClient = useQueryClient()
  const [businessName, setBusinessName] = useState("")
  const [nit, setNit] = useState("")
  const [municipio, setMunicipio] = useState("")
  const [direccion, setDireccion] = useState("")
  const [telefono, setTelefono] = useState("")
  const [snapshot, setSnapshot] = useState<Company | null>(null)

  const company = activeCompany?.company ?? null

  useEffect(() => {
    if (company && snapshot?.id !== company.id) {
      setSnapshot(company)
      setBusinessName(company.business_name)
      setNit(company.nit)
      setMunicipio(company.municipio ?? "")
      setDireccion(company.direccion ?? "")
      setTelefono(company.telefono ?? "")
    }
  }, [company, snapshot])

  const saveMutation = useMutation({
    mutationFn: () =>
      host.updateCompany(company!.id, {
        business_name: businessName.trim(),
        nit: nit.trim(),
        municipio: municipio.trim() || undefined,
        direccion: direccion.trim() || undefined,
        telefono: telefono.trim() || undefined,
      }),
    onSuccess: async () => {
      await refreshCompanies()
      queryClient.invalidateQueries({ queryKey: ["company"] })
      toast.success("Datos de la empresa actualizados")
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo guardar"
      ),
  })

  const dirty =
    company !== null &&
    (businessName !== company.business_name ||
      nit !== company.nit ||
      municipio !== (company.municipio ?? "") ||
      direccion !== (company.direccion ?? "") ||
      telefono !== (company.telefono ?? ""))

  if (!company) {
    return (
      <div className="mx-auto flex max-w-2xl flex-col gap-6">
        <PageHeader title="Datos fiscales" description="Identidad de tu empresa ante el SIAT." />
        <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
          Sin empresa activa. Elegí una en Empresas o creá la primera.
        </div>
        <div className="flex justify-center">
          <Button onClick={() => navigate("/companies")}>Ir a Empresas</Button>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <PageHeader
        title="Datos fiscales"
        description={`${company.business_name} · NIT ${company.nit} · ${company.ambiente}`}
      />

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
            title="Configuración SIAT"
            description="Solo lectura: se define al crear la empresa y en Actividad económica."
          >
            <dl className="grid gap-2 text-sm sm:grid-cols-2">
              <div className="flex justify-between gap-2 rounded-md border p-3">
                <dt className="text-muted-foreground">Código sistema</dt>
                <dd className="font-mono text-xs">{company.codigo_sistema || "—"}</dd>
              </div>
              <div className="flex justify-between gap-2 rounded-md border p-3">
                <dt className="text-muted-foreground">Ambiente</dt>
                <dd className="font-medium">{company.ambiente}</dd>
              </div>
              <div className="flex justify-between gap-2 rounded-md border p-3">
                <dt className="text-muted-foreground">Actividad</dt>
                <dd className="font-medium">{company.codigo_actividad ?? "Sin definir"}</dd>
              </div>
              <div className="flex justify-between gap-2 rounded-md border p-3">
                <dt className="text-muted-foreground">Alta</dt>
                <dd className="text-muted-foreground text-xs">{company.created_at}</dd>
              </div>
            </dl>
            <div className="mt-2 flex justify-end">
              <Button variant="outline" size="sm" onClick={() => navigate("/company/sectors")}>
                Cambiar actividad
              </Button>
            </div>
          </FormSection>

          <FormSection
            title="API keys"
            description="Claves para integrar tu ERP o POS con esta empresa."
          >
            <div className="flex items-center justify-between gap-4 text-sm">
              <span className="text-muted-foreground">
                Crea y revoca claves de acceso.
              </span>
              <Button variant="outline" size="sm" onClick={() => navigate("/company/api-keys")}>
                Gestionar keys
              </Button>
            </div>
          </FormSection>

          <FormSection
            title="Certificado digital"
            description="Firma .p12 de la empresa. Se gestiona en Seguridad."
          >
            <div className="flex items-center justify-between gap-4 text-sm">
              <span className="text-muted-foreground">
                Un activo por NIT. Solo se muestran metadatos.
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => navigate("/company/certificates")}
              >
                Gestionar certificado
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

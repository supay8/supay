import { useEffect, useMemo, useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import { Check, FileKey2, LoaderCircle } from "lucide-react"
import { toast } from "sonner"

import { ApiError, api } from "@/lib/api"
import type { Company, SectorInfo } from "@/lib/types"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { cn } from "@/lib/utils"

const STEPS = [
  { short: "Empresa", title: "Datos de la empresa" },
  { short: "Punto de venta", title: "Sucursal y punto de venta" },
  { short: "Actividad", title: "Actividad económica" },
  { short: "Certificado", title: "Certificado digital" },
] as const

const CHECKLIST = [
  { label: "Registro de tu empresa en Supay", hint: null },
  { label: "Código de autorización diario", hint: "CUIS" },
  { label: "Catálogos oficiales sincronizados", hint: null },
  { label: "Código de firma del día", hint: "CUFD" },
] as const

function TechHint({ term }: { term: string }) {
  const explanations: Record<string, string> = {
    CUIS: "Código Único de Inicio de Vigencia que entrega el SIAT a cada punto de venta. Se renueva periódicamente.",
    CUFD: "Código Único de Facturación Diaria: firma con la que se emiten las facturas del día.",
  }
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span className="underline decoration-dotted underline-offset-2" />
        }
      >
        ({term})
      </TooltipTrigger>
      <TooltipContent className="max-w-64">
        {explanations[term]}
      </TooltipContent>
    </Tooltip>
  )
}

function Stepper({
  step,
  onStepClick,
}: {
  step: number
  onStepClick: (index: number) => void
}) {
  return (
    <ol className="flex flex-wrap items-center gap-2">
      {STEPS.map((s, i) => {
        const done = i < step
        const current = i === step
        return (
          <li key={s.short} className="flex items-center gap-2">
            <button
              type="button"
              disabled={i >= step}
              onClick={() => onStepClick(i)}
              className={cn(
                "flex items-center gap-2 rounded-md text-xs font-medium",
                i < step && "cursor-pointer"
              )}
            >
              <span
                className={cn(
                  "flex size-5 shrink-0 items-center justify-center rounded-full text-[10px]",
                  done && "bg-success text-white",
                  current && "bg-primary text-primary-foreground",
                  !done && !current && "border text-muted-foreground"
                )}
              >
                {done ? <Check className="size-3" /> : i + 1}
              </span>
              <span className={cn(!current && !done && "text-muted-foreground")}>
                {s.short}
              </span>
            </button>
            {i < STEPS.length - 1 && <span className="h-px w-6 bg-border" />}
          </li>
        )
      })}
    </ol>
  )
}

export function SetupPage() {
  const navigate = useNavigate()
  const [step, setStep] = useState(0)

  const [businessName, setBusinessName] = useState("")
  const [nit, setNit] = useState("")
  const [municipio, setMunicipio] = useState("")
  const [direccion, setDireccion] = useState("")
  const [telefono, setTelefono] = useState("")

  const [branchName, setBranchName] = useState("Casa Matriz")
  const [posDescription, setPosDescription] = useState("Caja 1")
  const [companyId, setCompanyId] = useState<string | null>(null)
  const [posId, setPosId] = useState<string | null>(null)

  const [sectorCodigo, setSectorCodigo] = useState<number | null>(null)
  const [certName, setCertName] = useState("")
  const [connectPhase, setConnectPhase] = useState<
    "idle" | "connecting" | "success" | "error"
  >("idle")

  const sectoresQuery = useQuery({
    queryKey: ["sectores"],
    queryFn: () => api.invoices.sectores(),
    staleTime: 5 * 60_000,
  })

  const sectores = useMemo(
    () => (sectoresQuery.data ?? []).filter((s) => s.habilitado),
    [sectoresQuery]
  )
  const selectedSector: SectorInfo | undefined =
    sectores.find((s) => s.codigo === sectorCodigo)

  useEffect(() => {
    if (connectPhase !== "success") return
    const timer = setTimeout(() => navigate("/invoices"), 1800)
    return () => clearTimeout(timer)
  }, [connectPhase, navigate])

  const companyMutation = useMutation({
    mutationFn: async (): Promise<Company> => {
      const payload = {
        business_name: businessName.trim(),
        nit: nit.trim(),
        ambiente: "PILOTO" as const,
        municipio: municipio.trim() || undefined,
        direccion: direccion.trim() || undefined,
        telefono: telefono.trim() || undefined,
      }
      if (companyId) return api.companies.update(companyId, payload)
      return api.companies.create(payload)
    },
    onSuccess: (company) => {
      setCompanyId(company.id)
      setStep(1)
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError ? error.message : "No se pudo guardar la empresa"
      ),
  })

  const locationMutation = useMutation({
    mutationFn: async () => {
      const branch = await api.branches.create({
        company_id: companyId!,
        codigo_sucursal: 0,
        name: branchName.trim(),
        address: direccion.trim() || "",
        active: true,
      })
      const pos = await api.pointOfSale.create({
        company_id: companyId!,
        branch_id: branch.id,
        description: posDescription.trim(),
        is_active: true,
      })
      return pos
    },
    onSuccess: (pos) => {
      setPosId(pos.id)
      setStep(2)
    },
    onError: (error) =>
      toast.error(
        error instanceof ApiError
          ? error.message
          : "No se pudo crear el punto de venta"
      ),
  })

  const connectMutation = useMutation({
    mutationFn: () =>
      api.companies.setup(companyId!, {
        point_of_sale_id: posId,
        codigo_documento_sector: sectorCodigo,
      }),
    onSuccess: () => setConnectPhase("success"),
    onError: (error) => {
      setConnectPhase("error")
      toast.error(
        error instanceof ApiError ? error.message : "La conexión falló"
      )
    },
  })

  const step1Valid = businessName.trim() !== "" && nit.trim() !== ""
  const step2Valid =
    companyId !== null && branchName.trim() !== "" && posDescription.trim() !== ""
  const step3Valid = sectorCodigo !== null
  const step4Valid = certName !== ""

  function handleConnect() {
    setConnectPhase("connecting")
    connectMutation.mutate()
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-8 pb-16">
      <div className="flex flex-col gap-3">
        <h1 className="text-lg font-semibold tracking-tight">
          Configurar tu empresa
        </h1>
        <p className="text-muted-foreground -mt-2 text-sm">
          Cuatro pasos y una conexión. Podés reintentar la conexión con
          seguridad: nunca se duplica nada.
        </p>
        <Stepper step={step} onStepClick={setStep} />
      </div>

      {step === 0 && (
        <section className="flex flex-col gap-4 rounded-lg border p-6">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="setup-business">Razón social *</Label>
            <Input
              id="setup-business"
              data-field="business_name"
              value={businessName}
              onChange={(e) => setBusinessName(e.target.value)}
              placeholder="Comercial Andina SRL"
              autoFocus
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="setup-nit">NIT *</Label>
              <Input
                id="setup-nit"
                data-field="nit"
                value={nit}
                onChange={(e) => setNit(e.target.value)}
                placeholder="1020304015"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="setup-municipio">Municipio</Label>
              <Input
                id="setup-municipio"
                value={municipio}
                onChange={(e) => setMunicipio(e.target.value)}
                placeholder="La Paz"
              />
            </div>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="setup-direccion">Dirección</Label>
              <Input
                id="setup-direccion"
                value={direccion}
                onChange={(e) => setDireccion(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="setup-telefono">Teléfono</Label>
              <Input
                id="setup-telefono"
                value={telefono}
                onChange={(e) => setTelefono(e.target.value)}
              />
            </div>
          </div>
          <div className="flex justify-end pt-1">
            <Button
              disabled={!step1Valid || companyMutation.isPending}
              onClick={() => companyMutation.mutate()}
            >
              {companyMutation.isPending && (
                <Spinner data-icon="inline-start" />
              )}
              Continuar
            </Button>
          </div>
        </section>
      )}

      {step === 1 && (
        <section className="flex flex-col gap-4 rounded-lg border p-6">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="setup-branch">Sucursal *</Label>
              <Input
                id="setup-branch"
                value={branchName}
                onChange={(e) => setBranchName(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="setup-pos">Punto de venta *</Label>
              <Input
                id="setup-pos"
                value={posDescription}
                onChange={(e) => setPosDescription(e.target.value)}
                placeholder="Caja 1"
              />
            </div>
          </div>
          <p className="text-muted-foreground text-xs">
            El código de sucursal y de punto de venta se asignan
            automáticamente.
          </p>
          <div className="flex items-center justify-between pt-1">
            <Button variant="ghost" size="sm" onClick={() => setStep(0)}>
              Atrás
            </Button>
            <Button
              disabled={!step2Valid || locationMutation.isPending}
              onClick={() => locationMutation.mutate()}
            >
              {locationMutation.isPending && (
                <Spinner data-icon="inline-start" />
              )}
              Continuar
            </Button>
          </div>
        </section>
      )}

      {step === 2 && (
        <section className="flex flex-col gap-4 rounded-lg border p-6">
          <p className="text-muted-foreground text-xs">
            Elegí el rubro que corresponde a tu actividad. Define qué campos te
            pedirá el sistema al facturar.
          </p>
          {sectoresQuery.isPending && (
            <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
              <Spinner className="size-4" />
              Cargando actividades del SIAT…
            </div>
          )}
          <div className="grid max-h-80 gap-2 overflow-y-auto pr-1 sm:grid-cols-2">
            {sectores.map((sector) => {
              const selected = sectorCodigo === sector.codigo
              return (
                <button
                  key={sector.codigo}
                  type="button"
                  onClick={() => setSectorCodigo(sector.codigo)}
                  className={cn(
                    "rounded-md border p-3 text-left text-sm transition-colors",
                    selected
                      ? "border-ring ring-ring ring-1"
                      : "hover:bg-muted/50"
                  )}
                >
                  <span className="font-medium">{sector.label}</span>
                  {sector.campos && sector.campos.length > 0 && (
                    <span className="text-muted-foreground mt-1 block text-xs">
                      Te pedirá:{" "}
                      {sector.campos
                        .filter((c) => c.requerido)
                        .map((c) => c.label)
                        .join(", ") || "nada extra"}
                    </span>
                  )}
                </button>
              )
            })}
          </div>
          <div className="flex items-center justify-between pt-1">
            <Button variant="ghost" size="sm" onClick={() => setStep(1)}>
              Atrás
            </Button>
            <Button
              disabled={!step3Valid}
              onClick={() => setStep(3)}
            >
              Continuar
            </Button>
          </div>
        </section>
      )}

      {step === 3 && (
        <section className="flex flex-col gap-4 rounded-lg border p-6">
          <label
            className={cn(
              "flex cursor-pointer flex-col items-center gap-2 rounded-lg border border-dashed p-8 text-center transition-colors hover:bg-muted/50",
              certName && "border-solid"
            )}
          >
            <FileKey2 className="text-muted-foreground size-6" />
            <input
              type="file"
              accept=".p12,.pfx"
              className="hidden"
              onChange={(e) => setCertName(e.target.files?.[0]?.name ?? "")}
            />
            {certName ? (
              <span className="font-mono text-xs">{certName}</span>
            ) : (
              <>
                <span className="text-sm font-medium">
                  Cargá tu certificado digital (.p12 / .pfx)
                </span>
                <span className="text-muted-foreground text-xs">
                  Es el archivo que te dio Impuestos para firmar facturas.
                </span>
              </>
            )}
          </label>

          <ul className="flex flex-col gap-2 border-t pt-4 text-sm">
            <li className="flex justify-between gap-2">
              <span className="text-muted-foreground">Empresa</span>
              <span className="font-medium">{businessName}</span>
            </li>
            <li className="flex justify-between gap-2">
              <span className="text-muted-foreground">Punto de venta</span>
              <span className="font-medium">{posDescription}</span>
            </li>
            <li className="flex justify-between gap-2">
              <span className="text-muted-foreground">Actividad</span>
              <span className="font-medium">
                {selectedSector?.label ?? "—"}
              </span>
            </li>
          </ul>

          {(connectPhase === "connecting" ||
            connectPhase === "success" ||
            connectPhase === "error") && (
            <ul className="flex flex-col gap-2 rounded-lg bg-surface p-4 text-sm">
              {CHECKLIST.map((item, i) => {
                return (
                  <li key={item.label} className="flex items-center gap-2">
                    {connectPhase === "success" ? (
                      <Check className="text-success size-4" />
                    ) : connectPhase === "connecting" ? (
                      <LoaderCircle className="text-muted-foreground size-4 animate-spin" />
                    ) : connectPhase === "error" && i === 1 ? (
                      <span className="bg-destructive size-1.5 rounded-full" />
                    ) : (
                      <span className="bg-border size-1.5 rounded-full" />
                    )}
                    <span className={cn(connectPhase !== "success" && "text-muted-foreground")}>
                      {item.label}{" "}
                      {item.hint && <TechHint term={item.hint} />}
                    </span>
                  </li>
                )
              })}
            </ul>
          )}

          {connectPhase === "error" && (
            <p className="text-destructive text-xs">
              Algo falló durante la conexión. Podés reintentar con seguridad:
              no se duplicará nada.
            </p>
          )}

          <div className="flex items-center justify-between pt-1">
            <Button
              variant="ghost"
              size="sm"
              disabled={connectPhase === "connecting"}
              onClick={() => setStep(2)}
            >
              Atrás
            </Button>
            {connectPhase === "success" ? (
              <Badge className="bg-success h-7 gap-1.5 px-3 text-xs text-white">
                <Check className="size-3.5" />
                Lista para facturar · redirigiendo…
              </Badge>
            ) : (
              <Button
                disabled={!step4Valid || connectMutation.isPending}
                onClick={handleConnect}
              >
                {connectMutation.isPending && (
                  <Spinner data-icon="inline-start" />
                )}
                Conectar con SIAT
              </Button>
            )}
          </div>
        </section>
      )}

      {step === 0 && companyId !== null && (
        <p className="text-muted-foreground text-center text-xs">
          Empresa guardada. Los cambios se actualizan sobre el mismo registro.
        </p>
      )}
    </div>
  )
}

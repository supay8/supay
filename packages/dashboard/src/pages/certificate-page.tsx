import { useMemo, useState } from "react"
import { useDashboardHost } from "../host-context"
import { useAuth } from "../auth-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FileKey2, ShieldCheck, Trash2, Upload } from "lucide-react"
import { toast } from "sonner"

import { HostError } from "../host"
import { formatDate } from "../lib/format"
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
import { Badge } from "../components/ui/badge"
import { Button } from "../components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog"
import { Input } from "../components/ui/input"
import { Label } from "../components/ui/label"
import { Skeleton } from "../components/ui/skeleton"

const MAX_BYTES = 5 << 20

function daysLeft(notAfter?: string | null): number | null {
  if (!notAfter) return null
  const diff = new Date(notAfter).getTime() - Date.now()
  return Math.ceil(diff / 86_400_000)
}

export function CertificatePage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const queryClient = useQueryClient()

  const [uploadOpen, setUploadOpen] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [certName, setCertName] = useState("")
  const [password, setPassword] = useState("")
  const [revokingId, setRevokingId] = useState<string | null>(null)

  const certsQuery = useQuery({
    queryKey: ["certificates", companyId],
    queryFn: () => host.listCertificates(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  const activeQuery = useQuery({
    queryKey: ["certificate-active", companyId],
    queryFn: () => host.getActiveCertificate(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["certificates", companyId] })
    queryClient.invalidateQueries({ queryKey: ["certificate-active", companyId] })
  }

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error("Seleccioná el archivo .p12")
      return host.uploadCertificate(companyId, {
        file,
        name: certName.trim() || undefined,
        password,
      })
    },
    onSuccess: () => {
      setUploadOpen(false)
      setFile(null)
      setCertName("")
      setPassword("")
      invalidate()
      toast.success("Certificado subido y activado")
    },
    onError: (error) =>
      toast.error(
        error instanceof HostError ? error.message : "No se pudo subir el certificado"
      ),
  })

  const revokeMutation = useMutation({
    mutationFn: (certId: string) => host.revokeCertificate(companyId, certId),
    onSuccess: () => {
      setRevokingId(null)
      invalidate()
      toast.success("Certificado revocado")
    },
    onError: (error) => {
      setRevokingId(null)
      toast.error(
        error instanceof HostError ? error.message : "No se pudo revocar"
      )
    },
  })

  const certs = useMemo(
    () => (Array.isArray(certsQuery.data) ? certsQuery.data : []),
    [certsQuery.data]
  )
  const active = activeQuery.data ?? certs.find((c) => c.is_active) ?? null
  const left = daysLeft(active?.not_after)
  const expiring = left !== null && left <= 30

  const fileError = useMemo(() => {
    if (!file) return null
    const lower = file.name.toLowerCase()
    if (!lower.endsWith(".p12") && !lower.endsWith(".pfx"))
      return "El archivo debe ser .p12 o .pfx"
    if (file.size > MAX_BYTES) return "El archivo excede 5MB"
    if (file.size === 0) return "El archivo está vacío"
    return null
  }, [file])

  const canUpload =
    file !== null && password.trim() !== "" && !fileError && !uploadMutation.isPending

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4">
      <PageHeader
        title="Certificado digital"
        description={`Firma de ${activeCompany?.company.business_name ?? "la empresa"} (un activo por NIT). Solo se muestran metadatos, nunca la clave.`}
        actions={
          <Button size="sm" onClick={() => setUploadOpen(true)}>
            <Upload data-icon="inline-start" />
            Subir certificado
          </Button>
        }
      />

      {(certsQuery.isPending || activeQuery.isPending) && (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-24 w-full rounded-lg" />
          <Skeleton className="h-16 w-full rounded-lg" />
        </div>
      )}

      {(certsQuery.isError || activeQuery.isError) && (
        <QueryErrorState
          error={(certsQuery.error ?? activeQuery.error)!}
          onRetry={() => {
            certsQuery.refetch()
            activeQuery.refetch()
          }}
        />
      )}

      {!certsQuery.isPending && !certsQuery.isError && !active && (
        <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
          Todavía no hay certificado activo. Subí el .p12 que te dio Impuestos
          para poder firmar facturas.
        </div>
      )}

      {active && (
        <article className="flex flex-col gap-2 rounded-lg border p-4">
          <div className="flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <ShieldCheck className="size-4 shrink-0 text-muted-foreground" />
              <div className="flex min-w-0 flex-col gap-0.5">
                <span className="truncate text-sm font-medium">
                  {active.name || active.subject || "Certificado activo"}
                </span>
                <span className="text-muted-foreground font-mono text-[11px]">
                  {active.not_after
                    ? `vence ${formatDate(active.not_after)}`
                    : "vigencia desconocida"}
                  {left !== null && ` · ${left} días restantes`}
                </span>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              {expiring ? (
                <Badge variant="outline">Por vencer</Badge>
              ) : (
                <Badge variant="outline">Activo</Badge>
              )}
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label="Revocar certificado"
                onClick={() => setRevokingId(active.id)}
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          </div>
          {(active.issuer || active.subject) && (
            <p className="text-muted-foreground truncate font-mono text-[11px]">
              {[active.subject, active.issuer].filter(Boolean).join(" · ")}
            </p>
          )}
          {expiring && (
            <p className="text-xs text-warning">
              Quedan 30 días o menos. Renovalo antes del vencimiento para no
              interrumpir la facturación.
            </p>
          )}
        </article>
      )}

      {certs.length > 0 && (
        <div className="flex flex-col gap-3">
          <h2 className="text-sm font-medium text-muted-foreground">Historial</h2>
          {certs.map((cert) => (
            <article
              key={cert.id}
              className="flex items-center justify-between gap-3 rounded-lg border p-4"
            >
              <div className="flex min-w-0 items-center gap-3">
                <FileKey2 className="size-4 shrink-0 text-muted-foreground" />
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-sm font-medium">
                    {cert.name || cert.subject || cert.id.slice(0, 8)}
                  </span>
                  <span className="text-muted-foreground font-mono text-[11px]">
                    {cert.not_after ? formatDate(cert.not_after) : "sin fecha"} ·{" "}
                    {cert.status}
                  </span>
                </div>
              </div>
              {cert.is_active ? (
                <Badge variant="outline">Activo</Badge>
              ) : (
                <Badge variant="outline" className="text-muted-foreground">
                  {cert.status}
                </Badge>
              )}
            </article>
          ))}
        </div>
      )}

      <Dialog open={uploadOpen} onOpenChange={setUploadOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Subir certificado</DialogTitle>
            <DialogDescription>
              Archivo .p12/.pfx (máx 5MB) de la empresa. Se cifra en el servidor
              y nunca se vuelve a mostrar.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="cert-file">Archivo .p12 / .pfx *</Label>
              <Input
                id="cert-file"
                type="file"
                accept=".p12,.pfx"
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              />
              {file && !fileError && (
                <span className="font-mono text-xs text-muted-foreground">
                  {file.name} · {(file.size / 1024).toFixed(0)} KB
                </span>
              )}
              {fileError && (
                <span className="text-xs text-destructive">{fileError}</span>
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="cert-name">Nombre (opcional)</Label>
              <Input
                id="cert-name"
                value={certName}
                onChange={(e) => setCertName(e.target.value)}
                placeholder="Certificado NIT 2026"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="cert-pass">Contraseña del .p12 *</Label>
              <Input
                id="cert-pass"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="off"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setUploadOpen(false)}>
              Cancelar
            </Button>
            <Button disabled={!canUpload} onClick={() => uploadMutation.mutate()}>
              Subir y activar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={revokingId !== null} onOpenChange={() => setRevokingId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revocar certificado</AlertDialogTitle>
            <AlertDialogDescription>
              Se conserva para auditoría pero dejará de firmar de inmediato. Subí
              el reemplazo cuanto antes.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setRevokingId(null)}>
              Cancelar
            </AlertDialogCancel>
            <Button
              variant="destructive"
              disabled={revokeMutation.isPending}
              onClick={() => revokingId && revokeMutation.mutate(revokingId)}
            >
              Revocar
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

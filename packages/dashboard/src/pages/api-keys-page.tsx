import { useState } from "react"
import { useDashboardHost } from "../host-context"
import { useAuth } from "../auth-context"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, KeyRound, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { HostError, type CreateApiKeyResponse } from "../host"
import { formatDateTime } from "../lib/format"
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

export function ApiKeysPage() {
  const host = useDashboardHost()
  const { activeCompany } = useAuth()
  const companyId = activeCompany?.company.id ?? ""
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [keyName, setKeyName] = useState("")
  const [created, setCreated] = useState<CreateApiKeyResponse | null>(null)
  const [revokingId, setRevokingId] = useState<string | null>(null)

  const keysQuery = useQuery({
    queryKey: ["api-keys", companyId],
    queryFn: () => host.listApiKeys(companyId),
    retry: false,
    enabled: companyId !== "",
  })

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["api-keys", companyId] })
  }

  const createMutation = useMutation({
    mutationFn: () => host.createApiKey(companyId, keyName.trim()),
    onSuccess: (res) => {
      setCreated(res)
      setKeyName("")
      invalidate()
    },
    onError: (error) =>
      toast.error(
        error instanceof HostError ? error.message : "No se pudo crear la key"
      ),
  })

  const revokeMutation = useMutation({
    mutationFn: (keyId: string) => host.revokeApiKey(companyId, keyId),
    onSuccess: () => {
      setRevokingId(null)
      invalidate()
      toast.success("Key revocada")
    },
    onError: (error) => {
      setRevokingId(null)
      toast.error(
        error instanceof HostError ? error.message : "No se pudo revocar"
      )
    },
  })

  const keys = keysQuery.data ?? []

  async function copySecret() {
    if (!created) return
    try {
      await navigator.clipboard.writeText(created.api_key)
      toast.success("Clave copiada")
    } catch {
      toast.error("No se pudo copiar al portapapeles")
    }
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4">
      <PageHeader
        title="API keys"
        description={`Claves de ${activeCompany?.company.business_name ?? "la empresa"} para integrar tu ERP o POS.`}
        actions={
          <Button size="sm" onClick={() => { setCreated(null); setKeyName(""); setCreateOpen(true) }}>
            <Plus data-icon="inline-start" />
            Nueva key
          </Button>
        }
      />

      {keysQuery.isPending && (
        <div className="flex flex-col gap-3">
          {[...Array(2)].map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      )}

      {keysQuery.isError && (
        <QueryErrorState error={keysQuery.error} onRetry={() => keysQuery.refetch()} />
      )}

      {!keysQuery.isPending && !keysQuery.isError && keys.length === 0 && (
        <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
          Todavía no hay keys. Creá la primera para conectar tu sistema.
        </div>
      )}

      {!keysQuery.isPending && keys.length > 0 && (
        <div className="flex flex-col gap-3">
          {keys.map((key) => (
            <article key={key.id} className="flex items-center justify-between gap-3 rounded-lg border p-4">
              <div className="flex min-w-0 items-center gap-3">
                <KeyRound className="text-muted-foreground size-4 shrink-0" />
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-sm font-medium">{key.name}</span>
                  <span className="text-muted-foreground font-mono text-[11px]">
                    {key.key_prefix}… ·{" "}
                    {key.last_used_at ? `usada ${formatDateTime(key.last_used_at)}` : "sin uso"}
                  </span>
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {key.is_active ? (
                  <Badge variant="outline">Activa</Badge>
                ) : (
                  <Badge variant="outline" className="text-muted-foreground">Revocada</Badge>
                )}
                {key.is_active && (
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Revocar ${key.name}`}
                    onClick={() => setRevokingId(key.id)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                )}
              </div>
            </article>
          ))}
        </div>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Nueva API key</DialogTitle>
            <DialogDescription>
              Nómbrala según el sistema que la usará (ej. “POS Caja 1”).
            </DialogDescription>
          </DialogHeader>
          {created ? (
            <div className="flex flex-col gap-3">
              <p className="text-sm font-medium">Guardala ahora: no se mostrará de nuevo.</p>
              <code className="bg-muted block max-w-full overflow-x-auto rounded-md p-3 font-mono text-xs break-all">
                {created.api_key}
              </code>
              <DialogFooter>
                <Button variant="ghost" onClick={() => setCreateOpen(false)}>
                  Cerrar
                </Button>
                <Button onClick={copySecret}>
                  <Copy data-icon="inline-start" />
                  Copiar
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="key-name">Nombre *</Label>
                <Input
                  id="key-name"
                  value={keyName}
                  onChange={(e) => setKeyName(e.target.value)}
                  placeholder="POS Caja 1"
                  autoFocus
                />
              </div>
              <DialogFooter>
                <Button variant="ghost" onClick={() => setCreateOpen(false)}>
                  Cancelar
                </Button>
                <Button
                  disabled={keyName.trim() === "" || createMutation.isPending}
                  onClick={() => createMutation.mutate()}
                >
                  Crear
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog open={revokingId !== null} onOpenChange={() => setRevokingId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revocar key</AlertDialogTitle>
            <AlertDialogDescription>
              El sistema que la usa dejará de autenticarse de inmediato. Esta acción no se puede deshacer.
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

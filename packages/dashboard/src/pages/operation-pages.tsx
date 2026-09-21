import { PageHeader } from "../components/shared/page-parts"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "../components/ui/empty"
import { Button } from "../components/ui/button"
import { Boxes, ShoppingCart } from "lucide-react"
import { useNavigate } from "react-router-dom"

function PendingModule({
  icon,
  title,
  description,
  cta,
}: {
  icon: React.ReactNode
  title: string
  description: string
  cta?: { label: string; to: string }
}) {
  const navigate = useNavigate()
  return (
    <div className="mx-auto flex max-w-xl flex-col gap-6">
      <PageHeader title={title} />
      <Empty className="rounded-lg border border-dashed">
        <EmptyHeader>
          <EmptyMedia variant="icon">{icon}</EmptyMedia>
          <EmptyTitle>Disponible próximamente</EmptyTitle>
          <EmptyDescription>{description}</EmptyDescription>
        </EmptyHeader>
        {cta && (
          <EmptyContent>
            <Button variant="outline" size="sm" onClick={() => navigate(cta.to)}>
              {cta.label}
            </Button>
          </EmptyContent>
        )}
      </Empty>
    </div>
  )
}

export function LotesPage() {
  return (
    <PendingModule
      icon={<Boxes />}
      title="Lotes (envío masivo)"
      description="Envío y validación de paquetes de facturas. Se activa cuando el contrato v1 exponga los endpoints de paquetes."
      cta={{ label: "Ir a facturas", to: "/invoices" }}
    />
  )
}

export function ComprasPage() {
  return (
    <PendingModule
      icon={<ShoppingCart />}
      title="Compras"
      description="Registro de facturas de proveedores recibidas. Se activa cuando el contrato v1 exponga el módulo de compras."
      cta={{ label: "Ir a facturas", to: "/invoices" }}
    />
  )
}

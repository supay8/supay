import { Component, type ReactNode } from "react"
import { Button } from "@/components/ui/button"

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
}

export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-svh flex-col items-center justify-center gap-3 p-6 text-center">
          <p className="text-sm font-medium">Algo salió mal en la interfaz.</p>
          <p className="text-muted-foreground max-w-md break-all font-mono text-xs">
            {this.state.error.message}
          </p>
          <Button size="sm" onClick={() => window.location.reload()}>
            Recargar
          </Button>
        </div>
      )
    }
    return this.props.children
  }
}

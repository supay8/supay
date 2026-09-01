import * as React from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ThemeProvider } from "@/components/theme-provider"
import { ErrorBoundary } from "@/components/error-boundary"
import { TooltipProvider } from "@/components/ui/tooltip"
import { Toaster } from "@/components/ui/sonner"

export interface DashboardProvidersProps {
  children: React.ReactNode
  queryClient?: QueryClient
}

export function DashboardProviders({ children, queryClient }: DashboardProvidersProps) {
  const [client] = React.useState(() =>
    queryClient ?? new QueryClient({
      defaultOptions: {
        queries: {
          retry: 1,
          refetchOnWindowFocus: false,
        },
      },
    })
  )

  return (
    <ThemeProvider>
      <QueryClientProvider client={client}>
        <TooltipProvider>
          <ErrorBoundary>{children}</ErrorBoundary>
          <Toaster position="top-center" richColors />
        </TooltipProvider>
      </QueryClientProvider>
    </ThemeProvider>
  )
}

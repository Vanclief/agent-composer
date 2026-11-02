import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from 'next-themes'
import { BrowserRouter } from 'react-router'

import { ApiError } from '@/api/client'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'

import { App } from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // A 4xx will answer the same again; show it instead of retrying.
      retry: (failures, error) => {
        const clientError = error instanceof ApiError && error.status >= 400 && error.status < 500
        return !clientError && failures < 2
      },
      refetchOnWindowFocus: false,
    },
  },
})

createRoot(document.getElementById('root') as HTMLElement).render(
  <StrictMode>
    <ThemeProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem={false}
      storageKey="agc.panel.theme"
      disableTransitionOnChange
    >
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <BrowserRouter basename="/panel">
            <App />
          </BrowserRouter>
          {/* Below the 54px top bar, so live notifications (top-right) leave
              its live dot and speaker toggle visible. */}
          <Toaster offset={{ top: 62, right: 16, bottom: 24, left: 24 }} />
        </TooltipProvider>
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
)

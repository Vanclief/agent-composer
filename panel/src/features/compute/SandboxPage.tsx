// /compute/:name: one sandbox, its spec and its terminals (ADR 0008).
// TODO(ADR 0008): placeholder; the compute feature replaces it (useSandbox,
// useStopSandbox, useStartSandbox, useDeleteSandbox, and the terminals from
// src/features/terminals/).
import { useParams } from 'react-router'

import { usePageTitle } from '@/shell/page-title'

export function SandboxPage() {
  const { name = '' } = useParams()
  usePageTitle(name)
  return (
    <div className="p-6">
      <h2 className="text-lg font-semibold">SandboxPage</h2>
    </div>
  )
}

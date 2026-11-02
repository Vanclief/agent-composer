// /compute: every sandbox with its task (ADR 0008).
// TODO(ADR 0008): placeholder; the compute feature replaces it (useSandboxes).
import { usePageTitle } from '@/shell/page-title'

export function ComputePage() {
  usePageTitle('Compute')
  return (
    <div className="p-6">
      <h2 className="text-lg font-semibold">ComputePage</h2>
    </div>
  )
}

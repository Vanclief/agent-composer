// PLACEHOLDER owned by the tasks feature: replace freely, keep the export.
import { dump } from 'js-yaml'
import { useParams } from 'react-router'

import { useTask } from '@/api/hooks'

export function SpecTab() {
  const { name = '' } = useParams()
  const task = useTask(name)

  return (
    <div className="flex flex-col gap-3 p-6">
      <h3 className="text-lg font-semibold">SpecTab</h3>
      {task.data && (
        <pre className="overflow-auto rounded-md border border-zinc-200 p-3 font-mono text-xs dark:border-zinc-800">
          {dump(task.data.spec)}
        </pre>
      )}
    </div>
  )
}

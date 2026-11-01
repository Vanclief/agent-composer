// PLACEHOLDER owned by the terminals feature: replace freely, keep the
// export. The socket URL and frame protocol are in src/api/terminals.ts.
import { useParams } from 'react-router'

import { useTask, useTerminals } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'

export function TerminalsTab() {
  const { name = '' } = useParams()
  const task = useTask(name)
  const running = task.data?.status === 'running'
  const terminals = useTerminals(name, running)

  return (
    <div className="flex flex-col gap-3 p-6">
      <h3 className="text-lg font-semibold">TerminalsTab</h3>
      <ErrorText error={terminals.error} />
      <div className="flex flex-wrap gap-2">
        {terminals.data?.map((job) => (
          <IconChip key={job.id} icon={conceptIcons.terminal} title={job.running ? 'running' : 'exited'}>
            {job.command.join(' ')}
          </IconChip>
        ))}
        {task.data && !running && <EmptySlot reason={`Task ${task.data.status}`} />}
      </div>
    </div>
  )
}

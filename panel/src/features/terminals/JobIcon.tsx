// A job's icon: the task icon for the job "agent" (the program the sandbox
// was allocated to run for its task), the terminal icon for the rest.
import type { Terminal } from '@/api/terminals'
import { conceptIcons } from '@/components/concept-icons'

export const AGENT_JOB = 'agent'

export function JobIcon({ job, className }: { job: Terminal; className?: string }) {
  if (job.id === AGENT_JOB) {
    return <conceptIcons.task aria-hidden data-icon="task" className={className} />
  }
  return <conceptIcons.terminal aria-hidden data-icon="terminal" className={className} />
}

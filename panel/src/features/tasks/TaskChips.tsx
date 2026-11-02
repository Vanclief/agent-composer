// The chips that say who works on a task, who manages it, what it is for
// and what compute it has. Shared by the list cards and the task header.
import { Link } from 'react-router'

import { useKnowledge } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { cn } from '@/lib/utils'

import { assignedToHarness } from './format'

// A harness with its model (`claude_code · claude-opus-5-5`), or a person.
export function AssigneeChip({ task }: { task: TaskView }) {
  if (!task.assignee) {
    return (
      <span
        data-testid="assignee"
        className="inline-flex items-center rounded-md border border-dashed border-zinc-300 px-1.5 py-0.5 font-mono text-xs text-zinc-500 dark:border-zinc-700"
      >
        unassigned
      </span>
    )
  }
  if (assignedToHarness(task)) {
    return (
      <span data-testid="assignee" data-kind="harness" className="inline-flex">
        <IconChip
          icon={conceptIcons.task}
          concept="task"
          title={task.model ? `Agent ${task.assignee}, model ${task.model}` : `Agent ${task.assignee}`}
        >
          {task.model ? `${task.assignee} · ${task.model}` : task.assignee}
        </IconChip>
      </span>
    )
  }
  return (
    <span data-testid="assignee" data-kind="human" className="inline-flex">
      <IconChip icon={conceptIcons.human} concept="human" title={`Assigned to ${task.assignee}`}>
        {task.assignee}
      </IconChip>
    </span>
  )
}

// The messaging name agc addresses when the task opens (ADR 0009).
export function ManagerChip({ task }: { task: TaskView }) {
  if (!task.manager) {
    return null
  }
  return (
    <span data-testid="manager" className="inline-flex">
      <IconChip icon={conceptIcons.human} concept="human" title={`manager ${task.manager}`}>
        @{task.manager}
      </IconChip>
    </span>
  )
}

// The knowledge action or decision the task serves, linking to it in Think.
// An id that is not a decision is taken for an action.
export function ForChip({ task }: { task: TaskView }) {
  const knowledge = useKnowledge()
  if (!task.for) {
    return null
  }
  const decision = knowledge.data?.decisions.some((d) => d.id === task.for) ?? false
  const concept = decision ? 'decision' : 'action'
  const Icon = conceptIcons[concept]
  const color = conceptColors[concept]
  return (
    <Link
      to={`/think/${decision ? 'decisions' : 'actions'}/${encodeURIComponent(task.for)}`}
      data-testid="for"
      title={`For the ${concept} ${task.for}`}
      className={cn(
        'relative z-10 inline-flex max-w-72 items-center gap-1 rounded-md border px-1.5 py-0.5 font-mono text-xs hover:underline',
        color.bg,
        color.border,
        color.text,
      )}
    >
      <Icon aria-hidden className="size-3.5 shrink-0" />
      <span className="truncate">{task.for}</span>
    </Link>
  )
}

// How many sandboxes the task has and the newest one's status, verbatim.
export function SandboxSummary({ task }: { task: TaskView }) {
  const Icon = conceptIcons.sandbox
  const newest = task.sandboxes[0]
  if (!newest) {
    return (
      <span
        data-testid="sandbox-summary"
        title="No sandbox allocated"
        className="inline-flex items-center gap-1 rounded-md border border-dashed border-zinc-300 px-1.5 py-0.5 font-mono text-xs text-zinc-500 dark:border-zinc-700"
      >
        <Icon aria-hidden className="size-3.5 shrink-0" />
        no sandbox
      </span>
    )
  }
  const count = task.sandboxes.length
  return (
    <span
      data-testid="sandbox-summary"
      title={`${count} ${count === 1 ? 'sandbox' : 'sandboxes'}; newest ${newest.name}`}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border py-0.5 pr-0.5 pl-1.5 font-mono text-xs',
        conceptColors.sandbox.bg,
        conceptColors.sandbox.border,
        conceptColors.sandbox.text,
      )}
    >
      <Icon aria-hidden className="size-3.5 shrink-0" />
      <span data-testid="sandbox-count" className="tabular-nums">
        {count}
      </span>
      <StatusPill status={newest.status} detail={newest.status_error} className="bg-background py-0" />
    </span>
  )
}

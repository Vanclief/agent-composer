// The chips that say who works on a task, who manages it, what it is for
// Used by the task header; list rows (TaskRow) use muted text instead.
import { Link } from 'react-router'

import { useKnowledge } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { Avatar } from '@/components/avatar'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
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

// ADR 0009: the manager created the task and runs it day to day (an agent
// or a person); the owner is the human it answers to. agc addresses both
// when the task opens.
export function ManagerChip({ task, avatar }: { task: TaskView; avatar?: boolean }) {
  // The manager may be an agent; it is shown as human only when it is the
  // owner, who always is one.
  return <PersonChip role="manager" name={task.manager} avatar={avatar} human={task.manager === task.owner} />
}

export function OwnerChip({ task, avatar }: { task: TaskView; avatar?: boolean }) {
  return <PersonChip role="owner" name={task.owner} avatar={avatar} human />
}

// avatar: the task header's look, an xs identicon before the name; the
// list keeps the plain chip.
function PersonChip({
  role,
  name,
  avatar,
  human,
}: {
  role: 'manager' | 'owner'
  name: string
  avatar?: boolean
  human: boolean
}) {
  if (!name) {
    return null
  }
  if (avatar) {
    return (
      <span
        data-testid={role}
        title={`${role} ${name}`}
        className="inline-flex items-center gap-1 rounded-md border border-zinc-200 py-0.5 pr-1.5 pl-0.5 font-mono text-xs text-zinc-700 dark:border-zinc-800 dark:text-zinc-300"
      >
        <Avatar name={name} size="xs" human={human} />
        <span className="truncate">@{name}</span>
      </span>
    )
  }
  return (
    <span data-testid={role} className="inline-flex">
      <IconChip icon={conceptIcons.human} concept="human" title={`${role} ${name}`}>
        @{name}
      </IconChip>
    </span>
  )
}

// The knowledge action or decision the task serves, linking to it in Think.
// An id that is not a decision is taken for an action. plain drops the chip
// for muted text (list rows).
export function ForChip({ task, plain = false }: { task: TaskView; plain?: boolean }) {
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
        'relative z-10 inline-flex max-w-72 items-center gap-1 font-mono text-xs hover:underline',
        plain
          ? 'text-muted-foreground hover:text-foreground'
          : ['rounded-md border px-1.5 py-0.5', color.bg, color.border, color.text],
      )}
    >
      <Icon aria-hidden className={cn('size-3.5 shrink-0', plain && color.text, plain && 'opacity-70')} />
      <span className="truncate">{task.for}</span>
    </Link>
  )
}

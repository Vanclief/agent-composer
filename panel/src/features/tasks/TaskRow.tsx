// One task in the list as a flat row (panel/CLAUDE.md "Lists are rows, not
// cards"): the status light, the title, the name in dim mono and the age;
// under it, muted, who works on it (harness · model or a person), manager,
// owner and what it is for, then the compute as the only chip, since it is
// live. The whole row opens the task; the links in it sit above that.
// When the task waits on me (attention.tsx) the row has a lit amber mark at
// its left edge and a chip per act after the pill; when it waits on someone
// else, their name follows the pill, dim.
import { Link } from 'react-router'

import type { TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { StatusPill, statusTier } from '@/components/status-pill'
import { cn } from '@/lib/utils'

import { ActChip, AttentionMark, WaitsOn, type Attention } from './attention'
import { assignedToHarness, shortAge } from './format'
import { ForChip } from './TaskChips'

export function TaskRow({ task, attention }: { task: TaskView; attention?: Attention }) {
  const mine = Boolean(attention?.mine.length)
  const other = !mine && Boolean(attention?.others.length)
  // Waiting on me sorts into the attention group (TasksPage).
  const tier = mine ? 'attention' : statusTier(task.status)
  const off = tier === 'off'

  return (
    <li
      data-task={task.name}
      data-status={task.status}
      data-tier={tier}
      data-attention={mine ? 'me' : other ? 'other' : undefined}
      className="relative flex flex-col gap-1 px-3 py-2 text-sm transition-colors focus-within:z-10 hover:bg-zinc-500/5 has-[a:focus-visible]:rounded-md has-[a:focus-visible]:ring-1 has-[a:focus-visible]:ring-violet-500/70 dark:hover:bg-zinc-500/10"
    >
      {mine && <AttentionMark />}
      <div className="flex items-center gap-3">
        <div data-testid="task-status" className="flex w-20 shrink-0">
          <StatusPill status={task.status} />
        </div>
        <Link
          to={`/tasks/${encodeURIComponent(task.name)}`}
          className={cn(
            'truncate outline-none after:absolute after:inset-0 after:content-[""]',
            off && 'text-muted-foreground',
          )}
        >
          {task.title}
        </Link>
        <span title="Name, also the channel" className="truncate font-mono text-xs text-zinc-400 dark:text-zinc-500">
          {task.name}
        </span>
        <div data-testid="task-acts" className="ml-auto flex shrink-0 items-center gap-1.5">
          {other && <WaitsOn names={attention?.others ?? []} />}
          {attention?.mine.map((act) => (
            <ActChip key={act.act} act={act} />
          ))}
          <span
            title={`Created ${new Date(task.created_at).toLocaleString()}`}
            className="relative z-10 ml-2 shrink-0 font-mono text-xs text-muted-foreground tabular-nums"
          >
            {shortAge(task.created_at)}
          </span>
        </div>
      </div>
      <div
        className={cn(
          'flex flex-wrap items-center gap-x-3 gap-y-1 pl-[5.75rem] text-xs text-muted-foreground',
          off && 'opacity-70',
        )}
      >
        <Assignee task={task} />
        <Person role="manager" name={task.manager} />
        <Person role="owner" name={task.owner} />
        <ForChip task={task} plain />
        <ComputeChip task={task} />
      </div>
    </li>
  )
}

// harness · model, or a person with the human icon; plain muted text.
function Assignee({ task }: { task: TaskView }) {
  if (!task.assignee) {
    return (
      <span data-testid="assignee" className="italic">
        unassigned
      </span>
    )
  }
  if (assignedToHarness(task)) {
    return (
      <span
        data-testid="assignee"
        data-kind="harness"
        title={task.model ? `Agent ${task.assignee}, model ${task.model}` : `Agent ${task.assignee}`}
        className="font-mono"
      >
        {task.model ? `${task.assignee} · ${task.model}` : task.assignee}
      </span>
    )
  }
  return (
    <span data-testid="assignee" data-kind="human" className="inline-flex items-center gap-1">
      <conceptIcons.human aria-hidden className="size-3 shrink-0" />
      <span title={`Assigned to ${task.assignee}`}>{task.assignee}</span>
    </span>
  )
}

// ADR 0009: the manager runs the task day to day, the owner is the human it
// answers to.
function Person({ role, name }: { role: 'manager' | 'owner'; name: string }) {
  if (!name) {
    return null
  }
  return (
    <span data-testid={role} className="inline-flex items-center gap-1">
      <conceptIcons.human aria-hidden className="size-3 shrink-0" />
      <span title={`${role} ${name}`}>@{name}</span>
    </span>
  )
}

// How many sandboxes the task has and the newest one's light, linking to
// /compute; "no sandbox" as muted text when there is none.
function ComputeChip({ task }: { task: TaskView }) {
  const Icon = conceptIcons.sandbox
  const newest = task.sandboxes[0]
  if (!newest) {
    return (
      <span data-testid="sandbox-summary" title="No sandbox allocated" className="font-mono">
        no sandbox
      </span>
    )
  }
  const count = task.sandboxes.length
  return (
    <Link
      to="/compute"
      data-testid="sandbox-summary"
      title={`${count} ${count === 1 ? 'sandbox' : 'sandboxes'}; newest ${newest.name}`}
      className={cn(
        'relative z-10 inline-flex items-center gap-1.5 rounded-md border py-0.5 pr-0.5 pl-1.5 font-mono text-xs hover:brightness-125',
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
    </Link>
  )
}

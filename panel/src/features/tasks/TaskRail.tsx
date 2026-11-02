// The task page's facts, top to bottom: the actions, status and title, the
// name and channel, who works on it, delivered and accepted lines, the
// criteria it is accepted on (ADR 0010), the brief, what the agent received
// (ADR 0009 item 6) and the delivery summary. A rail on the right on wide
// screens, a card above the box on narrow ones.
import { useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { Check, ChevronDown, ChevronRight, CircleCheck, Copy } from 'lucide-react'

import type { TaskView } from '@/api/tasks'
import { Avatar } from '@/components/avatar'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { ActChip, useAttention } from './attention'
import { ago, assignedToHarness } from './format'
import { TaskActions, type useTaskActions } from './TaskActions'
import { ForChip } from './TaskChips'

type TaskRailProps = {
  task: TaskView
  actions: ReturnType<typeof useTaskActions>
  error?: unknown
  onDeleted: () => void
  className?: string
}

export function TaskRail({ task, actions, error, onDeleted, className }: TaskRailProps) {
  // Narrow screens only: the long sections fold away so the box stays near
  // the top. Wide screens always show them.
  const [details, setDetails] = useState(false)
  const Chevron = details ? ChevronDown : ChevronRight
  return (
    <aside data-testid="task-header" aria-label="Task facts" className={cn('flex flex-col gap-4 text-sm', className)}>
      <TaskActions task={task} actions={actions} onDeleted={onDeleted} className="justify-end" />
      <div className="flex flex-col gap-1.5">
        <div className="flex items-start gap-2">
          <StatusPill status={task.status} className="mt-1 shrink-0" />
          <h2 className="min-w-0 text-xl leading-tight font-semibold tracking-tight break-words">{task.title}</h2>
        </div>
        <p title="Name and channel" className="font-mono text-xs text-muted-foreground">
          {task.name} · #{task.name}
        </p>
        <Waiting task={task} />
      </div>
      <Who task={task} />
      <Moments task={task} />
      <ErrorText error={error} />
      <Button
        variant="ghost"
        size="xs"
        aria-expanded={details}
        onClick={() => setDetails(!details)}
        className="self-start px-1 text-muted-foreground min-[1200px]:hidden"
      >
        <Chevron /> {details ? 'Hide details' : 'Details: criteria, brief, summary'}
      </Button>
      <div className={cn('flex-col gap-4 min-[1200px]:flex', details ? 'flex' : 'hidden')}>
        <Criteria task={task} />
        <Brief task={task} />
        <Received task={task} />
        <Delivery task={task} />
      </div>
    </aside>
  )
}

// Who the task waits on, under the status: "Waiting on you" lit amber with
// the acts, or "Waiting on @name" dim; nothing when it waits on nobody.
function Waiting({ task }: { task: TaskView }) {
  const attention = useAttention()(task)
  if (attention.mine.length > 0) {
    return (
      <p
        data-testid="waiting-on"
        data-attention="me"
        className="flex flex-wrap items-center gap-1.5 text-sm font-medium text-amber-700 dark:text-amber-400"
      >
        <span aria-hidden className="block size-2 rounded-full bg-amber-500 shadow-[0_0_6px_1px] shadow-amber-500/60" />
        Waiting on you:
        {attention.mine.map((act) => (
          <ActChip key={act.act} act={act} />
        ))}
      </p>
    )
  }
  if (attention.others.length > 0) {
    return (
      <p data-testid="waiting-on" data-attention="other" className="text-sm text-muted-foreground">
        Waiting on {attention.others.map((n) => '@' + n).join(', ')}
      </p>
    )
  }
  return null
}

// One fact: a small dim icon, then the value as plain muted text.
// lead replaces the icon (the manager's and owner's avatars).
function Fact({
  icon: Icon,
  lead,
  children,
  title,
}: {
  icon?: typeof Check
  lead?: ReactNode
  children: ReactNode
  title?: string
}) {
  return (
    <li title={title} className="flex min-w-0 items-center gap-2 text-muted-foreground">
      {lead ?? (Icon && <Icon aria-hidden className="size-3.5 shrink-0" />)}
      <span className="min-w-0 truncate">{children}</span>
    </li>
  )
}

// Harness · model (or the person), manager, owner, what it is for and
// whether it notifies. Plain text, not chips: nothing here is lit.
function Who({ task }: { task: TaskView }) {
  const harness = assignedToHarness(task)
  return (
    <ul aria-label="Who" className="flex flex-wrap gap-x-5 gap-y-1 min-[1200px]:flex-col min-[1200px]:flex-nowrap">
      <Fact
        icon={harness ? conceptIcons.agent : conceptIcons.human}
        title={harness ? 'Agent harness and model' : 'Assigned to'}
      >
        <span data-testid="assignee" data-kind={task.assignee ? (harness ? 'harness' : 'human') : undefined}>
          {task.assignee ? (task.model ? `${task.assignee} · ${task.model}` : task.assignee) : 'unassigned'}
        </span>
      </Fact>
      {task.manager && (
        <Fact
          // The manager may be an agent; shown as human only when it is the
          // owner, who always is one (as TaskChips' ManagerChip).
          lead={<Avatar name={task.manager} size="xs" human={task.manager === task.owner} className="-mx-px" />}
          title={`manager ${task.manager}`}
        >
          <span data-testid="manager" className="text-foreground/80">
            @{task.manager}
          </span>{' '}
          <span className="text-xs">manager</span>
        </Fact>
      )}
      {task.owner && (
        <Fact lead={<Avatar name={task.owner} size="xs" human className="-mx-px" />} title={`owner ${task.owner}`}>
          <span data-testid="owner" className="text-foreground/80">
            @{task.owner}
          </span>{' '}
          <span className="text-xs">owner</span>
        </Fact>
      )}
      {task.notify && (
        <Fact icon={conceptIcons.notify} title="Notify: toast and sound on start and done">
          <span data-testid="notify-chip">notify</span>
        </Fact>
      )}
      {task.for && (
        <li className="pt-1">
          <ForChip task={task} />
        </li>
      )}
    </ul>
  )
}

// Delivered <when> until accepted or rejected; Accepted by <name> <when>.
function Moments({ task }: { task: TaskView }) {
  const done = task.status === 'done'
  if (!task.delivered_at && !done) {
    return null
  }
  return (
    <ul aria-label="Delivery and acceptance" className="flex flex-col gap-1">
      {task.delivered_at && !done ? (
        <li
          data-testid="delivered-chip"
          title={`Delivered ${new Date(task.delivered_at).toLocaleString()}`}
          className="flex items-center gap-2 text-amber-700 dark:text-amber-400"
        >
          <conceptIcons.delivery aria-hidden className="size-3.5 shrink-0" />
          Delivered {ago(task.delivered_at)}
        </li>
      ) : null}
      {done ? (
        <li
          data-testid="accepted"
          title={task.done_at ? new Date(task.done_at).toLocaleString() : undefined}
          className="flex items-center gap-2 text-muted-foreground"
        >
          <CircleCheck aria-hidden className="size-3.5 shrink-0" />
          {task.accepted_by ? `Accepted by ${task.accepted_by}` : 'Accepted'}
          {task.done_at ? ` ${ago(task.done_at)}` : ''}
        </li>
      ) : null}
    </ul>
  )
}

// A rail section: a hairline on top, a small heading with its concept icon.
function Section({
  label,
  region = label,
  icon: Icon,
  color,
  extra,
  children,
}: {
  label: string
  // The section's accessible name, when it differs from its heading.
  region?: string
  icon: typeof Check
  color: string
  extra?: ReactNode
  children: ReactNode
}) {
  return (
    <section aria-label={region} className="flex flex-col gap-2 border-t border-border pt-3">
      <header className="flex items-center gap-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
        <Icon aria-hidden className={cn('size-3.5', color)} />
        <h3>{label}</h3>
        {extra}
      </header>
      {children}
    </section>
  )
}

// The criteria as unchecked boxes: agc does not evaluate them; the manager
// or the owner does when accepting. Hidden when there are none.
function Criteria({ task }: { task: TaskView }) {
  if (task.accept.length === 0) {
    return null
  }
  return (
    <Section label="Accepted when" icon={conceptIcons.criteria} color={conceptColors.criteria.text}>
      <ul data-testid="criteria" className="flex flex-col gap-1.5">
        {task.accept.map((criterion, i) => (
          <li key={i} data-testid="criterion" className="flex items-start gap-2">
            <input
              type="checkbox"
              checked={false}
              disabled
              readOnly
              aria-label={criterion}
              className="mt-0.5 size-4 shrink-0 accent-zinc-500"
            />
            <span>{criterion}</span>
          </li>
        ))}
      </ul>
      <p className="text-xs text-muted-foreground">agc does not check these; the manager or owner does</p>
    </Section>
  )
}

// The brief, verbatim, clamped to its first lines until "Show more".
function Brief({ task }: { task: TaskView }) {
  const [open, setOpen] = useState(false)
  const [clipped, setClipped] = useState(false)
  const ref = useRef<HTMLPreElement>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (el && !open) {
      setClipped(el.scrollHeight > el.clientHeight + 1)
    }
  }, [task.brief, open])

  return (
    <Section label="Brief" icon={conceptIcons.brief} color={conceptColors.brief.text}>
      {task.brief ? (
        <>
          <pre
            ref={ref}
            data-testid="brief"
            className={cn('font-sans text-sm leading-relaxed whitespace-pre-wrap', !open && 'line-clamp-6')}
          >
            {task.brief}
          </pre>
          {(clipped || open) && (
            <Button variant="link" size="xs" className="self-start px-0" onClick={() => setOpen(!open)}>
              {open ? 'Show less' : 'Show more'}
            </Button>
          )}
        </>
      ) : (
        <EmptySlot reason="No brief" />
      )}
    </Section>
  )
}

// The exact TASK.md the newest sandbox's agent received, behind a toggle.
function Received({ task }: { task: TaskView }) {
  const [open, setOpen] = useState(false)
  const newest = task.sandboxes[0]
  const Chevron = open ? ChevronDown : ChevronRight
  return (
    <Section
      label="What the agent received"
      icon={conceptIcons.sandbox}
      color={conceptColors.sandbox.text}
      extra={newest && <CopyButton text={newest.instructions} className="ml-auto normal-case" />}
    >
      {newest ? (
        <>
          <Button
            variant="ghost"
            size="xs"
            aria-expanded={open}
            onClick={() => setOpen(!open)}
            className="self-start px-1 font-mono text-muted-foreground"
          >
            <Chevron /> TASK.md of {newest.name}
          </Button>
          {open && (
            <pre
              data-testid="instructions"
              className="max-h-96 overflow-auto rounded-md bg-zinc-50 px-3 py-2 font-mono text-xs leading-relaxed whitespace-pre-wrap dark:bg-zinc-950/60"
            >
              {newest.instructions}
            </pre>
          )}
        </>
      ) : (
        <EmptySlot reason="No sandbox allocated yet" />
      )}
    </Section>
  )
}

// The agent's last delivery summary, kept after a reject. Hidden when the
// task was never delivered nor accepted.
function Delivery({ task }: { task: TaskView }) {
  const done = task.status === 'done'
  if (!task.summary && !done) {
    return null
  }
  return (
    <Section label="Summary" region="Delivery" icon={conceptIcons.delivery} color={conceptColors.delivery.text}>
      {task.summary ? (
        <pre data-testid="summary" className="font-sans text-sm leading-relaxed whitespace-pre-wrap">
          {task.summary}
        </pre>
      ) : (
        <EmptySlot reason="Accepted without a delivery" />
      )}
    </Section>
  )
}

// Copies text; the icon turns into a check for a moment, or the label says
// the browser refused.
function CopyButton({ text, className }: { text: string; className?: string }) {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle')

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      setState('copied')
    } catch {
      setState('failed')
    }
    window.setTimeout(() => setState('idle'), 1500)
  }

  return (
    <Button variant="outline" size="xs" onClick={copy} className={className} title="Copy the text">
      {state === 'copied' ? <Check className="text-emerald-600" /> : <Copy />}
      {state === 'failed' ? 'Copy: blocked by the browser' : 'Copy'}
    </Button>
  )
}

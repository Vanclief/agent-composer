// The backend's status word with a colored shape (rtsdesign rule 5: color
// AND shape). The word is printed exactly as sent; a word not listed here
// gets the neutral question mark, never a relabel.
import { Circle, CircleCheck, CircleDot, CircleHelp, Square, Triangle, type LucideIcon } from 'lucide-react'

import { cn } from '@/lib/utils'

type StatusStyle = {
  icon: LucideIcon
  // Classes for the shape.
  shape: string
  // Classes for the pill border and text.
  pill: string
}

const RUNNING: StatusStyle = {
  icon: Circle,
  shape: 'fill-emerald-500 text-emerald-500',
  pill: 'border-emerald-500/30 text-emerald-700 dark:text-emerald-400',
}
const STOPPED: StatusStyle = {
  icon: Square,
  shape: 'text-zinc-400',
  pill: 'border-zinc-400/40 text-zinc-600 dark:text-zinc-400',
}
const TRANSITION: StatusStyle = {
  icon: Circle,
  shape: 'fill-sky-500 text-sky-500 animate-pulse',
  pill: 'border-sky-500/30 text-sky-700 dark:text-sky-400',
}
const FAILED: StatusStyle = {
  icon: Triangle,
  shape: 'fill-red-500 text-red-500',
  pill: 'border-red-500/30 text-red-700 dark:text-red-400',
}
// A task waiting for work: a hollow circle, dim.
const OPEN: StatusStyle = {
  icon: Circle,
  shape: 'text-zinc-500',
  pill: 'border-zinc-400/40 text-zinc-700 dark:text-zinc-300',
}
// A task marked done: a check, settled green.
const DONE: StatusStyle = {
  icon: CircleCheck,
  shape: 'text-emerald-600 dark:text-emerald-500',
  pill: 'border-zinc-400/40 text-zinc-600 dark:text-zinc-400',
}
// A task delivered and waiting for review (ADR 0010): an amber dot.
const REVIEW: StatusStyle = {
  icon: CircleDot,
  shape: 'text-amber-500',
  pill: 'border-amber-500/40 text-amber-700 dark:text-amber-400',
}
const NEUTRAL: StatusStyle = {
  icon: CircleHelp,
  shape: 'text-zinc-500',
  pill: 'border-dashed border-zinc-500/40 text-zinc-600 dark:text-zinc-300',
}

const STYLES: Record<string, StatusStyle> = {
  running: RUNNING,
  stopped: STOPPED,
  starting: TRANSITION,
  stopping: TRANSITION,
  creating: TRANSITION,
  crashed: FAILED,
  error: FAILED,
  failed: FAILED,
  open: OPEN,
  done: DONE,
  review: REVIEW,
}

type StatusPillProps = {
  // The backend status word, verbatim.
  status: string
  // Why the status is what it is (e.g. a task's status_error), as a tooltip.
  detail?: string
  className?: string
}

export function StatusPill({ status, detail, className }: StatusPillProps) {
  const style = STYLES[status] ?? NEUTRAL
  const Icon = style.icon
  return (
    <span
      data-status={status}
      title={detail || undefined}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        style.pill,
        className,
      )}
    >
      <Icon aria-hidden className={cn('size-2.5 shrink-0', style.shape)} strokeWidth={2.5} />
      {status}
    </span>
  )
}

// The backend's status word with a colored shape (rtsdesign rule 5: color
// AND shape), in one of four light tiers (`data-tier`):
// - lit: alive (running: a glowing green dot; starting, stopping, creating:
//   a pulsing blue dot; review: an amber dot);
// - off: ended well (done, stopped: the green light turned off, a thin ring,
//   with a check for done; dim text);
// - attention: died or needs a hand (failed, crashed, error, or any word with
//   a detail): a red triangle, the only red on screen;
// - idle: not started (open, and any word not listed): a hollow grey circle.
// The word is printed exactly as sent, never relabeled.
import { Check, Triangle } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export type StatusTier = 'lit' | 'off' | 'attention' | 'idle'

// The order lists sort in: what needs a hand first, finished things last.
export const TIER_ORDER: StatusTier[] = ['attention', 'lit', 'idle', 'off']

type StatusStyle = {
  tier: StatusTier
  // The shape, a 10px element.
  shape: ReactNode
  // Classes for the pill border and text.
  pill: string
}

const dot = 'block size-2.5 shrink-0 rounded-full'

const RUNNING: StatusStyle = {
  tier: 'lit',
  shape: <span className={cn(dot, 'bg-emerald-500 shadow-[0_0_6px_1px] shadow-emerald-500/60')} />,
  pill: 'border-emerald-500/40 text-emerald-700 dark:text-emerald-400',
}
const TRANSITION: StatusStyle = {
  tier: 'lit',
  shape: <span className={cn(dot, 'animate-pulse bg-sky-500')} />,
  pill: 'border-sky-500/40 text-sky-700 dark:text-sky-400',
}
// A task delivered and waiting for review (ADR 0010).
const REVIEW: StatusStyle = {
  tier: 'lit',
  shape: <span className={cn(dot, 'bg-amber-500')} />,
  pill: 'border-amber-500/40 text-amber-700 dark:text-amber-400',
}
// Ended well: the running dot turned off.
const OFF_PILL = 'border-zinc-400/30 text-emerald-800/60 dark:border-zinc-700 dark:text-emerald-300/50'
const OFF_RING = 'border border-emerald-600/50 dark:border-emerald-400/40'
const DONE: StatusStyle = {
  tier: 'off',
  shape: (
    <span className={cn(dot, OFF_RING, 'grid place-items-center')}>
      <Check className="size-2 text-emerald-700/60 dark:text-emerald-400/60" strokeWidth={3} />
    </span>
  ),
  pill: OFF_PILL,
}
const STOPPED: StatusStyle = {
  tier: 'off',
  shape: <span className={cn(dot, OFF_RING)} />,
  pill: OFF_PILL,
}
const FAILED: StatusStyle = {
  tier: 'attention',
  shape: <Triangle className="size-2.5 shrink-0 fill-red-500 text-red-500" strokeWidth={2.5} />,
  pill: 'border-red-500/60 text-red-700 dark:text-red-400',
}
// Not started, or a word this panel does not know.
const IDLE: StatusStyle = {
  tier: 'idle',
  shape: <span className={cn(dot, 'border-[1.5px] border-zinc-400 dark:border-zinc-500')} />,
  pill: 'border-zinc-400/40 text-zinc-600 dark:text-zinc-300',
}

const STYLES: Record<string, StatusStyle> = {
  running: RUNNING,
  starting: TRANSITION,
  stopping: TRANSITION,
  creating: TRANSITION,
  review: REVIEW,
  done: DONE,
  stopped: STOPPED,
  crashed: FAILED,
  error: FAILED,
  failed: FAILED,
  open: IDLE,
}

function styleOf(status: string, detail?: string) {
  const style = STYLES[status]
  if (style) {
    return style
  }
  return detail ? FAILED : IDLE
}

// The tier a status shows in; lists sort by it (TIER_ORDER).
export function statusTier(status: string, detail?: string): StatusTier {
  return styleOf(status, detail).tier
}

export function tierRank(status: string, detail?: string) {
  return TIER_ORDER.indexOf(statusTier(status, detail))
}

type StatusPillProps = {
  // The backend status word, verbatim.
  status: string
  // Why the status is what it is (e.g. a task's status_error), as a tooltip.
  detail?: string
  className?: string
}

export function StatusPill({ status, detail, className }: StatusPillProps) {
  const style = styleOf(status, detail)
  return (
    <span
      data-status={status}
      data-tier={style.tier}
      title={detail || undefined}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        style.pill,
        className,
      )}
    >
      <span aria-hidden className="grid size-2.5 shrink-0 place-items-center">
        {style.shape}
      </span>
      {status}
    </span>
  )
}

const LEGEND: [StatusTier, StatusStyle][] = [
  ['attention', FAILED],
  ['lit', RUNNING],
  ['idle', IDLE],
  ['off', DONE],
]

// The four tiers as bare shapes; each tier's word only on hover.
export function StatusLegend({ className }: { className?: string }) {
  return (
    <span
      data-testid="status-legend"
      aria-label="Status lights"
      className={cn('inline-flex items-center gap-2', className)}
    >
      {LEGEND.map(([tier, style]) => (
        <span
          key={tier}
          data-tier={tier}
          title={tier}
          aria-label={tier}
          className="grid size-4 place-items-center rounded-full hover:bg-zinc-500/10"
        >
          <span aria-hidden className="grid size-2.5 place-items-center">
            {style.shape}
          </span>
        </span>
      ))}
    </span>
  )
}

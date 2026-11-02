// The backend's status word with an LED, in one of four light tiers
// (`data-tier`). The LEDs are Franco's (2026-11-02), a test of the panel's
// look on the Tasks page:
// - lit: running, green blinking; review, yellow blinking; starting,
//   stopping, creating: blue pulsing;
// - off: done, green on; stopped, green off;
// - attention: crashed (or any word with a detail), red blinking; failed,
//   error: red on;
// - idle: open, and any word not listed: green off.
// The word is printed exactly as sent, never relabeled.
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

// An LED: a 10px disc. Lit ones glow; off ones are the same disc, dark.
const dot = 'block size-2.5 shrink-0 rounded-full'
const BLINK = 'animate-led-blink'
const GREEN_ON = 'bg-emerald-400 shadow-[0_0_6px_2px] shadow-emerald-400/70 ring-1 ring-emerald-200/50 ring-inset'
const GREEN_OFF = 'bg-emerald-950 ring-1 ring-emerald-800/70 ring-inset dark:bg-emerald-950/80'
const YELLOW_ON = 'bg-yellow-400 shadow-[0_0_6px_2px] shadow-yellow-400/70 ring-1 ring-yellow-100/60 ring-inset'
const RED_ON = 'bg-red-500 shadow-[0_0_6px_2px] shadow-red-500/70 ring-1 ring-red-200/50 ring-inset'

// Running: the green LED blinking.
const RUNNING: StatusStyle = {
  tier: 'lit',
  shape: <span className={cn(dot, GREEN_ON, BLINK)} />,
  pill: 'border-emerald-500/40 text-emerald-700 dark:text-emerald-400',
}
const TRANSITION: StatusStyle = {
  tier: 'lit',
  shape: <span className={cn(dot, 'animate-pulse bg-sky-400 shadow-[0_0_6px_2px] shadow-sky-400/60')} />,
  pill: 'border-sky-500/40 text-sky-700 dark:text-sky-400',
}
// Review: the yellow LED blinking, a delivery waiting for a human (ADR 0010).
const REVIEW: StatusStyle = {
  tier: 'lit',
  shape: <span className={cn(dot, YELLOW_ON, BLINK)} />,
  pill: 'border-yellow-500/40 text-yellow-700 dark:text-yellow-400',
}
// Done: the green LED on, steady.
const OFF_PILL = 'border-zinc-400/30 text-emerald-800/60 dark:border-zinc-700 dark:text-emerald-300/50'
const DONE: StatusStyle = {
  tier: 'off',
  shape: <span className={cn(dot, GREEN_ON)} />,
  pill: 'border-emerald-500/30 text-emerald-700/80 dark:text-emerald-400/80',
}
// Stopped: the green LED off.
const STOPPED: StatusStyle = {
  tier: 'off',
  shape: <span className={cn(dot, GREEN_OFF)} />,
  pill: OFF_PILL,
}
// Crashed: the red LED blinking, it needs a hand now.
const CRASHED: StatusStyle = {
  tier: 'attention',
  shape: <span className={cn(dot, RED_ON, BLINK)} />,
  pill: 'border-red-500/60 text-red-700 dark:text-red-400',
}
// Failed: the red LED on, steady; dead and not coming back by itself.
const FAILED: StatusStyle = {
  tier: 'attention',
  shape: <span className={cn(dot, RED_ON)} />,
  pill: 'border-red-500/60 text-red-700 dark:text-red-400',
}
// Open: the green LED off, not started yet.
const OPEN: StatusStyle = {
  tier: 'idle',
  shape: <span className={cn(dot, GREEN_OFF)} />,
  pill: 'border-emerald-500/25 text-emerald-800/70 dark:text-emerald-300/60',
}
// A word this panel does not know: the same dark LED, grey text.
const IDLE: StatusStyle = {
  tier: 'idle',
  shape: <span className={cn(dot, GREEN_OFF)} />,
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
  crashed: CRASHED,
  error: FAILED,
  failed: FAILED,
  open: OPEN,
  // An agent's state: its home sandbox running, stopped, or starting.
  awake: RUNNING,
  asleep: STOPPED,
  waking: TRANSITION,
}

function styleOf(status: string, detail?: string) {
  const style = STYLES[status]
  if (style) {
    return style
  }
  return detail ? CRASHED : IDLE
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
  ['attention', CRASHED],
  ['lit', RUNNING],
  ['idle', OPEN],
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

// A steady LED in the status pill's look, for facts that are not a status:
// green as done, yellow as review, red as failed, never blinking (a
// blinking LED is a live status). The task rail's Review section uses it.
export function Led({ color, className }: { color: 'green' | 'yellow' | 'red'; className?: string }) {
  const on = color === 'green' ? GREEN_ON : color === 'yellow' ? YELLOW_ON : RED_ON
  return <span data-led={color} className={cn(dot, on, className)} />
}

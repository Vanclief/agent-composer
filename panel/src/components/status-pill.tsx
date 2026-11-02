// The backend's status word in one of four light tiers (`data-tier`). The
// word is printed exactly as sent, never relabeled. Franco's spec
// (2026-11-02, the second round): no LED; the word itself is the light.
// Only what needs a hand blinks: review (yellow) and crashed (red). Running
// is filled green, steady. Anything not running is a green outline, not
// filled: done full, open and unknown words dim. Failed is a red outline,
// steady. Starting, stopping and creating are filled blue, steady. The
// filled and blinking looks live in the Tasks list's status tab
// (index.css); the pill alone carries the colors as border and text.
import { cn } from '@/lib/utils'

export type StatusTier = 'lit' | 'off' | 'attention' | 'idle'

// The order lists sort in: what needs a hand first, finished things last.
export const TIER_ORDER: StatusTier[] = ['attention', 'lit', 'idle', 'off']

type StatusStyle = {
  tier: StatusTier
  // Classes for the pill border and text.
  pill: string
}

// Running: green, alive.
const RUNNING: StatusStyle = {
  tier: 'lit',
  pill: 'border-emerald-500/40 text-emerald-700 dark:text-emerald-400',
}
// A machine in transition: blue.
const TRANSITION: StatusStyle = {
  tier: 'lit',
  pill: 'border-sky-500/40 text-sky-700 dark:text-sky-400',
}
// Review: yellow, a delivery waiting for a human (ADR 0010).
const REVIEW: StatusStyle = {
  tier: 'lit',
  pill: 'border-yellow-500/40 text-yellow-700 dark:text-yellow-400',
}
// Done: green, settled.
const DONE: StatusStyle = {
  tier: 'off',
  pill: 'border-emerald-500/30 text-emerald-700/80 dark:text-emerald-400/80',
}
// Stopped: ended, nothing alive.
const STOPPED: StatusStyle = {
  tier: 'off',
  pill: 'border-zinc-400/30 text-emerald-800/60 dark:border-zinc-700 dark:text-emerald-300/50',
}
// Crashed: red, it needs a hand now.
const CRASHED: StatusStyle = {
  tier: 'attention',
  pill: 'border-red-500/60 text-red-700 dark:text-red-400',
}
// Failed: red, dead and not coming back by itself.
const FAILED: StatusStyle = {
  tier: 'attention',
  pill: 'border-red-500/60 text-red-700 dark:text-red-400',
}
// Open: not started yet.
const OPEN: StatusStyle = {
  tier: 'idle',
  pill: 'border-emerald-500/25 text-emerald-800/70 dark:text-emerald-300/60',
}
// A word this panel does not know: grey text.
const IDLE: StatusStyle = {
  tier: 'idle',
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
        'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        style.pill,
        className,
      )}
    >
      {status}
    </span>
  )
}

// A steady light for facts that are not a status: green as done, yellow as
// review, red as failed, never blinking. The task rail's Review section
// uses it for criteria and checks.
const LIGHT = {
  green: 'bg-emerald-400 shadow-[0_0_6px_2px] shadow-emerald-400/70 ring-1 ring-emerald-200/50 ring-inset',
  yellow: 'bg-yellow-400 shadow-[0_0_6px_2px] shadow-yellow-400/70 ring-1 ring-yellow-100/60 ring-inset',
  red: 'bg-red-500 shadow-[0_0_6px_2px] shadow-red-500/70 ring-1 ring-red-200/50 ring-inset',
}

export function Led({ color, className }: { color: 'green' | 'yellow' | 'red'; className?: string }) {
  return <span data-led={color} className={cn('block size-2.5 shrink-0 rounded-full', LIGHT[color], className)} />
}

// How a belief, decision, rule or action is grounded, as color AND shape
// (rtsdesign 6): grounded is a filled green circle, opinion a hollow amber
// circle, in doubt a filled red triangle, contested two overlapping rose
// circles, one filled and one hollow. The word next to the shape is the
// server's label, verbatim; an unknown word gets a neutral question mark.
import type { ComponentType, SVGProps } from 'react'
import { Circle, CircleHelp, Triangle } from 'lucide-react'

import { cn } from '@/lib/utils'

type Shape = ComponentType<{ className?: string; strokeWidth?: number; 'aria-hidden'?: boolean }>
type MarkStyle = { icon: Shape; shape: string; text: string; tip: string }

// Two sides that disagree: a filled circle overlapped by a hollow one.
export function TwoCircles({ strokeWidth = 2, ...props }: SVGProps<SVGSVGElement> & { strokeWidth?: number }) {
  return (
    <svg viewBox="0 0 32 24" fill="none" stroke="currentColor" strokeWidth={strokeWidth} {...props}>
      <circle cx="10" cy="12" r="7.5" fill="currentColor" />
      <circle cx="22" cy="12" r="7.5" />
    </svg>
  )
}

const GROUNDED: MarkStyle = {
  icon: Circle,
  shape: 'fill-emerald-500 text-emerald-500',
  text: 'text-emerald-700 dark:text-emerald-400',
  tip: 'Every leaf cites live evidence',
}
const OPINION: MarkStyle = {
  icon: Circle,
  shape: 'text-amber-500',
  text: 'text-amber-700 dark:text-amber-400',
  tip: 'A leaf cites no evidence',
}
const IN_DOUBT: MarkStyle = {
  icon: Triangle,
  shape: 'fill-red-500 text-red-500',
  text: 'text-red-700 dark:text-red-400',
  tip: 'A leaf cites only retracted evidence',
}
const CONTESTED: MarkStyle = {
  icon: TwoCircles,
  shape: 'w-4 text-rose-500',
  text: 'text-rose-700 dark:text-rose-400',
  tip: 'Something grounded contradicts it',
}
const UNKNOWN: MarkStyle = {
  icon: CircleHelp,
  shape: 'text-zinc-500',
  text: 'text-zinc-600 dark:text-zinc-400',
  tip: '',
}

// knowledge/graph.go spells it "in doubt"; the API contract "in_doubt".
const STYLES: Record<string, MarkStyle> = {
  grounded: GROUNDED,
  opinion: OPINION,
  in_doubt: IN_DOUBT,
  'in doubt': IN_DOUBT,
  contested: CONTESTED,
}

type LabelMarkProps = {
  // The server's label word, verbatim.
  label: string
  className?: string
}

export function LabelMark({ label, className }: LabelMarkProps) {
  const style = STYLES[label] ?? UNKNOWN
  const Icon = style.icon
  return (
    <span
      data-label={label}
      title={style.tip || undefined}
      className={cn('inline-flex items-center gap-1.5 text-xs font-medium whitespace-nowrap', style.text, className)}
    >
      <Icon aria-hidden className={cn('size-3 shrink-0', style.shape)} strokeWidth={2.5} />
      {label}
    </span>
  )
}

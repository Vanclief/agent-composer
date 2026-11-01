// How a belief, decision or rule is grounded, as color AND shape
// (rtsdesign 6): grounded is a filled green circle, opinion a hollow amber
// circle, in doubt a filled red triangle. The word next to the shape is the
// server's label, verbatim; an unknown word gets a neutral question mark.
import { Circle, CircleHelp, Triangle, type LucideIcon } from 'lucide-react'

import { cn } from '@/lib/utils'

type MarkStyle = { icon: LucideIcon; shape: string; text: string; tip: string }

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

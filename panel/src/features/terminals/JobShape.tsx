// A job's state as a shape (rtsdesign rule 5): green filled dot running,
// grey hollow square exited. The word is the tooltip.
import { Circle, Square } from 'lucide-react'

import { cn } from '@/lib/utils'

export function JobShape({ running, className }: { running: boolean; className?: string }) {
  const word = running ? 'running' : 'exited'
  const Icon = running ? Circle : Square
  return (
    <span title={word} data-state={word} className={cn('inline-flex shrink-0', className)}>
      <Icon
        aria-hidden
        strokeWidth={2.5}
        className={cn('size-2.5', running ? 'fill-emerald-500 text-emerald-500' : 'text-zinc-400')}
      />
    </span>
  )
}

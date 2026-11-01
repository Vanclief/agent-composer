// An icon and a short mono value: `⛁ 4 GB`, `# review-42`, a path or an id
// (rtsdesign rules 1 and 12). The icon should come from concept-icons.ts.
import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

import { conceptColors, type Concept } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

type IconChipProps = {
  icon: LucideIcon
  children: ReactNode
  // Tooltip explaining what the chip is, e.g. "Memory".
  title?: string
  // Tints the chip in the concept's color (concept-icons.ts); without it
  // the chip stays neutral zinc.
  concept?: Concept
  className?: string
}

export function IconChip({ icon: Icon, children, title, concept, className }: IconChipProps) {
  const color = concept ? conceptColors[concept] : undefined
  return (
    <span
      title={title}
      className={cn(
        'inline-flex max-w-full items-center gap-1 rounded-md border px-1.5 py-0.5 font-mono text-xs',
        color
          ? [color.bg, color.border, color.text]
          : 'border-zinc-200 text-zinc-700 dark:border-zinc-800 dark:text-zinc-300',
        className,
      )}
    >
      <Icon aria-hidden className={cn('size-3.5 shrink-0', color ? color.text : 'text-zinc-500')} />
      <span className="truncate">{children}</span>
    </span>
  )
}

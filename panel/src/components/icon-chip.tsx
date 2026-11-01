// An icon and a short mono value: `⛁ 4 GB`, `# review-42`, a path or an id
// (rtsdesign rules 1 and 12). The icon should come from concept-icons.ts.
import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

import { cn } from '@/lib/utils'

type IconChipProps = {
  icon: LucideIcon
  children: ReactNode
  // Tooltip explaining what the chip is, e.g. "Memory".
  title?: string
  className?: string
}

export function IconChip({ icon: Icon, children, title, className }: IconChipProps) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex max-w-full items-center gap-1 rounded-md border border-zinc-200 px-1.5 py-0.5 font-mono text-xs text-zinc-700 dark:border-zinc-800 dark:text-zinc-300',
        className,
      )}
    >
      <Icon aria-hidden className="size-3.5 shrink-0 text-zinc-500" />
      <span className="truncate">{children}</span>
    </span>
  )
}

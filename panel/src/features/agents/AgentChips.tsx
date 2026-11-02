// Small pieces shared by the agents list and an agent's page.
import { BadgeCheck } from 'lucide-react'

import { cn } from '@/lib/utils'

// The role an agent was made from, as a plain chip.
export function RoleChip({ role, className }: { role: string; className?: string }) {
  return (
    <span
      data-testid="agent-role"
      title="Role: the template it was made from"
      className={cn(
        'inline-flex shrink-0 items-center rounded-md border border-zinc-200 px-1.5 py-0.5 text-xs text-zinc-700 dark:border-zinc-800 dark:text-zinc-300',
        className,
      )}
    >
      {role}
    </span>
  )
}

// "kept" once Franco named it; numbered agents say so instead.
export function KeptBadge({ kept }: { kept: boolean }) {
  if (kept) {
    return (
      <span
        data-testid="agent-kept"
        title="Kept: named, worth keeping"
        className="inline-flex items-center gap-1 rounded-full border border-pink-300 px-2 py-0.5 text-xs font-medium text-pink-700 dark:border-pink-800 dark:text-pink-300"
      >
        <BadgeCheck aria-hidden className="size-3" />
        kept
      </span>
    )
  }
  return (
    <span
      data-testid="agent-numbered"
      title="Numbered from its role; keep it to give it a name"
      className="text-xs text-muted-foreground italic"
    >
      numbered
    </span>
  )
}

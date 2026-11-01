// An empty slot (rtsdesign rule 3): a dashed tile with a + that fills it,
// or, when it cannot be filled, a short reason printed in place. Never put
// the reason only in a tooltip.
import { Plus } from 'lucide-react'

import { cn } from '@/lib/utils'

type EmptySlotProps = {
  // Short in-place reason the slot cannot be filled, e.g. "Task stopped".
  // When set, the slot is inert and shows this text instead of +.
  reason?: string
  // Fills the slot. Without it (and without reason) the + is decorative.
  onAdd?: () => void
  // Accessible name and tooltip for the + button, e.g. "Add secret".
  label?: string
  className?: string
}

const base =
  'flex min-h-9 min-w-9 items-center justify-center rounded-md border border-dashed border-zinc-300 text-zinc-500 dark:border-zinc-700'

export function EmptySlot({ reason, onAdd, label, className }: EmptySlotProps) {
  if (reason) {
    return <div className={cn(base, 'px-3 py-2 text-xs', className)}>{reason}</div>
  }
  if (onAdd) {
    return (
      <button
        type="button"
        onClick={onAdd}
        aria-label={label}
        title={label}
        className={cn(
          base,
          'transition-colors hover:border-zinc-400 hover:text-zinc-900 dark:hover:border-zinc-500 dark:hover:text-zinc-100',
          className,
        )}
      >
        <Plus className="size-4" />
      </button>
    )
  }
  return (
    <div className={cn(base, className)} title={label}>
      <Plus className="size-4" />
    </div>
  )
}

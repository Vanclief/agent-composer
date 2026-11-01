// Small building blocks of the create dialog: cards, tooltips, inputs with
// a leading icon and the lit/outlined chip look.
import type { ComponentProps, ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

// A toggle chip (Toggle or ToggleGroupItem): outlined when off, filled
// when on (rtsdesign rule 6).
export const chipClass =
  'h-7 min-w-9 rounded-md border border-border bg-transparent px-2 text-xs font-normal tabular-nums text-muted-foreground shadow-none hover:bg-accent hover:text-accent-foreground data-[state=on]:border-primary data-[state=on]:bg-primary data-[state=on]:text-primary-foreground data-[state=on]:hover:bg-primary/90 data-[state=on]:hover:text-primary-foreground'

// The same look for a plain element that is always lit.
export const litClass = 'border-primary bg-primary text-primary-foreground'

// A tooltip on any element; explanations live here, never in the layout.
// wrap puts the element in a span first: a Toggle needs it, because the
// trigger's own data-state would replace the toggle's on/off one.
export function Tip({ tip, wrap, children }: { tip: ReactNode; wrap?: boolean; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {wrap ? <span className="inline-flex shrink-0">{children}</span> : children}
      </TooltipTrigger>
      <TooltipContent>{tip}</TooltipContent>
    </Tooltip>
  )
}

// A bordered card with one icon in its corner and a one-word label.
export function Card({
  name,
  icon: Icon,
  label,
  tip,
  count,
  children,
}: {
  name: string
  icon: LucideIcon
  label: string
  tip: string
  // Badge after the label; hidden when 0.
  count?: number
  children: ReactNode
}) {
  return (
    <section data-card={name} className="flex flex-col gap-2.5 rounded-md border border-border p-3">
      <div className="flex items-center gap-1.5 text-xs font-medium tracking-wide text-muted-foreground uppercase">
        <Tip tip={tip}>
          <Icon aria-label={tip} className="size-3.5" />
        </Tip>
        {label}
        {count ? <CountBadge count={count} /> : null}
      </div>
      {children}
    </section>
  )
}

export function CountBadge({ count, className }: { count: number; className?: string }) {
  return (
    <span
      data-count
      className={cn(
        'rounded bg-muted px-1.5 font-mono tracking-normal text-foreground/80 normal-case tabular-nums',
        className,
      )}
    >
      {count}
    </span>
  )
}

// A compact mono input with a leading icon; tip explains the field.
export function IconInput({
  icon: Icon,
  tip,
  className,
  wrapperClassName,
  ...props
}: ComponentProps<'input'> & { icon: LucideIcon; tip: string; wrapperClassName?: string }) {
  return (
    <div className={cn('relative min-w-0', wrapperClassName)}>
      <Tip tip={tip}>
        <Icon aria-hidden className="absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
      </Tip>
      <Input
        aria-label={tip}
        className={cn('h-7 pr-2 pl-7 font-mono text-xs shadow-none md:text-xs', className)}
        {...props}
      />
    </div>
  )
}

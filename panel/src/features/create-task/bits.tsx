// Small building blocks of the create dialog: cards, tooltips, inputs with
// a leading icon and the lit/outlined chip look.
import type { ComponentProps, ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

// A toggle chip (Toggle or ToggleGroupItem): outlined and dim when off.
// Add a concept's `on` classes (concept-icons.ts) to light it in that
// concept's color when on (rtsdesign rules 3 and 6).
export const chipBase =
  'h-7 min-w-9 rounded-md border border-border bg-transparent px-2 text-xs font-normal tabular-nums text-muted-foreground shadow-none hover:bg-accent hover:text-accent-foreground'

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

// A bordered card with its concept's icon in the corner, a one-word label
// and a top edge in the concept's color, so cards are told apart at a glance.
export function Card({
  name,
  concept,
  label,
  tip,
  count,
  aside,
  className,
  children,
}: {
  name: string
  concept: Concept
  label: string
  tip: string
  // Badge after the label; hidden when 0.
  count?: number
  // Controls at the right end of the header row.
  aside?: ReactNode
  className?: string
  children?: ReactNode
}) {
  const Icon = conceptIcons[concept]
  const color = conceptColors[concept]
  return (
    <section
      data-card={name}
      className={cn(
        'flex flex-col gap-3 rounded-xl border border-border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
        color.edge,
        className,
      )}
    >
      <div className={cn('flex items-center gap-1.5 text-xs font-medium tracking-wide uppercase', color.text)}>
        <Tip tip={tip}>
          <Icon aria-label={tip} className="size-3.5" />
        </Tip>
        {label}
        {count ? <CountBadge count={count} className={cn(color.bg, color.text)} /> : null}
        {aside ? (
          <div className="ml-auto flex items-center gap-2 font-normal tracking-normal text-foreground normal-case">
            {aside}
          </div>
        ) : null}
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
// concept colors the icon.
export function IconInput({
  icon: Icon,
  concept,
  tip,
  className,
  wrapperClassName,
  ...props
}: ComponentProps<'input'> & { icon: LucideIcon; concept?: Concept; tip: string; wrapperClassName?: string }) {
  return (
    <div className={cn('relative min-w-0', wrapperClassName)}>
      <Tip tip={tip}>
        <Icon
          aria-hidden
          className={cn(
            'absolute top-1/2 left-2 size-3.5 -translate-y-1/2',
            concept ? conceptColors[concept].text : 'text-muted-foreground',
          )}
        />
      </Tip>
      <Input
        aria-label={tip}
        className={cn('h-7 pr-2 pl-7 font-mono text-xs shadow-none md:text-xs', className)}
        {...props}
      />
    </div>
  )
}

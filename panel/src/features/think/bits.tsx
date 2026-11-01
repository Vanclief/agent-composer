// Small pieces shared by the Think pages and the detail pane.
import type { ReactNode } from 'react'
import { Circle, Triangle, Undo2, type LucideIcon } from 'lucide-react'
import { Link } from 'react-router'

import type { Belief, Decision, Knowledge, Retraction, Rule } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

import { LabelMark } from './LabelMark'

export type Kind = 'beliefs' | 'decisions' | 'rules' | 'evidence'

export const KINDS: Record<Kind, { title: string; one: string; icon: LucideIcon }> = {
  beliefs: { title: 'Beliefs', one: 'belief', icon: conceptIcons.belief },
  decisions: { title: 'Decisions', one: 'decision', icon: conceptIcons.decision },
  rules: { title: 'Rules', one: 'rule', icon: conceptIcons.rule },
  evidence: { title: 'Evidence', one: 'evidence', icon: conceptIcons.evidence },
}

export function itemPath(kind: Kind, id?: string) {
  return id ? `/think/${kind}/${encodeURIComponent(id)}` : `/think/${kind}`
}

// Where each id lives and its label, to link and mark the nodes of a why
// tree, which carry neither.
export type Found = { kind: Kind; label?: string }

export function indexKnowledge(data: Knowledge) {
  const found = new Map<string, Found>()
  for (const e of data.evidence) {
    found.set(e.id, { kind: 'evidence' })
  }
  for (const b of data.beliefs) {
    found.set(b.id, { kind: 'beliefs', label: b.label })
  }
  for (const d of data.decisions) {
    found.set(d.id, { kind: 'decisions', label: d.label })
  }
  for (const r of data.rules) {
    found.set(r.id, { kind: 'rules', label: r.label })
  }
  return found
}

// Warning chips for what an item rests on: opinions in amber, doubted
// beliefs in red, each with the count and the ids in the tooltip. An
// opinion or doubted belief does not count itself.
export function RestsOn({ item }: { item: Belief | Decision | Rule }) {
  const opinions = (item.opinions ?? []).filter((id) => id !== item.id)
  const doubted = (item.doubted ?? []).filter((id) => id !== item.id)
  return (
    <>
      {opinions.length > 0 && (
        <span
          data-testid="rests-on-opinion"
          title={opinions.join('\n')}
          className="inline-flex items-center gap-1 rounded-md border border-amber-500/40 px-1.5 py-0.5 font-mono text-xs text-amber-700 dark:text-amber-400"
        >
          <Circle aria-hidden className="size-3 text-amber-500" strokeWidth={2.5} />
          rests on opinion: {opinions.length}
        </span>
      )}
      {doubted.length > 0 && (
        <span
          data-testid="rests-on-doubt"
          title={doubted.join('\n')}
          className="inline-flex items-center gap-1 rounded-md border border-red-500/40 px-1.5 py-0.5 font-mono text-xs text-red-700 dark:text-red-400"
        >
          <Triangle aria-hidden className="size-3 fill-red-500 text-red-500" strokeWidth={2.5} />
          rests on doubt: {doubted.length}
        </span>
      )}
    </>
  )
}

// One line in the detail pane: the mark and the text, linking to the item.
export function ItemRow({ kind, id, label, text }: { kind: Kind; id: string; label?: string; text: ReactNode }) {
  return (
    <Link
      to={itemPath(kind, id)}
      title={id}
      className="flex items-start gap-2 rounded px-1 py-0.5 text-sm transition-colors hover:bg-zinc-100 dark:hover:bg-zinc-800/60"
    >
      <span className="w-[4.75rem] shrink-0 pt-0.5">{label && <LabelMark label={label} />}</span>
      <span className="min-w-0">{text}</span>
    </Link>
  )
}

// A one-word section heading with its icon and an optional count.
export function Section({
  title,
  icon: Icon,
  count,
  children,
  className,
  sub,
}: {
  title: string
  icon: LucideIcon
  count?: number
  children: ReactNode
  className?: string
  // A group inside a section: smaller and dimmer heading.
  sub?: boolean
}) {
  return (
    <section aria-label={title} className={cn('flex flex-col gap-1.5', className)}>
      <h3
        className={cn(
          'flex items-center gap-1.5 font-medium',
          sub ? 'text-xs text-zinc-500 dark:text-zinc-400' : 'text-sm text-zinc-900 dark:text-zinc-100',
        )}
      >
        <Icon aria-hidden className="size-3.5" />
        {title}
        {count !== undefined && <span className="font-mono tabular-nums">{count}</span>}
      </h3>
      {children}
    </section>
  )
}

// Retracted evidence: a red chip with who, when and why in the tooltip.
export function RetractedChip({ retracted }: { retracted: Retraction }) {
  return (
    <span
      data-testid="retracted"
      title={`${retracted.who} · ${retracted.when}: ${retracted.why}`}
      className="inline-flex items-center gap-1 rounded-md border border-red-500/40 px-1.5 py-0.5 font-mono text-xs text-red-700 dark:text-red-400"
    >
      <Undo2 aria-hidden className="size-3" />
      retracted
    </span>
  )
}

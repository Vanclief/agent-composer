// Small pieces shared by the Think pages and the detail pane.
import type { KeyboardEvent, MouseEvent, ReactNode } from 'react'
import { CalendarCheck, CalendarPlus, Check, Circle, Triangle, Undo2, type LucideIcon } from 'lucide-react'
import { Link, useNavigate } from 'react-router'

import type { Action, Belief, Decision, Knowledge, Retraction, Rule, WhyKind } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
import { cn } from '@/lib/utils'

import { LabelMark } from './LabelMark'

export type Kind = 'beliefs' | 'decisions' | 'rules' | 'actions' | 'evidence'

// edge is the item's concept color as a left border, for the or groups
// of a why tree.
export const KINDS: Record<Kind, { title: string; one: string; icon: LucideIcon; edge: string }> = {
  beliefs: {
    title: 'Beliefs',
    one: 'belief',
    icon: conceptIcons.belief,
    edge: 'border-l-indigo-400 dark:border-l-indigo-500',
  },
  decisions: {
    title: 'Decisions',
    one: 'decision',
    icon: conceptIcons.decision,
    edge: 'border-l-fuchsia-400 dark:border-l-fuchsia-500',
  },
  rules: { title: 'Rules', one: 'rule', icon: conceptIcons.rule, edge: 'border-l-orange-400 dark:border-l-orange-500' },
  actions: {
    title: 'Actions',
    one: 'action',
    icon: conceptIcons.action,
    edge: 'border-l-lime-400 dark:border-l-lime-500',
  },
  evidence: {
    title: 'Evidence',
    one: 'evidence',
    icon: conceptIcons.evidence,
    edge: 'border-l-teal-400 dark:border-l-teal-500',
  },
}

// The page kind of a why node's kind.
export const WHY_KINDS: Record<WhyKind, Kind> = {
  belief: 'beliefs',
  decision: 'decisions',
  rule: 'rules',
  action: 'actions',
  evidence: 'evidence',
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
  for (const a of data.actions ?? []) {
    found.set(a.id, { kind: 'actions', label: a.label })
  }
  return found
}

// Warning chips for what an item rests on: opinions in amber, doubted
// beliefs in red, each with the count and the ids in the tooltip. An
// opinion or doubted belief does not count itself.
export function RestsOn({ item }: { item: Belief | Decision | Rule | Action }) {
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

// Who holds a belief when it is not the repo owner (by), as a person chip.
export function ByChip({ belief }: { belief: Belief }) {
  if (!belief.by) {
    return null
  }
  return (
    <span data-testid="by" className="contents">
      <IconChip icon={conceptIcons.human} title={belief.when ? `Held by, on ${belief.when}` : 'Held by'}>
        {belief.by}
      </IconChip>
    </span>
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

// "contested by <id>" chips, rose like the contested mark, each opening the
// item on the other side. Inside a card (itself a link) they navigate on
// click instead of nesting a link in a link.
export function ContestedBy({ ids, found, nested }: { ids: string[]; found: Map<string, Found>; nested?: boolean }) {
  const navigate = useNavigate()
  const style =
    'relative inline-flex items-center gap-1 rounded-md border border-rose-500/40 px-1.5 py-0.5 font-mono text-xs text-rose-700 transition-colors hover:bg-rose-500/10 dark:text-rose-400'
  return (
    <>
      {ids.map((id) => {
        const kind = found.get(id)?.kind ?? 'beliefs'
        const Icon = KINDS[kind].icon
        const to = itemPath(kind, id)
        const body = (
          <>
            <Icon aria-hidden className="size-3" />
            contested by {id}
          </>
        )
        if (!nested) {
          return (
            <Link key={id} to={to} data-testid="contested-by" title={`Open ${id}`} className={style}>
              {body}
            </Link>
          )
        }
        const go = (e: MouseEvent | KeyboardEvent) => {
          e.preventDefault()
          e.stopPropagation()
          navigate(to)
        }
        return (
          <span
            key={id}
            role="link"
            tabIndex={0}
            data-testid="contested-by"
            title={`Open ${id}`}
            className={style}
            onClick={go}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                go(e)
              }
            }}
          >
            {body}
          </span>
        )
      })}
    </>
  )
}

// Whether an action is done, as a checkbox shape in neutral colors: a
// hollow square while open, a filled square with a check once done. It is
// not a truth label, so it takes none of the label colors.
export function DoneMark({ done, className }: { done: boolean; className?: string }) {
  return (
    <span
      data-testid="done-mark"
      data-done={done}
      title={done ? 'Done' : 'To do'}
      className={cn(
        'inline-grid size-3.5 shrink-0 place-items-center rounded-[3px] border-2',
        done
          ? 'border-zinc-500 bg-zinc-500 text-white dark:border-zinc-400 dark:bg-zinc-400 dark:text-zinc-900'
          : 'border-zinc-400 dark:border-zinc-500',
        className,
      )}
    >
      {done && <Check aria-hidden className="size-2.5" strokeWidth={4} />}
    </span>
  )
}

// The mark of an action: its checkbox and the word, neutral, since open or
// done is not a truth label.
export function ActionMark({ done }: { done: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium whitespace-nowrap text-zinc-600 dark:text-zinc-400">
      <DoneMark done={done} />
      {done ? 'done' : 'to do'}
    </span>
  )
}

// The decisions and beliefs an action serves.
export function ServesChips({ action, found }: { action: Action; found: Map<string, Found> }) {
  const ids = action.because
  return (
    <>
      {ids.map((id) => {
        const kind = found.get(id)?.kind ?? 'decisions'
        return (
          <span key={id} data-testid="serves" className="contents">
            <IconChip icon={KINDS[kind].icon} title={kind === 'decisions' ? 'Serves the decision' : 'Rests on'}>
              {id}
            </IconChip>
          </span>
        )
      })}
    </>
  )
}

export function ObservedChips({ action }: { action: Action }) {
  const { opened, done } = action
  return (
    <>
      <IconChip icon={CalendarPlus} title={`Opened by ${opened.who} on ${opened.when}`}>
        {opened.who} · {opened.when}
      </IconChip>
      {done && (
        <IconChip icon={CalendarCheck} title={`Done by ${done.who} on ${done.when}`}>
          {done.who} · {done.when}
        </IconChip>
      )}
    </>
  )
}

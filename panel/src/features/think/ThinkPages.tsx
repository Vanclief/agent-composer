// /think/beliefs, /think/decisions, /think/rules, /think/actions and
// /think/evidence: one card per item in a fixed single column, and the
// detail pane on the right for /:id. Read-only: the files in git are the
// source of truth. ?without=<id> judges everything as if that belief or
// evidence were gone (what-if); nothing is written.
import { useMemo, type ReactNode } from 'react'
import { FlaskConical } from 'lucide-react'
import { Link, useParams, useSearchParams } from 'react-router'

import { useKnowledgeView } from '@/api/hooks'
import type { Action, Belief, Decision, Evidence, Knowledge, Rule } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { Conflicts } from './Conflicts'
import { DetailPane } from './DetailPane'
import { LabelMark } from './LabelMark'
import {
  ActionMark,
  ByChip,
  ContestedBy,
  DoneMark,
  indexKnowledge,
  ObservedChips,
  itemPath,
  KINDS,
  RestsOn,
  RetractedChip,
  ServesChips,
  type Found,
  type Kind,
} from './bits'
import { actionSentence, beliefSentence, decisionSentence, evidenceSentence, properWords } from './sentence'

export function BeliefsPage() {
  return <ThinkPage kind="beliefs" />
}

export function DecisionsPage() {
  return <ThinkPage kind="decisions" />
}

export function RulesPage() {
  return <ThinkPage kind="rules" />
}

export function ActionsPage() {
  return <ThinkPage kind="actions" />
}

export function EvidencePage() {
  return <ThinkPage kind="evidence" />
}

function itemsOf(data: Knowledge, kind: Kind) {
  return kind === 'actions' ? (data.actions ?? []) : data[kind]
}

// What the cards need to print sentences and link ids, from one answer.
type View = { data: Knowledge; proper: Set<string>; found: Map<string, Found>; without: string[] }

function ThinkPage({ kind }: { kind: Kind }) {
  const { id } = useParams()
  const [params] = useSearchParams()
  const asked = params.get('without') || undefined
  const query = useKnowledgeView(asked)
  const data = query.data
  const view = useMemo<View | undefined>(
    () => data && { data, proper: properWords(data), found: indexKnowledge(data), without: data.without ?? [] },
    [data],
  )
  const without = view?.without ?? []
  const { title, icon: Icon } = KINDS[kind]
  usePageTitle(`${title}${without.length > 0 ? ` · without ${without.join(', ')}` : ''}`)

  return (
    <div className="flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-y-auto p-6">
        <div className="flex max-w-2xl flex-col gap-3">
          {without.length > 0 && (
            <div
              data-testid="what-if-banner"
              role="status"
              className="flex items-center gap-1.5 rounded-md border border-amber-500/40 bg-amber-500/10 px-2 py-1 text-xs text-amber-800 dark:text-amber-300"
            >
              <FlaskConical aria-hidden className="size-3.5 shrink-0" />
              Showing labels without {without.join(', ')}
            </div>
          )}
          <div data-testid="think-strip" className="flex flex-wrap items-center gap-x-6 gap-y-2">
            <span
              title={title}
              className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
            >
              <Icon className="size-4" />
              {data ? itemsOf(data, kind).length : '–'}
            </span>
            {data && <LabelCounts kind={kind} data={data} />}
          </div>
          <ErrorText error={query.error} />
          {query.isPending && (
            <div className="flex flex-col gap-2" aria-label={`Loading ${kind}`}>
              <Skeleton className="h-16" />
              <Skeleton className="h-16" />
              <Skeleton className="h-16" />
            </div>
          )}
          {kind === 'beliefs' && view && view.data.beliefs.length > 0 && (
            <Conflicts data={view.data} proper={view.proper} />
          )}
          {view && <List kind={kind} view={view} selected={id} />}
        </div>
      </div>
      {id && data && <DetailPane key={`${kind}/${id}`} kind={kind} id={id} data={data} without={asked} />}
    </div>
  )
}

// The header strip's counts: how many items carry each label, the
// server's words verbatim, in the order they first appear; evidence has no
// label, so it counts the retracted pieces.
function LabelCounts({ kind, data }: { kind: Kind; data: Knowledge }) {
  const counts = new Map<string, number>()
  if (kind === 'evidence') {
    const retracted = data.evidence.filter((e) => e.retracted).length
    if (retracted > 0) {
      counts.set('retracted', retracted)
    }
  } else {
    for (const item of itemsOf(data, kind) as { label: string }[]) {
      counts.set(item.label, (counts.get(item.label) ?? 0) + 1)
    }
  }
  if (counts.size === 0) {
    return null
  }
  return (
    <span data-testid="label-counts" className="inline-flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
      {[...counts].map(([label, n]) => (
        <span key={label} data-label-count={label} className="inline-flex items-center gap-1.5">
          {kind === 'evidence' ? <span className="font-mono">{label}</span> : <LabelMark label={label} />}
          <b className="font-mono tabular-nums">{String(n).padStart(2, '0')}</b>
        </span>
      ))}
    </span>
  )
}

function List({ kind, view, selected }: { kind: Kind; view: View; selected?: string }) {
  const { data } = view
  if (itemsOf(data, kind).length === 0) {
    const all =
      data.evidence.length +
      data.beliefs.length +
      data.decisions.length +
      data.rules.length +
      (data.actions ?? []).length
    const reason = all === 0 ? `No knowledge files in ${data.dir}` : `No ${kind} in ${data.dir}`
    return <EmptySlot reason={reason} className="h-24" />
  }
  if (kind === 'beliefs') {
    return (
      <Cards>
        {data.beliefs.map((b) => (
          <BeliefCard key={b.id} belief={b} view={view} selected={b.id === selected} />
        ))}
      </Cards>
    )
  }
  if (kind === 'decisions') {
    return (
      <Cards>
        {data.decisions.map((d) => (
          <DecisionCard key={d.id} decision={d} view={view} selected={d.id === selected} />
        ))}
      </Cards>
    )
  }
  if (kind === 'rules') {
    return <RuleGroups rules={data.rules} selected={selected} />
  }
  if (kind === 'actions') {
    return <ActionGroups actions={data.actions ?? []} view={view} selected={selected} />
  }
  return (
    <Cards>
      {data.evidence.map((e) => (
        <EvidenceCard key={e.id} evidence={e} view={view} selected={e.id === selected} />
      ))}
    </Cards>
  )
}

function Cards({ children }: { children: ReactNode }) {
  return <ul className="flex flex-col gap-2">{children}</ul>
}

type CardProps = {
  to: string
  id: string
  selected: boolean
  // The left slot: a LabelMark, the evidence icon or an action's mark.
  mark: ReactNode
  text: string
  // The raw text from the file, in a tooltip over the sentence.
  raw?: string
  struck?: boolean
  dim?: boolean
  // Mono chips under the text.
  trailer: ReactNode
  // Pinned to the right of the text, e.g. a count badge.
  badge?: ReactNode
}

// One item: the mark, the sentence as the primary line, and a trailer of
// chips. The whole card opens the detail pane.
function Card({ to, id, selected, mark, text, raw, struck, dim, trailer, badge }: CardProps) {
  return (
    <li data-testid="think-card" data-id={id}>
      <Link
        to={to}
        aria-current={selected ? 'true' : undefined}
        className={cn(
          'flex flex-col gap-2 rounded-md border px-3 py-2.5 transition-colors',
          selected
            ? 'border-zinc-400 bg-zinc-100 dark:border-zinc-600 dark:bg-zinc-800/80'
            : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-900/60',
        )}
      >
        <div className="flex items-start gap-3">
          <span className="w-[4.75rem] shrink-0 pt-0.5">{mark}</span>
          <p
            data-testid="card-text"
            title={raw}
            className={cn(
              'min-w-0 flex-1 text-sm',
              dim && 'text-zinc-500 dark:text-zinc-400',
              struck && 'text-zinc-500 line-through dark:text-zinc-500',
            )}
          >
            {text}
          </p>
          {badge}
        </div>
        <div className="flex flex-wrap gap-1.5 pl-[5.5rem]">{trailer}</div>
      </Link>
    </li>
  )
}

function IdChips({ icon, id, file }: { icon: typeof conceptIcons.belief; id: string; file: string }) {
  return (
    <>
      <IconChip icon={icon} title="Id">
        {id}
      </IconChip>
      <IconChip icon={conceptIcons.file} title="File">
        {file}
      </IconChip>
    </>
  )
}

function BeliefCard({ belief, view, selected }: { belief: Belief; view: View; selected: boolean }) {
  return (
    <Card
      to={itemPath('beliefs', belief.id)}
      id={belief.id}
      selected={selected}
      mark={<LabelMark label={belief.label} />}
      text={beliefSentence(belief, view.proper, view.without)}
      raw={belief.text}
      trailer={
        <>
          <IdChips icon={conceptIcons.belief} id={belief.id} file={belief.file} />
          <ByChip belief={belief} />
          <RestsOn item={belief} />
          <ContestedBy ids={belief.contested_by ?? []} found={view.found} nested />
        </>
      }
    />
  )
}

function DecisionCard({ decision, view, selected }: { decision: Decision; view: View; selected: boolean }) {
  return (
    <Card
      to={itemPath('decisions', decision.id)}
      id={decision.id}
      selected={selected}
      mark={<LabelMark label={decision.label} />}
      text={decisionSentence(decision.text, view.proper)}
      raw={decision.text}
      trailer={
        <>
          <IdChips icon={conceptIcons.decision} id={decision.id} file={decision.file} />
          <RestsOn item={decision} />
        </>
      }
    />
  )
}

// Rules in one panel per subject, in file order.
function RuleGroups({ rules, selected }: { rules: Rule[]; selected?: string }) {
  const subjects = [...new Set(rules.map((r) => r.subject))]
  const Subject = conceptIcons.subject
  return (
    <div className="flex flex-col gap-4">
      {subjects.map((subject) => {
        const group = rules.filter((r) => r.subject === subject)
        return (
          <section
            key={subject}
            aria-label={subject}
            data-testid="rule-group"
            className="flex flex-col gap-2 rounded-lg border border-zinc-200 p-2 dark:border-zinc-800"
          >
            <h2 className="flex items-center gap-1.5 px-1 text-sm font-medium">
              <Subject aria-hidden className="size-4 text-zinc-500" />
              {subject}
              <span className="font-mono text-xs text-zinc-500 tabular-nums">{group.length}</span>
            </h2>
            <Cards>
              {group.map((r) => (
                <RuleCard key={r.id} rule={r} selected={r.id === selected} />
              ))}
            </Cards>
          </section>
        )
      })}
    </div>
  )
}

function RuleCard({ rule, selected }: { rule: Rule; selected: boolean }) {
  return (
    <Card
      to={itemPath('rules', rule.id)}
      id={rule.id}
      selected={selected}
      mark={<LabelMark label={rule.label} />}
      text={rule.text}
      trailer={
        <>
          <IdChips icon={conceptIcons.rule} id={rule.id} file={rule.file} />
          <IconChip icon={conceptIcons.subject} title="Subject">
            {rule.subject}
          </IconChip>
          {rule.kind === 'mechanical' && (
            <span data-testid="check-chip" className="contents">
              <IconChip icon={conceptIcons.terminal} title={`Check: ${rule.check}`}>
                {rule.check}
              </IconChip>
            </span>
          )}
          <RestsOn item={rule} />
        </>
      }
    />
  )
}

// Open actions first, then done ones, each in its own fixed panel.
function ActionGroups({ actions, view, selected }: { actions: Action[]; view: View; selected?: string }) {
  const groups = [
    { title: 'To do', done: false, items: actions.filter((a) => !a.done), empty: 'Nothing to do' },
    { title: 'Done', done: true, items: actions.filter((a) => a.done), empty: 'Nothing done yet' },
  ]
  return (
    <div className="flex flex-col gap-4">
      {groups.map(({ title, done, items, empty }) => (
        <section
          key={title}
          aria-label={title}
          data-testid="action-group"
          className="flex flex-col gap-2 rounded-lg border border-zinc-200 p-2 dark:border-zinc-800"
        >
          <h2 className="flex items-center gap-1.5 px-1 text-sm font-medium">
            <DoneMark done={done} />
            {title}
            <span className="font-mono text-xs text-zinc-500 tabular-nums">{items.length}</span>
          </h2>
          {items.length === 0 && <EmptySlot reason={empty} />}
          <Cards>
            {items.map((a) => (
              <ActionCard key={a.id} action={a} view={view} selected={a.id === selected} />
            ))}
          </Cards>
        </section>
      ))}
    </div>
  )
}

function ActionCard({ action, view, selected }: { action: Action; view: View; selected: boolean }) {
  const done = Boolean(action.done)
  return (
    <Card
      to={itemPath('actions', action.id)}
      id={action.id}
      selected={selected}
      mark={<ActionMark done={done} />}
      text={actionSentence(action, view.proper)}
      raw={action.text}
      dim={done}
      trailer={
        <>
          <IconChip icon={conceptIcons.action} title="Id">
            {action.id}
          </IconChip>
          <ObservedChips action={action} />
          <ServesChips action={action} found={view.found} />
          {!done && <RestsOn item={action} />}
        </>
      }
    />
  )
}

function EvidenceCard({ evidence, view, selected }: { evidence: Evidence; view: View; selected: boolean }) {
  const Eye = conceptIcons.evidence
  const Belief = conceptIcons.belief
  const cited = evidence.cited_by.length
  return (
    <Card
      to={itemPath('evidence', evidence.id)}
      id={evidence.id}
      selected={selected}
      mark={<Eye aria-hidden className="size-4 text-zinc-500" />}
      text={evidenceSentence(evidence, view.proper)}
      raw={evidence.text}
      struck={Boolean(evidence.retracted)}
      badge={
        <span
          data-testid="cited-by"
          title={`Cited by ${cited} ${cited === 1 ? 'belief' : 'beliefs'}`}
          className={cn(
            'inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 font-mono text-xs tabular-nums',
            cited > 0
              ? 'border-zinc-300 text-zinc-700 dark:border-zinc-700 dark:text-zinc-300'
              : 'border-dashed border-zinc-300 text-zinc-500 dark:border-zinc-700',
          )}
        >
          <Belief aria-hidden className="size-3" />
          {cited}
        </span>
      }
      trailer={
        <>
          <IdChips icon={conceptIcons.evidence} id={evidence.id} file={evidence.file} />
          {evidence.retracted && <RetractedChip retracted={evidence.retracted} />}
        </>
      }
    />
  )
}

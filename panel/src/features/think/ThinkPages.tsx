// /think/beliefs, /think/decisions, /think/rules and /think/evidence: one
// card per item in a fixed single column, and the detail pane on the right
// for /:id. Read-only: the files in git are the source of truth.
import type { ReactNode } from 'react'
import { CalendarDays } from 'lucide-react'
import { Link, useParams } from 'react-router'

import { useKnowledge } from '@/api/hooks'
import type { Belief, Decision, Evidence, Knowledge, Rule } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { DetailPane } from './DetailPane'
import { LabelMark } from './LabelMark'
import { itemPath, KINDS, RestsOn, RetractedChip, type Kind } from './bits'

export function BeliefsPage() {
  usePageTitle('Think · Beliefs')
  return <ThinkPage kind="beliefs" />
}

export function DecisionsPage() {
  usePageTitle('Think · Decisions')
  return <ThinkPage kind="decisions" />
}

export function RulesPage() {
  usePageTitle('Think · Rules')
  return <ThinkPage kind="rules" />
}

export function EvidencePage() {
  usePageTitle('Think · Evidence')
  return <ThinkPage kind="evidence" />
}

function ThinkPage({ kind }: { kind: Kind }) {
  const { id } = useParams()
  const query = useKnowledge()
  const data = query.data
  const { title, icon: Icon } = KINDS[kind]

  return (
    <div className="flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-y-auto p-6">
        <div className="flex max-w-2xl flex-col gap-3">
          <span
            title={title}
            className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
          >
            <Icon className="size-4" />
            {data ? data[kind].length : '–'}
          </span>
          <ErrorText error={query.error} />
          {query.isPending && (
            <div className="flex flex-col gap-2" aria-label={`Loading ${kind}`}>
              <Skeleton className="h-16" />
              <Skeleton className="h-16" />
              <Skeleton className="h-16" />
            </div>
          )}
          {data && <List kind={kind} data={data} selected={id} />}
        </div>
      </div>
      {id && data && <DetailPane key={`${kind}/${id}`} kind={kind} id={id} data={data} />}
    </div>
  )
}

function List({ kind, data, selected }: { kind: Kind; data: Knowledge; selected?: string }) {
  if (data[kind].length === 0) {
    const nothing = data.evidence.length + data.beliefs.length + data.decisions.length + data.rules.length === 0
    const reason = nothing ? `No knowledge files in ${data.dir}` : `No ${kind} in ${data.dir}`
    return <EmptySlot reason={reason} className="h-24" />
  }
  if (kind === 'beliefs') {
    return (
      <Cards>
        {data.beliefs.map((b) => (
          <BeliefCard key={b.id} belief={b} selected={b.id === selected} />
        ))}
      </Cards>
    )
  }
  if (kind === 'decisions') {
    return (
      <Cards>
        {data.decisions.map((d) => (
          <DecisionCard key={d.id} decision={d} selected={d.id === selected} />
        ))}
      </Cards>
    )
  }
  if (kind === 'rules') {
    return <RuleGroups rules={data.rules} selected={selected} />
  }
  return (
    <Cards>
      {data.evidence.map((e) => (
        <EvidenceCard key={e.id} evidence={e} selected={e.id === selected} />
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
  // The left slot: a LabelMark, or the evidence icon.
  mark: ReactNode
  text: string
  struck?: boolean
  // Mono chips under the text.
  trailer: ReactNode
  // Pinned to the right of the text, e.g. a count badge.
  badge?: ReactNode
}

// One item: the mark, the text as the primary line, and a trailer of chips.
// The whole card opens the detail pane.
function Card({ to, id, selected, mark, text, struck, trailer, badge }: CardProps) {
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
            className={cn('min-w-0 flex-1 text-sm', struck && 'text-zinc-500 line-through dark:text-zinc-500')}
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

function BeliefCard({ belief, selected }: { belief: Belief; selected: boolean }) {
  return (
    <Card
      to={itemPath('beliefs', belief.id)}
      id={belief.id}
      selected={selected}
      mark={<LabelMark label={belief.label} />}
      text={belief.text}
      trailer={
        <>
          <IdChips icon={conceptIcons.belief} id={belief.id} file={belief.file} />
          <RestsOn item={belief} />
        </>
      }
    />
  )
}

function DecisionCard({ decision, selected }: { decision: Decision; selected: boolean }) {
  return (
    <Card
      to={itemPath('decisions', decision.id)}
      id={decision.id}
      selected={selected}
      mark={<LabelMark label={decision.label} />}
      text={decision.text}
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

function EvidenceCard({ evidence, selected }: { evidence: Evidence; selected: boolean }) {
  const Eye = conceptIcons.evidence
  const Belief = conceptIcons.belief
  const cited = evidence.cited_by.length
  return (
    <Card
      to={itemPath('evidence', evidence.id)}
      id={evidence.id}
      selected={selected}
      mark={<Eye aria-hidden className="size-4 text-zinc-500" />}
      text={evidence.text}
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
          <IconChip icon={conceptIcons.human} title="Seen by">
            {evidence.who}
          </IconChip>
          <IconChip icon={CalendarDays} title="Seen on">
            {evidence.when}
          </IconChip>
          {evidence.retracted && <RetractedChip retracted={evidence.retracted} />}
        </>
      }
    />
  )
}

// The right-hand pane for one item: what it is, what it rests on (Because,
// down to the evidence) and what falls if it is wrong.
import type { ReactNode } from 'react'
import { CalendarDays, Link2, ListTree, TriangleAlert, X } from 'lucide-react'
import { Link } from 'react-router'

import { useDoubt, useWhy } from '@/api/hooks'
import type { Knowledge, WhyEvidence, WhyNode } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { LabelMark } from './LabelMark'
import {
  indexKnowledge,
  ItemRow,
  itemPath,
  KINDS,
  RestsOn,
  RetractedChip,
  Section,
  type Found,
  type Kind,
} from './bits'

type DetailPaneProps = {
  kind: Kind
  id: string
  data: Knowledge
}

export function DetailPane({ kind, id, data }: DetailPaneProps) {
  const found = indexKnowledge(data)
  return (
    <aside
      aria-label="Detail"
      className="relative flex w-[28rem] shrink-0 flex-col gap-4 overflow-y-auto border-l border-zinc-200 p-4 dark:border-zinc-800 [&>section]:border-t [&>section]:border-zinc-200 [&>section]:pt-4 dark:[&>section]:border-zinc-800"
    >
      <Link
        to={itemPath(kind)}
        aria-label="Close"
        title="Close"
        className="absolute top-3 right-3 rounded p-1 text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
      >
        <X className="size-4" />
      </Link>
      <Detail kind={kind} id={id} data={data} found={found} />
    </aside>
  )
}

function Detail({ kind, id, data, found }: DetailPaneProps & { found: Map<string, Found> }) {
  if (kind === 'beliefs') {
    const belief = data.beliefs.find((b) => b.id === id)
    if (belief) {
      return (
        <>
          <Header label={belief.label} text={belief.text}>
            <IconChip icon={conceptIcons.belief} title="Id">
              {belief.id}
            </IconChip>
            <IconChip icon={conceptIcons.file} title="File">
              {belief.file}
            </IconChip>
            <RestsOn item={belief} />
          </Header>
          <Because id={id} found={found} />
          <IfWrong id={id} />
        </>
      )
    }
  }
  if (kind === 'decisions') {
    const decision = data.decisions.find((d) => d.id === id)
    if (decision) {
      return (
        <>
          <Header label={decision.label} text={decision.text}>
            <IconChip icon={conceptIcons.decision} title="Id">
              {decision.id}
            </IconChip>
            <IconChip icon={conceptIcons.file} title="File">
              {decision.file}
            </IconChip>
            {decision.adr && (
              <IconChip icon={conceptIcons.file} title="ADR">
                {decision.adr.split('/').at(-1)}
              </IconChip>
            )}
            <IconChip icon={conceptIcons.human} title="Decided by">
              {decision.decided.who}
            </IconChip>
            <IconChip icon={CalendarDays} title="Decided on">
              {decision.decided.when}
            </IconChip>
            <RestsOn item={decision} />
          </Header>
          <Because id={id} found={found} />
          <IfWrong id={id} />
        </>
      )
    }
  }
  if (kind === 'rules') {
    const rule = data.rules.find((r) => r.id === id)
    if (rule) {
      return (
        <>
          <Header label={rule.label} text={rule.text}>
            <IconChip icon={conceptIcons.rule} title="Id">
              {rule.id}
            </IconChip>
            <IconChip icon={conceptIcons.file} title="File">
              {rule.file}
            </IconChip>
            <IconChip icon={conceptIcons.subject} title="Subject">
              {rule.subject}
            </IconChip>
            {rule.kind === 'mechanical' && (
              <IconChip icon={conceptIcons.terminal} title={`Check: ${rule.check}`}>
                {rule.check}
              </IconChip>
            )}
            <RestsOn item={rule} />
          </Header>
          <Because id={id} found={found} />
        </>
      )
    }
  }
  if (kind === 'evidence') {
    const evidence = data.evidence.find((e) => e.id === id)
    if (evidence) {
      const citing = data.beliefs.filter((b) => evidence.cited_by.includes(b.id))
      return (
        <>
          <div className="flex flex-col gap-2">
            <div className="flex items-start gap-2 pr-6">
              <conceptIcons.evidence aria-hidden className="mt-0.5 size-4 shrink-0 text-zinc-500" />
              <p className={cn('text-sm font-medium', evidence.retracted && 'text-zinc-500 line-through')}>
                {evidence.text}
              </p>
            </div>
            <div className="flex flex-wrap gap-1.5">
              <IconChip icon={conceptIcons.evidence} title="Id">
                {evidence.id}
              </IconChip>
              <IconChip icon={conceptIcons.file} title="File">
                {evidence.file}
              </IconChip>
              <IconChip icon={conceptIcons.human} title="Seen by">
                {evidence.who}
              </IconChip>
              <IconChip icon={CalendarDays} title="Seen on">
                {evidence.when}
              </IconChip>
              {evidence.source && (
                <IconChip icon={Link2} title="Source">
                  {evidence.source}
                </IconChip>
              )}
              {evidence.retracted && <RetractedChip retracted={evidence.retracted} />}
            </div>
          </div>
          <Section title="Cited by" icon={conceptIcons.belief} count={citing.length}>
            {citing.length === 0 && <EmptySlot reason="No belief cites this" />}
            {citing.map((b) => (
              <ItemRow key={b.id} kind="beliefs" id={b.id} label={b.label} text={b.text} />
            ))}
          </Section>
          <IfWrong id={id} />
        </>
      )
    }
  }
  return <EmptySlot reason={`No ${KINDS[kind].one} ${id} in ${data.dir}`} />
}

function Header({ label, text, children }: { label: string; text: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <LabelMark label={label} className="self-start" />
      <p className="text-sm font-medium">{text}</p>
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </div>
  )
}

// The item's because chain as an indented tree, down to the evidence.
function Because({ id, found }: { id: string; found: Map<string, Found> }) {
  const why = useWhy(id)
  const root = why.data
  return (
    <Section title="Because" icon={ListTree}>
      <ErrorText error={why.error} />
      {why.isPending && <Skeleton className="h-16" />}
      {root && root.because.length === 0 && root.evidence.length === 0 && <EmptySlot reason="Rests on nothing" />}
      {root && (
        <div data-testid="why-tree" className="flex flex-col gap-1">
          {root.evidence.map((e) => (
            <EvidenceRow key={e.id} evidence={e} />
          ))}
          {root.because.map((node) => (
            <WhyBranch key={node.id} node={node} found={found} />
          ))}
        </div>
      )}
    </Section>
  )
}

function WhyBranch({ node, found }: { node: WhyNode; found: Map<string, Found> }) {
  const at = found.get(node.id)
  return (
    <div data-testid="why-node" data-id={node.id} className="flex flex-col gap-1">
      <ItemRow kind={at?.kind ?? 'beliefs'} id={node.id} label={at?.label} text={node.text} />
      {(node.evidence.length > 0 || node.because.length > 0) && (
        <div className="ml-3 flex flex-col gap-1 border-l border-zinc-200 pl-3 dark:border-zinc-800">
          {node.evidence.map((e) => (
            <EvidenceRow key={e.id} evidence={e} />
          ))}
          {node.because.map((child) => (
            <WhyBranch key={child.id} node={child} found={found} />
          ))}
        </div>
      )}
    </div>
  )
}

// "who · when" with the evidence icon; struck through with the why in the
// tooltip once retracted.
function EvidenceRow({ evidence }: { evidence: WhyEvidence }) {
  const { retracted } = evidence
  const Eye = conceptIcons.evidence
  return (
    <Link
      to={itemPath('evidence', evidence.id)}
      data-testid="evidence-row"
      data-retracted={Boolean(retracted)}
      title={
        retracted
          ? `Retracted by ${retracted.who} on ${retracted.when}: ${retracted.why}`
          : `${evidence.id}: ${evidence.text}`
      }
      className={cn(
        'flex items-center gap-2 rounded px-1 py-0.5 font-mono text-xs text-zinc-600 transition-colors hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800/60',
        retracted && 'text-red-700 line-through dark:text-red-400',
      )}
    >
      <Eye aria-hidden className="size-3.5 shrink-0" />
      <span className="shrink-0">
        {evidence.who} · {evidence.when}
      </span>
      <span className="truncate text-zinc-500">{evidence.id}</span>
    </Link>
  )
}

// What rests on id: three fixed groups, or one empty slot when nothing does.
function IfWrong({ id }: { id: string }) {
  const doubt = useDoubt(id)
  const d = doubt.data
  const empty = d && d.beliefs.length + d.decisions.length + d.rules.length === 0
  return (
    <Section title="If this is wrong" icon={TriangleAlert}>
      <ErrorText error={doubt.error} />
      {doubt.isPending && <Skeleton className="h-16" />}
      {empty && <EmptySlot reason="Nothing rests on this" />}
      {d && !empty && (
        <div data-testid="doubt-groups" className="flex flex-col gap-3">
          <Section sub title="Beliefs" icon={conceptIcons.belief} count={d.beliefs.length}>
            {d.beliefs.map((b) => (
              <ItemRow key={b.id} kind="beliefs" id={b.id} label={b.label} text={b.text} />
            ))}
          </Section>
          <Section sub title="Decisions" icon={conceptIcons.decision} count={d.decisions.length}>
            {d.decisions.map((x) => (
              <ItemRow key={x.id} kind="decisions" id={x.id} label={x.label} text={x.text} />
            ))}
          </Section>
          <Section sub title="Rules" icon={conceptIcons.rule} count={d.rules.length}>
            {d.rules.map((r) => (
              <ItemRow key={r.id} kind="rules" id={r.id} label={r.label} text={r.text} />
            ))}
          </Section>
        </div>
      )}
    </Section>
  )
}

// The right-hand pane for one item: what it is, what it rests on (Because,
// down to the evidence, with each alternative set apart under Or) and what
// falls if it is wrong. Beliefs and evidence get a what-if switch.
import type { ReactNode } from 'react'
import { CalendarDays, FlaskConical, Link2, ListTree, PenLine, TriangleAlert, X } from 'lucide-react'
import { Link, useSearchParams } from 'react-router'

import { useDoubt, useWhy } from '@/api/hooks'
import type { Knowledge, WhyEvidence, WhyNode } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

import { LabelMark } from './LabelMark'
import {
  ActionMark,
  ByChip,
  ContestedBy,
  indexKnowledge,
  ItemRow,
  itemPath,
  KINDS,
  ObservedChips,
  RestsOn,
  RetractedChip,
  Section,
  ServesChips,
  WHY_KINDS,
  type Found,
  type Kind,
} from './bits'
import { actionSentence, beliefSentence, decisionSentence, evidenceSentence, properWords } from './sentence'

type DetailPaneProps = {
  kind: Kind
  id: string
  data: Knowledge
  // The id the page is judged without (?without=), if any.
  without?: string
}

type Ctx = DetailPaneProps & { found: Map<string, Found>; proper: Set<string> }

export function DetailPane({ kind, id, data, without }: DetailPaneProps) {
  const found = indexKnowledge(data)
  const proper = properWords(data)
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
      <Detail kind={kind} id={id} data={data} without={without} found={found} proper={proper} />
    </aside>
  )
}

function Detail({ kind, id, data, without, found, proper }: Ctx) {
  if (kind === 'beliefs') {
    const belief = data.beliefs.find((b) => b.id === id)
    if (belief) {
      return (
        <>
          <Header
            mark={<LabelMark label={belief.label} />}
            text={beliefSentence(belief, proper, data.without ?? [])}
            raw={belief.text}
            note={belief.experience && `${belief.experience.of}'s experience in ${belief.experience.in}`}
          >
            <IconChip icon={conceptIcons.belief} title="Id">
              {belief.id}
            </IconChip>
            <IconChip icon={conceptIcons.file} title="File">
              {belief.file}
            </IconChip>
            <ByChip belief={belief} />
            <RestsOn item={belief} />
            <ContestedBy ids={belief.contested_by ?? []} found={found} />
          </Header>
          <WhatIf id={id} on={without === id} />
          <Because id={id} kind={kind} found={found} />
          <IfWrong id={id} found={found} />
        </>
      )
    }
  }
  if (kind === 'decisions') {
    const decision = data.decisions.find((d) => d.id === id)
    if (decision) {
      return (
        <>
          <Header
            mark={<LabelMark label={decision.label} />}
            text={decisionSentence(decision.text, proper)}
            raw={decision.text}
          >
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
          <Because id={id} kind={kind} found={found} />
          <IfWrong id={id} found={found} />
        </>
      )
    }
  }
  if (kind === 'rules') {
    const rule = data.rules.find((r) => r.id === id)
    if (rule) {
      return (
        <>
          <Header mark={<LabelMark label={rule.label} />} text={rule.text}>
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
          <Because id={id} kind={kind} found={found} />
        </>
      )
    }
  }
  if (kind === 'actions') {
    const action = (data.actions ?? []).find((a) => a.id === id)
    if (action) {
      return (
        <>
          <Header
            mark={
              <span className="flex items-center gap-3">
                <ActionMark done={Boolean(action.done)} />
                <LabelMark label={action.label} />
              </span>
            }
            text={actionSentence(action, proper)}
            raw={action.text}
          >
            <IconChip icon={conceptIcons.action} title="Id">
              {action.id}
            </IconChip>
            <IconChip icon={conceptIcons.file} title="File">
              {action.file}
            </IconChip>
            <ObservedChips action={action} />
            <ServesChips action={action} found={found} />
            {!action.done && <RestsOn item={action} />}
          </Header>
          <Because id={id} kind={kind} found={found} />
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
          <Header
            mark={<conceptIcons.evidence aria-hidden className="size-4 shrink-0 text-zinc-500" />}
            text={evidenceSentence(evidence, proper)}
            raw={evidence.text}
            struck={Boolean(evidence.retracted)}
          >
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
            {evidence.translated_by && (
              <IconChip icon={PenLine} title="Written up from the source by">
                {evidence.translated_by}
              </IconChip>
            )}
            {evidence.source && (
              <IconChip icon={Link2} title="Source">
                {evidence.source}
              </IconChip>
            )}
            {evidence.retracted && <RetractedChip retracted={evidence.retracted} />}
          </Header>
          <WhatIf id={id} on={without === id} />
          <Section title="Cited by" icon={conceptIcons.belief} count={citing.length}>
            {citing.length === 0 && <EmptySlot reason="No belief cites this" />}
            {citing.map((b) => (
              <ItemRow key={b.id} kind="beliefs" id={b.id} label={b.label} text={b.text} />
            ))}
          </Section>
          <IfWrong id={id} found={found} />
        </>
      )
    }
  }
  return <EmptySlot reason={`No ${KINDS[kind].one} ${id} in ${data.dir}`} />
}

function Header({
  mark,
  text,
  raw,
  struck,
  note,
  children,
}: {
  mark: ReactNode
  text: string
  // The text as written in the file, in a tooltip over the sentence.
  raw?: string
  struck?: boolean
  // A dimmed line under the sentence, e.g. whose experience a judgment
  // comes from.
  note?: string | null
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-2">
      <div className="self-start">{mark}</div>
      <p
        data-testid="detail-text"
        title={raw}
        className={cn('pr-6 text-sm font-medium', struck && 'text-zinc-500 line-through')}
      >
        {text}
      </p>
      {note && (
        <p data-testid="detail-note" className="text-xs text-zinc-500 dark:text-zinc-400">
          {note}
        </p>
      )}
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </div>
  )
}

// Judge the whole page as if this belief or evidence were gone. The state
// lives in ?without=, so the list and this pane read the same answer;
// nothing is written, and switching off (or opening another item) shows
// the real state again.
function WhatIf({ id, on }: { id: string; on: boolean }) {
  const [, setParams] = useSearchParams()
  return (
    <label
      title="Labels everywhere are judged as if this were retracted or absent. Nothing is written."
      className={cn(
        'flex cursor-pointer items-center gap-2 rounded-md border px-2 py-1.5 text-sm transition-colors',
        on
          ? 'border-amber-500/50 bg-amber-500/10 text-amber-800 dark:text-amber-300'
          : 'border-zinc-200 text-zinc-600 hover:bg-zinc-50 dark:border-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-900/60',
      )}
    >
      <FlaskConical aria-hidden className="size-4 shrink-0" />
      <span className="flex-1">What if this were gone?</span>
      <Switch
        data-testid="what-if"
        checked={on}
        onCheckedChange={(next) => setParams(next ? { without: id } : {}, { replace: true })}
        className="data-[state=checked]:bg-amber-500"
      />
    </label>
  )
}

// The item's because chain as an indented tree, down to the evidence, with
// each alternative set of reasons in its own Or group.
function Because({ id, kind, found }: { id: string; kind: Kind; found: Map<string, Found> }) {
  const why = useWhy(id)
  const root = why.data
  return (
    <Section title="Because" icon={ListTree}>
      <ErrorText error={why.error} />
      {why.isPending && <Skeleton className="h-16" />}
      {root && root.because.length === 0 && root.evidence.length === 0 && (root.or ?? []).length === 0 && (
        <EmptySlot reason="Rests on nothing" />
      )}
      {root && (
        <div data-testid="why-tree" className="flex flex-col gap-1">
          <Reasons node={root} found={found} />
          <OrGroups node={root} kind={kind} found={found} />
        </div>
      )}
    </Section>
  )
}

// A node's own set of reasons: its evidence, then the beliefs and
// decisions it rests on.
function Reasons({ node, found }: { node: WhyNode; found: Map<string, Found> }) {
  return (
    <>
      {node.evidence.map((e) => (
        <EvidenceRow key={e.id} evidence={e} />
      ))}
      {node.because.map((child) => (
        <WhyBranch key={child.id} node={child} found={found} />
      ))}
    </>
  )
}

// Each other set of reasons, apart, with a left border in the owning
// item's concept color, so two independent reasons read as a split.
function OrGroups({ node, kind, found }: { node: WhyNode; kind: Kind; found: Map<string, Found> }) {
  return (
    <>
      {(node.or ?? []).map((set, i) => (
        <section
          key={i}
          aria-label="Or"
          data-testid="why-or"
          className={cn('mt-1 flex flex-col gap-1 border-l-2 pl-3', KINDS[kind].edge)}
        >
          <h4 className="text-xs font-medium text-zinc-500 dark:text-zinc-400">Or</h4>
          {set.map((n) =>
            n.kind === 'evidence' ? (
              <EvidenceRow
                key={n.id}
                evidence={{
                  id: n.id,
                  text: n.text,
                  who: n.who ?? '',
                  when: n.when ?? '',
                  translated_by: n.translated_by ?? '',
                  retracted: n.retracted ?? null,
                }}
              />
            ) : (
              <WhyBranch key={n.id} node={n} found={found} />
            ),
          )}
        </section>
      ))}
    </>
  )
}

function WhyBranch({ node, found }: { node: WhyNode; found: Map<string, Found> }) {
  const at = found.get(node.id)
  const kind = at?.kind ?? (node.kind ? WHY_KINDS[node.kind] : 'beliefs')
  const hasReasons = node.evidence.length > 0 || node.because.length > 0 || (node.or ?? []).length > 0
  return (
    <div data-testid="why-node" data-id={node.id} className="flex flex-col gap-1">
      <ItemRow kind={kind} id={node.id} label={at?.label} text={node.text} />
      {hasReasons && (
        <div className="ml-3 flex flex-col gap-1 border-l border-zinc-200 pl-3 dark:border-zinc-800">
          <Reasons node={node} found={found} />
          <OrGroups node={node} kind={kind} found={found} />
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

// What rests on id: four fixed groups and, when some rest on it but still
// have other support, a dimmed fifth; or one empty slot when nothing does.
// Marks come from the page's answer, so a what-if relabels them too.
function IfWrong({ id, found }: { id: string; found: Map<string, Found> }) {
  const doubt = useDoubt(id)
  const d = doubt.data
  const actions = d?.actions ?? []
  const s = d?.standing
  const standing = s ? s.beliefs.length + s.decisions.length + s.rules.length + s.actions.length : 0
  const empty = d && d.beliefs.length + d.decisions.length + d.rules.length + actions.length + standing === 0
  return (
    <Section title="If this is wrong" icon={TriangleAlert}>
      <ErrorText error={doubt.error} />
      {doubt.isPending && <Skeleton className="h-16" />}
      {empty && <EmptySlot reason="Nothing rests on this" />}
      {d && !empty && (
        <div data-testid="doubt-groups" className="flex flex-col gap-3">
          <Section sub title="Beliefs" icon={conceptIcons.belief} count={d.beliefs.length}>
            {d.beliefs.map((b) => (
              <ItemRow key={b.id} kind="beliefs" id={b.id} label={found.get(b.id)?.label ?? b.label} text={b.text} />
            ))}
          </Section>
          <Section sub title="Decisions" icon={conceptIcons.decision} count={d.decisions.length}>
            {d.decisions.map((x) => (
              <ItemRow key={x.id} kind="decisions" id={x.id} label={found.get(x.id)?.label ?? x.label} text={x.text} />
            ))}
          </Section>
          <Section sub title="Rules" icon={conceptIcons.rule} count={d.rules.length}>
            {d.rules.map((r) => (
              <ItemRow key={r.id} kind="rules" id={r.id} label={found.get(r.id)?.label ?? r.label} text={r.text} />
            ))}
          </Section>
          <Section sub title="Actions" icon={conceptIcons.action} count={actions.length}>
            {actions.map((a) => (
              <ItemRow key={a.id} kind="actions" id={a.id} label={found.get(a.id)?.label ?? a.label} text={a.text} />
            ))}
          </Section>
          {s && standing > 0 && (
            <Section sub title="Still standing (other support)" icon={ListTree} count={standing} className="opacity-60">
              {[...s.beliefs, ...s.decisions, ...s.rules, ...s.actions].map((x) => (
                <ItemRow
                  key={x.id}
                  kind={found.get(x.id)?.kind ?? 'beliefs'}
                  id={x.id}
                  label={found.get(x.id)?.label}
                  text={x.text}
                />
              ))}
            </Section>
          )}
        </div>
      )}
    </Section>
  )
}

// The contested contradictions, at the top of the Beliefs page and only
// when there are some: "A, but B", with what each side comes down to in
// two columns. The panel never picks a side; a human retracts one in the
// files.
import { Undo2 } from 'lucide-react'
import { Link } from 'react-router'

import { useConflicts } from '@/api/hooks'
import type { Conflict, ConflictSide, Knowledge } from '@/api/knowledge'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'

import { LabelMark, TwoCircles } from './LabelMark'
import { itemPath } from './bits'
import { sideClause } from './sentence'

export function Conflicts({ data, proper }: { data: Knowledge; proper: Set<string> }) {
  const query = useConflicts()
  const conflicts = query.data?.conflicts ?? []
  if (query.error) {
    return <ErrorText error={query.error} />
  }
  if (conflicts.length === 0) {
    return null
  }
  return (
    <section aria-label="Conflicts" data-testid="conflicts" className="flex flex-col gap-2">
      <h2
        title="Contradictions where both sides still stand"
        className="flex items-center gap-1.5 text-sm font-medium text-rose-700 dark:text-rose-400"
      >
        <TwoCircles aria-hidden className="size-4" />
        Conflicts
        <span className="font-mono text-xs tabular-nums">{conflicts.length}</span>
      </h2>
      <ul className="flex flex-col gap-2">
        {conflicts.map((c) => (
          <ConflictCard key={`${c.a.id} ${c.b.id}`} conflict={c} data={data} proper={proper} />
        ))}
      </ul>
    </section>
  )
}

function ConflictCard({ conflict, data, proper }: { conflict: Conflict; data: Knowledge; proper: Set<string> }) {
  const { a, b } = conflict
  return (
    <li
      data-testid="conflict"
      data-a={a.id}
      data-b={b.id}
      className="flex flex-col gap-2.5 rounded-md border border-rose-300 bg-rose-500/5 px-3 py-2.5 dark:border-rose-900"
    >
      <p data-testid="conflict-text" className="text-sm">
        {sideClause(a.id, data, proper)}, but {sideClause(b.id, data, proper)}.
      </p>
      <div className="grid grid-cols-2 gap-3">
        <Side side={a} restsOn={conflict.a_rests_on} data={data} />
        <Side side={b} restsOn={conflict.b_rests_on} data={data} />
      </div>
      <p className="flex items-center gap-1.5 border-t border-rose-200 pt-2 text-xs text-zinc-500 dark:border-rose-900/60 dark:text-zinc-400">
        <Undo2 aria-hidden className="size-3.5" />
        Pick one to retract in the files
      </p>
    </li>
  )
}

// One column: the side's mark and id, then the leaves it rests on.
function Side({ side, restsOn, data }: { side: ConflictSide; restsOn: string[]; data: Knowledge }) {
  const belief = data.beliefs.find((x) => x.id === side.id)
  const Eye = conceptIcons.evidence
  return (
    <div data-testid="conflict-side" data-id={side.id} className="flex min-w-0 flex-col gap-1.5">
      <Link
        to={itemPath(side.kind === 'evidence' ? 'evidence' : 'beliefs', side.id)}
        title={side.text}
        className="flex min-w-0 items-center gap-2 rounded px-1 py-0.5 font-mono text-xs transition-colors hover:bg-zinc-100 dark:hover:bg-zinc-800/60"
      >
        {belief ? <LabelMark label={belief.label} /> : <Eye aria-hidden className="size-3.5 shrink-0 text-zinc-500" />}
        <span className="truncate">{side.id}</span>
      </Link>
      <div className="flex flex-wrap gap-1.5 pl-1">
        {restsOn
          .filter((id) => id !== side.id)
          .map((id) => (
            <Leaf key={id} id={id} data={data} />
          ))}
      </div>
    </div>
  )
}

// A leaf: live evidence (eye) or an opinion (lightbulb), linking to it.
function Leaf({ id, data }: { id: string; data: Knowledge }) {
  const evidence = data.evidence.find((e) => e.id === id)
  const Icon = evidence ? conceptIcons.evidence : conceptIcons.belief
  return (
    <Link
      to={itemPath(evidence ? 'evidence' : 'beliefs', id)}
      data-testid="conflict-leaf"
      data-kind={evidence ? 'evidence' : 'belief'}
      title={evidence ? `${evidence.who} · ${evidence.when}: ${evidence.text}` : 'Opinion: no evidence'}
      className="inline-flex max-w-full items-center gap-1 rounded-md border border-zinc-200 px-1.5 py-0.5 font-mono text-xs text-zinc-700 transition-colors hover:bg-zinc-100 dark:border-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-800/60"
    >
      <Icon aria-hidden className="size-3.5 shrink-0 text-zinc-500" />
      <span className="truncate">{id}</span>
    </Link>
  )
}

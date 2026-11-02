// /compute: every sandbox with its task (ADR 0008), as flat rows grouped by
// status, each verbatim, the groups sorted by status light: attention
// (failed), lit (starting, running), idle (unknown and any other word), off
// (stopped). A stopped row recedes.
import { Link } from 'react-router'

import { useSandboxes, useTasks } from '@/api/hooks'
import type { SandboxView } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { StatusLegend, StatusPill, statusTier, tierRank } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { formatMemory, sandboxMoment, shortAge } from './format'
import { SandboxActions, StatusError } from './SandboxParts'

const ORDER = ['failed', 'starting', 'running', 'unknown', 'stopped']
const color = conceptColors.sandbox

export function ComputePage() {
  usePageTitle('Compute')
  const sandboxes = useSandboxes()
  const tasks = useTasks()
  const titles = new Map((tasks.data ?? []).map((task) => [task.name, task.title]))
  const groups = groupByStatus(sandboxes.data ?? [])

  return (
    <div className="flex flex-col gap-5 p-6">
      <div className="flex items-center gap-4">
        <span
          title="Sandboxes"
          data-testid="sandbox-count"
          className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
        >
          <conceptIcons.sandbox className={cn('size-4', color.text)} />
          {sandboxes.data?.length ?? '–'}
        </span>
        <StatusLegend />
      </div>
      <ErrorText error={sandboxes.error} />
      {sandboxes.isPending && (
        <div className="flex flex-col gap-2" aria-label="Loading sandboxes">
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
        </div>
      )}
      {sandboxes.data?.length === 0 && (
        <EmptySlot reason="No sandboxes: allocate compute from a task" className="self-start" />
      )}
      {groups.map(([status, list]) => (
        <section key={status} aria-label={status} className="flex flex-col gap-2">
          <h2 className="flex items-center gap-2">
            <StatusPill status={status} detail={list[0]?.status_error} />
            <span title={`${list.length} ${status}`} data-testid="group-count" className="text-sm tabular-nums">
              {list.length}
            </span>
          </h2>
          <ul className="flex flex-col divide-y divide-border border-y border-border">
            {list.map((sandbox) => (
              <li key={sandbox.id}>
                <SandboxRow sandbox={sandbox} taskTitle={titles.get(sandbox.task)} />
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

// Groups by status light (TIER_ORDER), known statuses in their fixed order
// inside a light, then any other word as it came.
function groupByStatus(list: SandboxView[]) {
  const groups = new Map<string, SandboxView[]>()
  for (const sandbox of list) {
    const group = groups.get(sandbox.status) ?? []
    group.push(sandbox)
    groups.set(sandbox.status, group)
  }
  const rank = (status: string, group: SandboxView[]) => {
    const i = ORDER.indexOf(status)
    return tierRank(status, group[0]?.status_error) * 10 + (i < 0 ? ORDER.length : i)
  }
  return [...groups].sort(([a, ga], [b, gb]) => rank(a, ga) - rank(b, gb))
}

// One sandbox as a flat row (panel/CLAUDE.md "Lists are rows, not
// cards"): its light, its name, the task it serves and the age; under it,
// muted, cpus · memory · image · command, the boot error in red when there
// is one, and small Stop/Start/Delete. The whole row opens the sandbox; the
// task link and the buttons sit above that.
export function SandboxRow({ sandbox, taskTitle }: { sandbox: SandboxView; taskTitle?: string }) {
  const tier = statusTier(sandbox.status, sandbox.status_error)
  const off = tier === 'off'
  const moment = sandboxMoment(sandbox)
  return (
    <article
      aria-label={sandbox.name}
      data-status={sandbox.status}
      data-tier={tier}
      className="relative flex flex-col gap-1 px-3 py-2 text-sm transition-colors hover:bg-zinc-500/5 has-[a:focus-visible]:rounded-md has-[a:focus-visible]:ring-1 has-[a:focus-visible]:ring-violet-500/70 dark:hover:bg-zinc-500/10"
    >
      <div className="flex items-center gap-3">
        <div className="flex w-20 shrink-0">
          <StatusPill status={sandbox.status} detail={sandbox.status_error} />
        </div>
        <Link
          to={`/compute/${encodeURIComponent(sandbox.name)}`}
          className={cn(
            'truncate font-mono outline-none after:absolute after:inset-0 after:content-[""]',
            off && 'text-muted-foreground',
          )}
        >
          {sandbox.name}
        </Link>
        <Link
          to={`/tasks/${encodeURIComponent(sandbox.task)}`}
          title={`Task ${sandbox.task}`}
          className="relative z-10 inline-flex min-w-0 items-center gap-1 text-xs text-muted-foreground hover:text-foreground hover:underline"
        >
          <conceptIcons.task aria-hidden className={cn('size-3.5 shrink-0 opacity-70', conceptColors.task.text)} />
          <span className="truncate">{taskTitle || sandbox.task}</span>
        </Link>
        <span
          title={`${moment.label} ${new Date(moment.at).toLocaleString()}`}
          className="relative z-10 ml-auto shrink-0 font-mono text-xs text-muted-foreground tabular-nums"
        >
          {shortAge(moment.at)}
        </span>
      </div>
      <div
        className={cn(
          'flex flex-wrap items-center gap-x-1.5 gap-y-1 pl-[5.75rem] font-mono text-xs text-muted-foreground',
          off && 'opacity-70',
        )}
      >
        {specParts(sandbox).map(([title, value], i) => (
          <span key={title} className="inline-flex items-center gap-1.5">
            {i > 0 && (
              <span aria-hidden className="opacity-50">
                ·
              </span>
            )}
            <span title={title}>{value}</span>
          </span>
        ))}
        <SandboxActions sandbox={sandbox} quiet className="relative z-10 ml-auto" />
      </div>
      <StatusError sandbox={sandbox} className="pl-[5.75rem]" />
    </article>
  )
}

// cpus, memory, image and command, each with its tooltip, in that order.
function specParts(sandbox: SandboxView) {
  const { spec } = sandbox
  const parts: [string, string][] = []
  if (spec.cpus) {
    parts.push(['CPUs', String(spec.cpus)])
  }
  if (spec.memory) {
    parts.push(['Memory', formatMemory(spec.memory)])
  }
  if (spec.image) {
    parts.push(['Image', spec.image])
  }
  if (sandbox.command.trim()) {
    parts.push(['Command (the agent terminal)', sandbox.command])
  }
  return parts
}

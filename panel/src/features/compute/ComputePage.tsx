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
import { StatusPill, statusTier, tierRank } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { sandboxMoment, shortAge } from './format'
import { Readout, SandboxActions, SpecReadouts, StatusError } from './SandboxParts'

const ORDER = ['failed', 'starting', 'running', 'unknown', 'stopped']
const color = conceptColors.sandbox

// How many sandboxes carry each status word, in the order the groups come;
// the words are the backend's, verbatim.
function StatusCounts({ groups }: { groups: [string, SandboxView[]][] }) {
  if (groups.length === 0) {
    return null
  }
  return (
    <span data-testid="status-counts" className="ml-2 inline-flex items-center gap-4 text-xs text-muted-foreground">
      {groups.map(([status, list]) => (
        <span key={status} data-status-count={status} className="inline-flex items-center gap-1.5">
          <StatusPill status={status} detail={list[0]?.status_error} className="border-0 px-0" />
          <b className="font-mono tabular-nums">{String(list.length).padStart(2, '0')}</b>
        </span>
      ))}
    </span>
  )
}

export function ComputePage() {
  usePageTitle('Compute')
  const sandboxes = useSandboxes()
  const tasks = useTasks()
  const titles = new Map((tasks.data ?? []).map((task) => [task.name, task.title]))
  const groups = groupByStatus(sandboxes.data ?? [])

  return (
    <div data-testid="compute-page" className="flex flex-col gap-5 p-6">
      <div data-testid="compute-header" className="flex items-center gap-4">
        <span
          title="Sandboxes"
          data-testid="sandbox-count"
          className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
        >
          <conceptIcons.sandbox className={cn('size-4', color.text)} />
          {sandboxes.data?.length ?? '–'}
        </span>
        <StatusCounts groups={groups} />
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
// cards"): its light, its name with the command under it, then columns: the
// task it serves, cpus, memory, disk and image as labelled readouts, the
// age, and at the far right Terminal (while running) and Stop/Start/Delete.
// The boot error follows in red when there is one. The whole row opens the
// sandbox; the task link and the controls sit above that.
export function SandboxRow({ sandbox, taskTitle }: { sandbox: SandboxView; taskTitle?: string }) {
  const tier = statusTier(sandbox.status, sandbox.status_error)
  const off = tier === 'off'
  const moment = sandboxMoment(sandbox)
  const command = sandbox.command.trim()
  return (
    <article
      aria-label={sandbox.name}
      data-status={sandbox.status}
      data-tier={tier}
      className="relative flex flex-col gap-1 px-3 py-2 text-sm transition-colors hover:bg-zinc-500/5 has-[a:focus-visible]:rounded-md has-[a:focus-visible]:ring-1 has-[a:focus-visible]:ring-violet-500/70 dark:hover:bg-zinc-500/10"
    >
      <div className="flex items-center gap-4">
        <div data-testid="sandbox-status" className="flex w-20 shrink-0">
          <StatusPill status={sandbox.status} detail={sandbox.status_error} />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <Link
            to={`/compute/${encodeURIComponent(sandbox.name)}`}
            data-testid="sandbox-name"
            className={cn(
              'truncate font-mono outline-none after:absolute after:inset-0 after:content-[""]',
              off && 'text-muted-foreground',
            )}
          >
            {sandbox.name}
          </Link>
          {command && (
            <span title="Command (the agent terminal)" className="truncate font-mono text-xs text-muted-foreground">
              {command}
            </span>
          )}
        </div>
        <div
          data-testid="sandbox-readouts"
          className={cn(
            'grid shrink-0 grid-cols-[10rem_2.5rem_4rem_3.5rem_6rem] items-start gap-x-4 text-xs',
            off && 'opacity-70',
          )}
        >
          <Readout label="Task">
            <Link
              to={`/tasks/${encodeURIComponent(sandbox.task)}`}
              title={`Task ${sandbox.task}`}
              className="relative z-10 inline-flex max-w-full min-w-0 items-center gap-1 text-muted-foreground hover:text-foreground hover:underline"
            >
              <conceptIcons.task aria-hidden className={cn('size-3.5 shrink-0 opacity-70', conceptColors.task.text)} />
              <span className="truncate">{taskTitle || sandbox.task}</span>
            </Link>
          </Readout>
          <SpecReadouts sandbox={sandbox} />
        </div>
        <span
          title={`${moment.label} ${new Date(moment.at).toLocaleString()}`}
          className="relative z-10 w-8 shrink-0 text-right font-mono text-xs text-muted-foreground tabular-nums"
        >
          {shortAge(moment.at)}
        </span>
        <div data-testid="sandbox-acts" className="relative z-10 flex w-64 shrink-0 items-start justify-end gap-2">
          {sandbox.status === 'running' && (
            <Link
              to={`/compute/${encodeURIComponent(sandbox.name)}`}
              data-control="terminal"
              title={`Open the terminals of ${sandbox.name}`}
              className="inline-flex h-6 items-center gap-1 rounded-md px-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            >
              <conceptIcons.terminal aria-hidden className="size-3" />
              Terminal
            </Link>
          )}
          <SandboxActions sandbox={sandbox} quiet />
        </div>
      </div>
      <StatusError sandbox={sandbox} className="pl-[6rem]" />
    </article>
  )
}

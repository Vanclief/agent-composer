// /compute: every sandbox with its task (ADR 0008), as cards grouped by
// status: starting, running, failed, stopped, unknown, then any other word
// the backend sends, each verbatim.
import { Link } from 'react-router'

import { useSandboxes, useTasks } from '@/api/hooks'
import type { SandboxView } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { StatusPill } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { CommandChip, MomentChip, SandboxActions, SpecChips, StatusError, TaskChip } from './SandboxParts'

const ORDER = ['starting', 'running', 'failed', 'stopped', 'unknown']
const color = conceptColors.sandbox

export function ComputePage() {
  usePageTitle('Compute')
  const sandboxes = useSandboxes()
  const tasks = useTasks()
  const titles = new Map((tasks.data ?? []).map((task) => [task.name, task.title]))
  const groups = groupByStatus(sandboxes.data ?? [])

  return (
    <div className="flex flex-col gap-5 p-6">
      <span
        title="Sandboxes"
        data-testid="sandbox-count"
        className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
      >
        <conceptIcons.sandbox className={cn('size-4', color.text)} />
        {sandboxes.data?.length ?? '–'}
      </span>
      <ErrorText error={sandboxes.error} />
      {sandboxes.isPending && (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-3" aria-label="Loading sandboxes">
          <Skeleton className="h-36" />
          <Skeleton className="h-36" />
          <Skeleton className="h-36" />
        </div>
      )}
      {sandboxes.data?.length === 0 && (
        <EmptySlot reason="No sandboxes: allocate compute from a task" className="self-start" />
      )}
      {groups.map(([status, list]) => (
        <section key={status} aria-label={status} className="flex flex-col gap-2">
          <h2 className="flex items-center gap-2">
            <StatusPill status={status} />
            <span title={`${list.length} ${status}`} data-testid="group-count" className="text-sm tabular-nums">
              {list.length}
            </span>
          </h2>
          <ul className="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-3">
            {list.map((sandbox) => (
              <li key={sandbox.id}>
                <SandboxCard sandbox={sandbox} taskTitle={titles.get(sandbox.task)} />
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

// Known statuses in their fixed order, then any other word as it came.
function groupByStatus(list: SandboxView[]) {
  const groups = new Map<string, SandboxView[]>()
  for (const status of ORDER) {
    groups.set(status, [])
  }
  for (const sandbox of list) {
    const group = groups.get(sandbox.status) ?? []
    group.push(sandbox)
    groups.set(sandbox.status, group)
  }
  return [...groups].filter(([, group]) => group.length > 0)
}

// The whole card opens the sandbox; the task chip and buttons sit above the
// card's link so they work on their own.
export function SandboxCard({ sandbox, taskTitle }: { sandbox: SandboxView; taskTitle?: string }) {
  return (
    <article
      aria-label={sandbox.name}
      className={cn(
        'relative flex h-full flex-col gap-2.5 rounded-xl border border-border bg-white p-3.5 shadow-sm transition-colors hover:bg-zinc-50 dark:bg-zinc-900/60 dark:hover:bg-zinc-800/60',
        color.edge,
      )}
    >
      <div className="flex items-center gap-2">
        <StatusPill status={sandbox.status} detail={sandbox.status_error} />
        <conceptIcons.sandbox aria-hidden className={cn('size-4 shrink-0', color.text)} />
        <Link
          to={`/compute/${encodeURIComponent(sandbox.name)}`}
          className="truncate font-mono text-sm font-medium after:absolute after:inset-0 after:content-['']"
        >
          {sandbox.name}
        </Link>
        <MomentChip sandbox={sandbox} className="ml-auto" />
      </div>
      <div className="relative z-10 flex">
        <TaskChip task={sandbox.task} title={taskTitle} />
      </div>
      <div className="flex flex-wrap gap-1.5">
        <SpecChips sandbox={sandbox} />
        <CommandChip command={sandbox.command} />
      </div>
      <StatusError sandbox={sandbox} />
      <SandboxActions sandbox={sandbox} className="relative z-10 mt-auto" />
    </article>
  )
}

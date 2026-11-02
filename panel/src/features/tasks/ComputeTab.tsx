// /tasks/:name/compute, inside the task page's box: the task's sandboxes as
// rows, newest first (ADR 0008), and the selected one's parts below it:
// spec, work status, Stop/Start/Delete and its terminals. ?sandbox=<name>
// keeps the selection in the URL; without it the newest is selected.
// "Allocate compute" sits in the toolbar.
import { useEffect, useState } from 'react'
import { ExternalLink, GitCommitHorizontal, Plus } from 'lucide-react'
import { Link, useParams, useSearchParams } from 'react-router'

import { useSandbox, useTask } from '@/api/hooks'
import type { SandboxView } from '@/api/tasks'
import { conceptColors } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { sandboxMoment } from '@/features/compute/format'
import { CommandChip, SandboxActions, SpecChips, StatusError } from '@/features/compute/SandboxParts'
import { TerminalsTab } from '@/features/terminals/TerminalsTab'
import { cn } from '@/lib/utils'

import { AllocateForm } from './AllocateForm'
import { shortAge } from './format'

const BOOT_POLL_MS = 2000

export function ComputeTab() {
  const { name = '' } = useParams()
  const task = useTask(name)
  const [params, setParams] = useSearchParams()
  const [allocating, setAllocating] = useState(false)
  if (!task.data) {
    // TaskPage shows the loading state and the error.
    return null
  }
  const data = task.data
  const done = data.status === 'done'
  const wanted = params.get('sandbox')
  const selected = data.sandboxes.find((b) => b.name === wanted) ?? data.sandboxes[0]

  function select(sandbox: string) {
    setParams({ sandbox }, { replace: true })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex max-h-[45%] shrink-0 flex-col overflow-y-auto border-b border-border">
        {allocating && !done ? (
          <div className="p-3">
            <AllocateForm task={data} onClose={() => setAllocating(false)} />
          </div>
        ) : (
          <div className="flex items-center px-3 py-2">
            {/* Outline: Accept in the rail is the page's one primary button. */}
            <Button variant="outline" size="sm" disabled={done} onClick={() => setAllocating(true)} className="ml-auto">
              <Plus className={conceptColors.sandbox.text} />{' '}
              {done ? 'Allocate compute: task is done' : 'Allocate compute'}
            </Button>
          </div>
        )}
        {data.sandboxes.length > 0 && (
          <ul aria-label="Sandboxes" className="divide-y divide-border border-t border-border">
            {data.sandboxes.map((sandbox) => (
              <SandboxRow
                key={sandbox.id}
                sandbox={sandbox}
                selected={sandbox.name === selected?.name}
                onSelect={() => select(sandbox.name)}
              />
            ))}
          </ul>
        )}
      </div>
      {selected ? (
        <SandboxDetail key={selected.name} sandbox={selected} task={data.name} />
      ) : (
        !allocating && (
          <div className="grid flex-1 place-items-center p-6">
            <EmptySlot reason="No sandbox allocated yet" />
          </div>
        )
      )}
    </div>
  )
}

// A flat row: status light, name, then image and age dim. The whole row
// selects; Open goes to the sandbox's own page.
function SandboxRow({
  sandbox,
  selected,
  onSelect,
}: {
  sandbox: SandboxView
  selected: boolean
  onSelect: () => void
}) {
  const moment = sandboxMoment(sandbox)
  return (
    <li
      data-sandbox={sandbox.name}
      aria-current={selected ? 'true' : undefined}
      className={cn(
        'relative flex items-center gap-3 border-l-2 py-1.5 pr-2 pl-3 text-sm transition-colors',
        selected
          ? cn(conceptColors.sandbox.bg, 'border-l-blue-500')
          : 'border-l-transparent hover:bg-zinc-50 dark:hover:bg-zinc-800/40',
      )}
    >
      <div className="flex w-20 shrink-0">
        <StatusPill status={sandbox.status} detail={sandbox.status_error} />
      </div>
      <button
        type="button"
        data-testid="sandbox-name"
        onClick={onSelect}
        className="truncate text-left font-mono after:absolute after:inset-0 after:content-['']"
      >
        {sandbox.name}
      </button>
      {sandbox.spec.image && (
        <span title="Image" className="truncate font-mono text-xs text-muted-foreground">
          {sandbox.spec.image}
        </span>
      )}
      <span
        title={`${moment.label} ${new Date(moment.at).toLocaleString()}`}
        className="ml-auto shrink-0 text-xs text-muted-foreground tabular-nums"
      >
        {shortAge(moment.at)}
      </span>
      <Button variant="ghost" size="xs" asChild className="relative z-10 text-muted-foreground">
        <Link to={`/compute/${encodeURIComponent(sandbox.name)}`} title="The sandbox's own page">
          <ExternalLink /> Open
        </Link>
      </Button>
    </li>
  )
}

// The selected sandbox: its spec and work, its controls, then its
// terminals (or why there are none).
function SandboxDetail({ sandbox: row, task }: { sandbox: SandboxView; task: string }) {
  // The task's embedded sandbox has no work; the sandbox read does.
  const live = useSandbox(row.name)
  const sandbox = live.data ?? row
  const { refetch } = live
  const status = sandbox.status

  // useSandbox polls every 5s; a booting sandbox is watched closer.
  useEffect(() => {
    if (status !== 'starting') {
      return
    }
    const timer = window.setInterval(() => void refetch(), BOOT_POLL_MS)
    return () => window.clearInterval(timer)
  }, [status, refetch])

  return (
    <section
      data-testid="sandbox-parts"
      data-sandbox-parts={sandbox.name}
      aria-label={`Sandbox ${sandbox.name}`}
      className="flex min-h-0 flex-1 flex-col"
    >
      <div className="flex shrink-0 flex-wrap items-start gap-2 px-4 pt-3">
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
          <SpecChips sandbox={sandbox} full />
          <CommandChip command={sandbox.command} />
          <WorkChip sandbox={sandbox} />
        </div>
        <SandboxActions sandbox={sandbox} deleteNote={`The task ${task} stays.`} />
      </div>
      <ErrorText error={live.error} className="mx-4 mt-2" />
      <div className="flex min-h-0 flex-1 flex-col">
        {status === 'starting' && (
          <div className="p-4">
            <EmptySlot reason="Booting: terminals appear when the sandbox is running" />
          </div>
        )}
        {status === 'failed' && (
          <div className="p-4">
            <StatusError
              sandbox={sandbox}
              className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm"
            />
          </div>
        )}
        {status !== 'starting' && status !== 'failed' && <TerminalsTab sandbox={sandbox.name} />}
      </div>
    </section>
  )
}

// What stopping would lose, read live from the clone: lit amber when there
// is unpushed or uncommitted work, dim "clean" otherwise. Hidden when agc
// did not check (not running, no repo, or the check failed).
function WorkChip({ sandbox }: { sandbox: SandboxView }) {
  const work = sandbox.work
  if (!work) {
    return null
  }
  const parts = [work.unpushed > 0 ? `${work.unpushed} unpushed` : '', work.dirty ? 'uncommitted changes' : ''].filter(
    Boolean,
  )
  const title = `Work in the clone, checked ${new Date(work.checked_at).toLocaleString()}`
  if (parts.length === 0) {
    return (
      <span data-testid="work" title={title} className="inline-flex">
        <IconChip icon={GitCommitHorizontal}>clean</IconChip>
      </span>
    )
  }
  return (
    <span
      data-testid="work"
      title={title}
      className="inline-flex items-center gap-1 rounded-md border border-amber-500/40 bg-amber-500/10 px-1.5 py-0.5 font-mono text-xs text-amber-700 dark:text-amber-400"
    >
      <GitCommitHorizontal aria-hidden className="size-3.5 shrink-0" />
      {parts.join(' · ')}
    </span>
  )
}

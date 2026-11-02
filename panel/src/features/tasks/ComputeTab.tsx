// /tasks/:name/compute: "Allocate compute" on top, then the task's
// sandboxes, newest first (ADR 0008). Each row carries its own Stop or
// Start, Delete (both say what the clone would lose) and a link to the
// sandbox's page with its terminals.
import { useState } from 'react'
import { ExternalLink, Loader2, Play, Plus, Power, PowerOff, Square, Trash2 } from 'lucide-react'
import { Link, useParams } from 'react-router'

import { useDeleteSandbox, useStartSandbox, useStopSandbox, useTask } from '@/api/hooks'
import type { SandboxView, TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { ConfirmSandbox } from '@/features/compute/LostWork'
import { cn } from '@/lib/utils'

import { AllocateForm } from './AllocateForm'
import { formatMemory, shortAge } from './format'

export function ComputeTab() {
  const { name = '' } = useParams()
  const task = useTask(name)
  const [allocating, setAllocating] = useState(false)
  if (!task.data) {
    // TaskPage shows the loading state and the error.
    return null
  }
  const data = task.data
  const done = data.status === 'done'

  return (
    <div className="h-full overflow-y-auto">
      <div className="flex max-w-5xl flex-col gap-3 p-6">
        <div className="flex items-center gap-3">
          <span
            title="Sandboxes"
            className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
          >
            <conceptIcons.sandbox className={cn('size-4', conceptColors.sandbox.text)} />
            {data.sandboxes.length}
          </span>
          {!allocating && (
            <Button
              disabled={done}
              onClick={() => setAllocating(true)}
              className={cn('ml-auto', !done && conceptColors.sandbox.solid)}
            >
              <Plus /> {done ? 'Allocate compute: task is done' : 'Allocate compute'}
            </Button>
          )}
        </div>
        {allocating && !done && <AllocateForm task={data} onClose={() => setAllocating(false)} />}
        {data.sandboxes.length === 0 && !allocating && <EmptySlot reason="No sandbox allocated yet" className="h-16" />}
        {data.sandboxes.length > 0 && (
          <ul aria-label="Sandboxes" className="flex flex-col gap-2">
            {data.sandboxes.map((sandbox) => (
              <SandboxRow key={sandbox.id} task={data} sandbox={sandbox} />
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function SandboxRow({ task, sandbox }: { task: TaskView; sandbox: SandboxView }) {
  const stop = useStopSandbox()
  const start = useStartSandbox()
  const remove = useDeleteSandbox()
  const { spec } = sandbox
  const SandboxIcon = conceptIcons.sandbox
  const canStop = sandbox.status === 'running'
  const canStart = sandbox.status === 'stopped' || sandbox.status === 'unknown'

  return (
    <li
      data-sandbox={sandbox.name}
      className={cn(
        'flex flex-col gap-2 rounded-xl border bg-white px-4 py-2.5 text-sm shadow-sm dark:bg-zinc-900/60',
        conceptColors.sandbox.border,
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex w-20 shrink-0">
          <StatusPill status={sandbox.status} detail={sandbox.status_error} />
        </div>
        <SandboxIcon aria-hidden className={cn('size-4 shrink-0', conceptColors.sandbox.text)} />
        <span data-testid="sandbox-name" className="font-mono font-medium">
          {sandbox.name}
        </span>
        {spec.cpus !== undefined && (
          <IconChip icon={conceptIcons.cpu} concept="cpu" title="CPUs">
            {spec.cpus}
          </IconChip>
        )}
        {spec.memory !== undefined && (
          <IconChip icon={conceptIcons.memory} concept="memory" title="Memory">
            {formatMemory(spec.memory)}
          </IconChip>
        )}
        {spec.image && (
          <IconChip icon={conceptIcons.image} concept="image" title="Image">
            {spec.image}
          </IconChip>
        )}
        <div className="ml-auto flex shrink-0 items-center gap-2">
          {canStop && (
            <ConfirmSandbox
              sandbox={sandbox.name}
              verb="Stop"
              icon={Square}
              onConfirm={() => stop.mutate(sandbox.name)}
              trigger={
                <Button variant="outline" size="sm" disabled={stop.isPending}>
                  {stop.isPending ? <Loader2 className="animate-spin" /> : <Square />}
                  Stop
                </Button>
              }
            />
          )}
          {canStart && (
            <Button variant="outline" size="sm" disabled={start.isPending} onClick={() => start.mutate(sandbox.name)}>
              {start.isPending ? <Loader2 className="animate-spin" /> : <Play />}
              Start
            </Button>
          )}
          <Button variant="outline" size="sm" asChild>
            <Link to={`/compute/${encodeURIComponent(sandbox.name)}`} title="The sandbox with its terminals">
              <ExternalLink /> Open
            </Link>
          </Button>
          <ConfirmSandbox
            sandbox={sandbox.name}
            verb="Delete"
            icon={Trash2}
            destructive
            onConfirm={() => remove.mutate(sandbox.name)}
            trigger={
              <Button
                variant="outline"
                size="sm"
                disabled={remove.isPending}
                className="ml-2 border-red-500/40 text-red-600 hover:bg-red-500/10 hover:text-red-600 dark:text-red-400"
              >
                {remove.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}
                Delete
              </Button>
            }
          >
            {`The task ${task.name} stays.`}
          </ConfirmSandbox>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2 pl-[5.5rem]">
        {sandbox.command && (
          <IconChip icon={conceptIcons.terminal} concept="terminal" title="Command, run as the job agent">
            {sandbox.command}
          </IconChip>
        )}
        {sandbox.booted_at && (
          <IconChip icon={Power} title={`Booted ${new Date(sandbox.booted_at).toLocaleString()}`}>
            booted {shortAge(sandbox.booted_at)}
          </IconChip>
        )}
        {sandbox.stopped_at && (
          <IconChip icon={PowerOff} title={`Stopped ${new Date(sandbox.stopped_at).toLocaleString()}`}>
            stopped {shortAge(sandbox.stopped_at)}
          </IconChip>
        )}
      </div>
      {sandbox.status_error && (
        <p className="pl-[5.5rem] font-mono text-xs text-red-700 dark:text-red-400">{sandbox.status_error}</p>
      )}
      <ErrorText error={stop.error ?? start.error ?? remove.error} />
    </li>
  )
}

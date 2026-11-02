// Pieces shared by the Compute cards and the sandbox page: the task chip,
// the spec chips, the time chip and the Stop/Start/Delete controls.
import { Clock, Loader2, Play, Square, Trash2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { useDeleteSandbox, useStartSandbox, useStopSandbox } from '@/api/hooks'
import type { SandboxView } from '@/api/tasks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { formatMemory, sandboxMoment, shortAge } from './format'
import { ConfirmSandbox } from './LostWork'

// The task the sandbox was allocated to: its icon and title, linking to it.
export function TaskChip({ task, title, className }: { task: string; title?: string; className?: string }) {
  return (
    <Link
      to={`/tasks/${encodeURIComponent(task)}`}
      title={`Task ${task}`}
      className={cn('inline-flex max-w-full rounded-md hover:brightness-125', className)}
    >
      <IconChip icon={conceptIcons.task} concept="task" className="font-sans">
        {title || task}
      </IconChip>
    </Link>
  )
}

// A spec's secrets entry, verbatim: NAME, or NAME → ENV for NAME as ENV;
// the hosts after @ go in the tooltip.
export function SecretChip({ entry }: { entry: string }) {
  const at = entry.indexOf('@')
  const head = (at < 0 ? entry : entry.slice(0, at)).trim()
  const hosts = at < 0 ? '' : entry.slice(at + 1)
  const as = head.match(/^(\S+)\s+as\s+(\S+)$/)
  return (
    <span data-spec-secret={as ? as[1] : head} className="inline-flex max-w-full">
      <IconChip
        icon={conceptIcons.secret}
        concept="secret"
        title={hosts ? 'Secret ' + entry + ', sent only to ' + hosts : 'Secret ' + entry}
      >
        {as ? (
          <>
            {as[1]} <span className="text-muted-foreground/70">→</span> {as[2]}
          </>
        ) : (
          head
        )}
      </IconChip>
    </span>
  )
}

// cpus, memory and image, plus repo, ref and secrets when full is set.
export function SpecChips({ sandbox, full = false }: { sandbox: SandboxView; full?: boolean }) {
  const { spec } = sandbox
  return (
    <>
      {spec.cpus ? (
        <IconChip icon={conceptIcons.cpu} concept="cpu" title="CPUs">
          {spec.cpus}
        </IconChip>
      ) : null}
      {spec.memory ? (
        <IconChip icon={conceptIcons.memory} concept="memory" title="Memory">
          {formatMemory(spec.memory)}
        </IconChip>
      ) : null}
      {spec.image && (
        <IconChip icon={conceptIcons.image} concept="image" title="Image">
          {spec.image}
        </IconChip>
      )}
      {full && spec.repo && (
        <IconChip icon={conceptIcons.repo} concept="repo" title="Repo">
          {spec.repo}
        </IconChip>
      )}
      {full && spec.ref && (
        <IconChip icon={conceptIcons.ref} concept="ref" title="Ref">
          {spec.ref}
        </IconChip>
      )}
      {full && (spec.secrets ?? []).map((entry) => <SecretChip key={entry} entry={entry} />)}
    </>
  )
}

// A small label over a value: one column of a sandbox's readouts.
export function Readout({ label, children }: { label: string; children: ReactNode }) {
  return (
    <span data-readout={label} className="flex min-w-0 flex-col">
      <span aria-hidden className="text-[10px] tracking-wider text-muted-foreground/70 uppercase">
        {label}
      </span>
      {children}
    </span>
  )
}

// cpus, memory, disk and image as readouts, plus repo, ref and the command
// when full is set, each value with its tooltip. A value the spec leaves out
// reads "–" so columns stay aligned.
export function SpecReadouts({ sandbox, full = false }: { sandbox: SandboxView; full?: boolean }) {
  const { spec } = sandbox
  const parts: [string, string, string][] = [
    ['CPU', 'CPUs', spec.cpus ? String(spec.cpus) : '–'],
    ['Mem', 'Memory', spec.memory ? formatMemory(spec.memory) : '–'],
    ['Disk', spec.disk ? 'Disk' : 'Disk: not set, the default', spec.disk || '–'],
    ['Image', 'Image', spec.image || '–'],
  ]
  if (full) {
    parts.push(['Repo', 'Repo', spec.repo || '–'], ['Ref', 'Ref', spec.ref || '–'])
    if (sandbox.command.trim()) {
      parts.push(['Cmd', 'Command (the agent terminal)', sandbox.command])
    }
  }
  return (
    <>
      {parts.map(([label, title, value]) => (
        <Readout key={label} label={label}>
          <span title={title} className="truncate font-mono text-muted-foreground tabular-nums">
            {value}
          </span>
        </Readout>
      ))}
    </>
  )
}

// The program the sandbox runs as its agent terminal, in mono.
export function CommandChip({ command }: { command: string }) {
  if (!command.trim()) {
    return null
  }
  return (
    <IconChip icon={conceptIcons.terminal} concept="terminal" title="Command (the agent terminal)">
      {command}
    </IconChip>
  )
}

// When it booted, stopped or was allocated; the full time is the tooltip.
export function MomentChip({ sandbox, className }: { sandbox: SandboxView; className?: string }) {
  const moment = sandboxMoment(sandbox)
  return (
    <IconChip
      icon={Clock}
      title={`${moment.label} ${new Date(moment.at).toLocaleString()}`}
      className={cn('shrink-0', className)}
    >
      {shortAge(moment.at)}
    </IconChip>
  )
}

// The boot error of a failed sandbox, verbatim.
export function StatusError({ sandbox, className }: { sandbox: SandboxView; className?: string }) {
  if (!sandbox.status_error) {
    return null
  }
  return (
    <p
      data-testid="status-error"
      className={cn('font-mono text-xs whitespace-pre-wrap text-red-700 dark:text-red-400', className)}
    >
      {sandbox.status_error}
    </p>
  )
}

type SandboxActionsProps = {
  sandbox: SandboxView
  // Runs after the sandbox is deleted.
  onDeleted?: () => void
  // Small ghost buttons for a list row; Delete turns red on hover only.
  quiet?: boolean
  className?: string
  // More of what Delete does, under what is lost.
  deleteNote?: ReactNode
}

// Stop when running, Start when stopped or unknown, and Delete always,
// behind a dialog. Stop and Delete ask first, saying what work the clone
// would lose. A failed sandbox only gets Delete.
export function SandboxActions({ sandbox, onDeleted, quiet = false, className, deleteNote }: SandboxActionsProps) {
  const stop = useStopSandbox()
  const start = useStartSandbox()
  const remove = useDeleteSandbox()
  const busy = stop.isPending || start.isPending || remove.isPending
  const canStop = sandbox.status === 'running'
  const canStart = sandbox.status === 'stopped' || sandbox.status === 'unknown'
  const error = stop.error ?? start.error ?? remove.error
  const variant = quiet ? 'ghost' : 'outline'
  const size = quiet ? 'xs' : 'sm'
  const quietText = quiet && 'text-muted-foreground'

  return (
    <div className={cn('flex flex-col items-end gap-2', className)}>
      <div className="flex items-center gap-2">
        {canStop && (
          <ConfirmSandbox
            sandbox={sandbox.name}
            verb="Stop"
            icon={Square}
            onConfirm={() => stop.mutate(sandbox.name)}
            trigger={
              <Button data-control="stop" variant={variant} size={size} disabled={busy} className={cn(quietText)}>
                {stop.isPending ? <Loader2 className="animate-spin" /> : <Square />}
                Stop
              </Button>
            }
          />
        )}
        {canStart && (
          <Button
            data-control="start"
            variant={variant}
            size={size}
            disabled={busy}
            className={cn(quietText)}
            onClick={() => start.mutate(sandbox.name)}
          >
            {start.isPending ? <Loader2 className="animate-spin" /> : <Play />}
            Start
          </Button>
        )}
        <ConfirmSandbox
          sandbox={sandbox.name}
          verb="Delete"
          icon={Trash2}
          destructive
          onConfirm={() => remove.mutate(sandbox.name, { onSuccess: () => onDeleted?.() })}
          trigger={
            <Button
              data-control="delete"
              variant={variant}
              size={size}
              disabled={busy}
              title="Delete the sandbox"
              className={
                quiet
                  ? 'text-muted-foreground hover:bg-red-500/10 hover:text-red-600 dark:hover:text-red-400'
                  : 'border-red-500/40 text-red-600 hover:bg-red-500/10 hover:text-red-600 dark:text-red-400'
              }
            >
              {remove.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}
              Delete
            </Button>
          }
        >
          {deleteNote}
        </ConfirmSandbox>
      </div>
      <ErrorText error={error} className="text-xs" />
    </div>
  )
}

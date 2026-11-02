// Pieces shared by the Compute cards and the sandbox page: the task chip,
// the spec chips, the time chip and the Stop/Start/Delete controls.
import { Clock, Loader2, Play, Square, Trash2 } from 'lucide-react'
import { Link } from 'react-router'

import { useDeleteSandbox, useStartSandbox, useStopSandbox } from '@/api/hooks'
import type { SandboxView } from '@/api/tasks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { formatMemory, sandboxMoment, shortAge } from './format'

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

// cpus, memory and image, plus repo and ref when full is set.
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
  className?: string
}

// Stop when running, Start when stopped or unknown, and Delete always,
// behind a dialog. A failed sandbox only gets Delete.
export function SandboxActions({ sandbox, onDeleted, className }: SandboxActionsProps) {
  const stop = useStopSandbox()
  const start = useStartSandbox()
  const remove = useDeleteSandbox()
  const busy = stop.isPending || start.isPending || remove.isPending
  const canStop = sandbox.status === 'running'
  const canStart = sandbox.status === 'stopped' || sandbox.status === 'unknown'
  const error = stop.error ?? start.error ?? remove.error

  return (
    <div className={cn('flex flex-col items-end gap-2', className)}>
      <div className="flex items-center gap-2">
        {canStop && (
          <Button variant="outline" size="sm" disabled={busy} onClick={() => stop.mutate(sandbox.name)}>
            {stop.isPending ? <Loader2 className="animate-spin" /> : <Square />}
            Stop
          </Button>
        )}
        {canStart && (
          <Button variant="outline" size="sm" disabled={busy} onClick={() => start.mutate(sandbox.name)}>
            {start.isPending ? <Loader2 className="animate-spin" /> : <Play />}
            Start
          </Button>
        )}
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              title="Delete the sandbox"
              className="border-red-500/40 text-red-600 hover:bg-red-500/10 hover:text-red-600 dark:text-red-400"
            >
              {remove.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}
              Delete
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete {sandbox.name}?</AlertDialogTitle>
              <AlertDialogDescription>Uncommitted work in the sandbox is lost.</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                onClick={() => remove.mutate(sandbox.name, { onSuccess: () => onDeleted?.() })}
              >
                <Trash2 />
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
      <ErrorText error={error} className="text-xs" />
    </div>
  )
}

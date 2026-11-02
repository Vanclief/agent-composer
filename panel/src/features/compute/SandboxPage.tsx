// /compute/:name: one sandbox (ADR 0008): a header with its status, then
// its task and spec as labelled readouts, and its controls, then its terminals. While starting it polls every 2s
// and the terminals area says it is booting; a failed sandbox shows its
// error verbatim and can only be deleted.
import { useEffect } from 'react'
import { Link, useNavigate, useParams } from 'react-router'

import { useSandbox, useTask } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { StatusPill } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { TerminalsTab } from '@/features/terminals/TerminalsTab'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { MomentChip, Readout, SandboxActions, SpecReadouts, StatusError } from './SandboxParts'

const BOOT_POLL_MS = 2000
const color = conceptColors.sandbox

export function SandboxPage() {
  const { name = '' } = useParams()
  usePageTitle(name)
  const navigate = useNavigate()
  const sandbox = useSandbox(name)
  const task = useTask(sandbox.data?.task)
  const status = sandbox.data?.status
  const { refetch } = sandbox

  // useSandbox polls every 5s; a booting sandbox is watched closer.
  useEffect(() => {
    if (status !== 'starting') {
      return
    }
    const timer = window.setInterval(() => void refetch(), BOOT_POLL_MS)
    return () => window.clearInterval(timer)
  }, [status, refetch])

  if (sandbox.isPending) {
    return (
      <div className="flex flex-col gap-4 p-6" aria-label="Loading sandbox">
        <Skeleton className="h-24" />
        <Skeleton className="h-64" />
      </div>
    )
  }
  if (!sandbox.data) {
    return (
      <div className="p-6">
        <ErrorText error={sandbox.error} />
      </div>
    )
  }

  const box = sandbox.data
  return (
    <div className="flex h-full min-h-0 flex-col">
      <header
        data-testid="sandbox-header"
        className={cn(
          'mx-6 mt-6 flex shrink-0 flex-wrap items-start gap-x-4 gap-y-2.5 rounded-xl border border-border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
          color.edge,
        )}
      >
        <div data-testid="sandbox-status" className="flex shrink-0">
          <StatusPill status={box.status} detail={box.status_error} />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2.5">
          <div className="flex items-center gap-2">
            <conceptIcons.sandbox aria-hidden className={cn('size-4 shrink-0', color.text)} />
            <h2 className="truncate font-mono text-base font-semibold">{box.name}</h2>
            <MomentChip sandbox={box} />
          </div>
          <div
            data-testid="sandbox-readouts"
            className="flex flex-wrap items-start gap-x-6 gap-y-2 text-xs [&>[data-readout]]:max-w-72"
          >
            <Readout label="Task">
              <Link
                to={`/tasks/${encodeURIComponent(box.task)}`}
                title={`Task ${box.task}`}
                className="inline-flex max-w-full min-w-0 items-center gap-1 text-muted-foreground hover:text-foreground hover:underline"
              >
                <conceptIcons.task
                  aria-hidden
                  className={cn('size-3.5 shrink-0 opacity-70', conceptColors.task.text)}
                />
                <span className="truncate">{task.data?.title || box.task}</span>
              </Link>
            </Readout>
            <SpecReadouts sandbox={box} full />
          </div>
        </div>
        <SandboxActions sandbox={box} onDeleted={() => void navigate('/compute')} />
      </header>
      <ErrorText error={sandbox.error} className="mx-6 mt-4" />
      <div className="flex min-h-0 flex-1 flex-col">
        {box.status === 'starting' && (
          <div className="p-6">
            <EmptySlot reason="Booting: terminals appear when the sandbox is running" className="self-start" />
          </div>
        )}
        {box.status === 'failed' && (
          <div className="p-6">
            <StatusError
              sandbox={box}
              className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm"
            />
          </div>
        )}
        {box.status !== 'starting' && box.status !== 'failed' && <TerminalsTab />}
      </div>
    </div>
  )
}

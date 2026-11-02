// A sandbox's terminals: a rail of job tiles with quick-start buttons, and
// the selected job's xterm. Panes the user has viewed stay mounted (hidden
// when not selected) so switching is instant and keeps scrollback; jobs
// never viewed are not attached.
// Mounted by SandboxPage at /compute/:name; :name is the sandbox. The job
// "agent" (the sandbox's command) is listed first with the task icon.
import { useCallback, useState, type FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useParams } from 'react-router'

import { queryKeys, useSandbox, useStartTerminal, useTerminals } from '@/api/hooks'
import type { TaskSpec } from '@/api/client'
import type { Terminal } from '@/api/terminals'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import { AGENT_JOB, JobIcon } from './JobIcon'
import { JobShape } from './JobShape'
import { ExitedBadge, TerminalPane } from './TerminalPane'

const QUICK_START = ['claude', 'codex', 'bash']
// The VM user's home; the repo is cloned into a directory under it.
const HOME_DIR = '/home/agent'
const color = conceptColors.terminal

export function TerminalsTab() {
  const { name = '' } = useParams()
  // A fresh view per sandbox, so panes never outlive their sandbox's route.
  return <TerminalsView key={name} name={name} />
}

function TerminalsView({ name }: { name: string }) {
  const queryClient = useQueryClient()
  const sandbox = useSandbox(name)
  const status = sandbox.data?.status
  const running = status === 'running'
  const terminals = useTerminals(name, running)
  const start = useStartTerminal(name)
  const [command, setCommand] = useState('')
  const [picked, setPicked] = useState<string | null>(null)
  const [viewed, setViewed] = useState<string[]>([])
  const [exited, setExited] = useState<ReadonlySet<string>>(new Set())

  const jobs = running ? agentFirst(terminals.data ?? []) : []
  // Until the user picks one, the first running job is shown.
  const selected = picked ?? jobs.find((job) => job.running)?.id ?? null
  const panes = jobs.filter((job) => job.id === selected || viewed.includes(job.id))
  const selectedJob = jobs.find((job) => job.id === selected)
  const dir = repoDir(sandbox.data?.spec)

  const select = (id: string) => {
    setPicked(id)
    setViewed((ids) => unique([...ids, ...(selected ? [selected] : []), id]))
  }

  const startJob = (text: string) => {
    const trimmed = text.trim()
    if (!trimmed) {
      return
    }
    start.mutate(
      { command: trimmed, dir },
      {
        onSuccess: ({ id }) => {
          setCommand('')
          select(id)
        },
      },
    )
  }

  const submit = (event: FormEvent) => {
    event.preventDefault()
    startJob(command)
  }

  const onExited = useCallback(
    (id: string) => {
      setExited((ids) => new Set(ids).add(id))
      void queryClient.invalidateQueries({ queryKey: queryKeys.terminals(name) })
    },
    [queryClient, name],
  )

  return (
    <div className="flex h-full min-h-0 gap-4 p-6">
      <aside className="flex w-64 shrink-0 flex-col gap-2 overflow-y-auto">
        <ErrorText error={terminals.error} />
        {jobs.length > 0 && (
          <ul aria-label="Terminals" className="flex flex-col gap-1.5">
            {jobs.map((job) => (
              <li key={job.id}>
                <JobTile
                  job={job}
                  selected={job.id === selected}
                  exited={exited.has(job.id)}
                  onSelect={() => select(job.id)}
                />
              </li>
            ))}
          </ul>
        )}
        {sandbox.data && !running && <EmptySlot reason={`Sandbox ${status}`} />}
        {running && (
          <div aria-label="Start terminal" role="group" className="flex flex-col gap-2 pt-2">
            <div className="grid grid-cols-3 gap-1.5">
              {QUICK_START.map((quick) => (
                <Button
                  key={quick}
                  variant="outline"
                  size="sm"
                  className="font-mono"
                  disabled={start.isPending}
                  onClick={() => startJob(quick)}
                >
                  {start.isPending && start.variables?.command === quick ? (
                    <Loader2 className="animate-spin" />
                  ) : (
                    <conceptIcons.terminal className={color.text} />
                  )}
                  {quick}
                </Button>
              ))}
            </div>
            <form onSubmit={submit} className="relative">
              <conceptIcons.terminal
                aria-hidden
                className={cn('pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2', color.text)}
              />
              <Input
                aria-label="Command"
                placeholder="claude --resume"
                value={command}
                disabled={start.isPending}
                onChange={(event) => setCommand(event.target.value)}
                className="h-8 pl-8 font-mono text-xs md:text-xs"
              />
            </form>
            <IconChip icon={conceptIcons.repo} concept="repo" title="Working directory" className="self-start">
              {dir}
            </IconChip>
            <ErrorText error={start.error} />
          </div>
        )}
      </aside>

      <section
        aria-label="Terminal"
        className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-xl border border-border bg-white shadow-sm dark:bg-zinc-900/60"
      >
        {panes.map((job) => (
          <TerminalPane key={job.id} sandbox={name} job={job} visible={job.id === selected} onExited={onExited} />
        ))}
        {!selectedJob && (
          <div className="grid flex-1 place-items-center bg-zinc-50 dark:bg-zinc-950">
            <conceptIcons.terminal aria-hidden className="size-10 text-zinc-300 dark:text-zinc-700" />
          </div>
        )}
      </section>
    </div>
  )
}

type JobTileProps = {
  job: Terminal
  selected: boolean
  exited: boolean
  onSelect: () => void
}

function JobTile({ job, selected, exited, onSelect }: JobTileProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-current={selected ? 'true' : undefined}
      title={job.id}
      className={cn(
        'flex w-full items-center gap-2 rounded-md border px-2.5 py-2 text-left font-mono text-sm transition-colors',
        // Selected: the terminal tint; the rest stay dim zinc.
        selected
          ? [color.bg, color.border, 'text-foreground']
          : 'border-border bg-white text-zinc-700 hover:bg-zinc-50 dark:bg-zinc-900/60 dark:text-zinc-300 dark:hover:bg-zinc-800/60',
      )}
    >
      <JobIcon job={job} className={cn('size-4 shrink-0', selected ? color.text : 'text-muted-foreground')} />
      <span className="min-w-0 flex-1 truncate">{job.command.join(' ')}</span>
      {exited && <ExitedBadge />}
      <JobShape running={job.running} />
    </button>
  )
}

// Where the sandbox's repo is cloned: "github.com/o/n" lives in /home/agent/n.
function repoDir(spec: TaskSpec | undefined) {
  const repo = spec?.repo?.trim() ?? ''
  const last =
    repo
      .replace(/\/+$/, '')
      .split('/')
      .pop()
      ?.replace(/\.git$/, '') ?? ''
  return last ? `${HOME_DIR}/${last}` : HOME_DIR
}

// The agent job first; the rest keep the server's order.
function agentFirst(jobs: Terminal[]) {
  return [...jobs.filter((job) => job.id === AGENT_JOB), ...jobs.filter((job) => job.id !== AGENT_JOB)]
}

function unique(ids: string[]) {
  return [...new Set(ids)]
}

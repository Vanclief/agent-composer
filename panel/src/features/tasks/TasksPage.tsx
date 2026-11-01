// /tasks: every task as one row with its status, age and quick actions.
import { useState } from 'react'
import { Clock, Plus } from 'lucide-react'
import { Link } from 'react-router'

import type { Task } from '@/api/client'
import { useTasks } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { CreateTaskDialog } from '@/features/create-task/CreateTaskDialog'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { formatMemory, shortAge } from './format'
import { TaskActions, useTaskActions } from './TaskActions'

export function TasksPage() {
  usePageTitle('Tasks')
  const tasks = useTasks()
  const [creating, setCreating] = useState(false)
  const TaskIcon = conceptIcons.task

  return (
    <div className="flex flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <span
          title="Tasks"
          className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
        >
          <TaskIcon className={cn('size-4', conceptColors.task.text)} />
          {tasks.data?.length ?? '–'}
        </span>
        <Button onClick={() => setCreating(true)} className={conceptColors.task.solid}>
          <Plus /> New task
        </Button>
      </div>
      <ErrorText error={tasks.error} />
      {tasks.isPending && (
        <div className="flex flex-col gap-2" aria-label="Loading tasks">
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
        </div>
      )}
      {tasks.data?.length === 0 && <EmptySlot onAdd={() => setCreating(true)} label="New task" className="h-24 w-48" />}
      {Boolean(tasks.data?.length) && (
        <ul className="flex flex-col divide-y divide-border overflow-hidden rounded-xl border border-border bg-white shadow-sm dark:bg-zinc-900/60">
          {tasks.data?.map((task) => (
            <TaskRow key={task.id} task={task} />
          ))}
        </ul>
      )}
      <CreateTaskDialog open={creating} onOpenChange={setCreating} />
    </div>
  )
}

// The whole row opens the task; status, age, actions and errors sit above
// the link so their tooltips and buttons work.
function TaskRow({ task }: { task: Task }) {
  const actions = useTaskActions()
  const TaskIcon = conceptIcons.task
  const { cpus, memory } = task.spec

  return (
    <li className="relative flex flex-col gap-2 px-4 py-2.5 text-sm transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-800/40">
      <div className="flex items-center gap-3">
        <TaskIcon className={cn('size-4 shrink-0', conceptColors.task.text)} />
        <Link
          to={`/tasks/${encodeURIComponent(task.name)}`}
          className="truncate font-medium after:absolute after:inset-0 after:content-['']"
        >
          {task.title}
        </Link>
        <IconChip icon={conceptIcons.channel} concept="channel" title="Name and channel" className="shrink-0">
          {task.name}
        </IconChip>
        <div className="ml-auto hidden shrink-0 items-center gap-1.5 lg:flex">
          {cpus ? (
            <IconChip icon={conceptIcons.cpu} concept="cpu" title="CPUs">
              {cpus} CPU
            </IconChip>
          ) : null}
          {memory ? (
            <IconChip icon={conceptIcons.memory} concept="memory" title="Memory">
              {formatMemory(memory)}
            </IconChip>
          ) : null}
        </div>
        <div className="relative z-10 ml-auto flex shrink-0 items-center gap-3 lg:ml-3">
          <div className="flex w-24 justify-start">
            <StatusPill status={task.status} detail={task.status_error} />
          </div>
          <IconChip
            icon={Clock}
            title={`Created ${new Date(task.created_at).toLocaleString()}`}
            className="w-14 justify-start"
          >
            {shortAge(task.created_at)}
          </IconChip>
          <TaskActions task={task} actions={actions} />
        </div>
      </div>
      <ErrorText error={actions.error} className="relative z-10" />
    </li>
  )
}

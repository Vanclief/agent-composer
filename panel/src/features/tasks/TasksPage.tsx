// /tasks: every task as one card: status, title and name, then who works on
// it, who manages it, what it is for and its compute. Open tasks first,
// then running, then done, newest first inside each.
import { useMemo, useState } from 'react'
import { Clock, Plus } from 'lucide-react'
import { Link } from 'react-router'

import { useTasks } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
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

import { shortAge, sortTasks } from './format'
import { AssigneeChip, ForChip, ManagerChip, OwnerChip, SandboxSummary } from './TaskChips'

export function TasksPage() {
  usePageTitle('Tasks')
  const tasks = useTasks()
  const [creating, setCreating] = useState(false)
  const sorted = useMemo(() => sortTasks(tasks.data ?? []), [tasks.data])
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
        <ul aria-label="Tasks" className="flex flex-col gap-2">
          {sorted.map((task) => (
            <TaskCard key={task.id} task={task} />
          ))}
        </ul>
      )}
      <CreateTaskDialog open={creating} onOpenChange={setCreating} />
    </div>
  )
}

// The whole card opens the task; the for link and the tooltips sit above
// the card's link so they work on their own.
function TaskCard({ task }: { task: TaskView }) {
  const TaskIcon = conceptIcons.task

  return (
    <li
      data-task={task.name}
      className={cn(
        'relative flex flex-col gap-2 rounded-xl border bg-white px-4 py-2.5 text-sm shadow-sm transition-colors hover:bg-violet-500/5 dark:bg-zinc-900/60 dark:hover:bg-violet-500/10',
        conceptColors.task.border,
        conceptColors.task.edge,
      )}
    >
      <div className="flex items-center gap-3">
        <div data-testid="task-status" className="flex w-20 shrink-0">
          <StatusPill status={task.status} />
        </div>
        <TaskIcon aria-hidden className={cn('size-4 shrink-0', conceptColors.task.text)} />
        <Link
          to={`/tasks/${encodeURIComponent(task.name)}`}
          className="truncate font-medium after:absolute after:inset-0 after:content-['']"
        >
          {task.title}
        </Link>
        <span title="Name, also the channel" className="truncate font-mono text-xs text-muted-foreground">
          {task.name}
        </span>
        <span className="relative z-10 ml-auto shrink-0">
          <IconChip icon={Clock} title={`Created ${new Date(task.created_at).toLocaleString()}`}>
            {shortAge(task.created_at)}
          </IconChip>
        </span>
      </div>
      <div className="relative z-10 flex w-fit flex-wrap items-center gap-1.5 pl-[5.75rem]">
        <AssigneeChip task={task} />
        <ManagerChip task={task} />
        <OwnerChip task={task} />
        <ForChip task={task} />
        <SandboxSummary task={task} />
      </div>
    </li>
  )
}

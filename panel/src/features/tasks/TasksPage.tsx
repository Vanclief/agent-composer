// /tasks: every task as one flat row (TaskRow): status, title and name,
// then who works on it, who manages it, what it is for and its compute. Sorted by status
// light: attention (also every task waiting on me, attention.tsx), lit
// (review, then running), idle (open), off (done), newest first inside
// each; a done row recedes.
import { useMemo, useState } from 'react'
import { Plus } from 'lucide-react'

import { useTasks } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { StatusLegend } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { CreateTaskDialog } from '@/features/create-task/CreateTaskDialog'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { useAttention } from './attention'
import { sortTasks } from './format'
import { TaskRow } from './TaskRow'

export function TasksPage() {
  usePageTitle('Tasks')
  const tasks = useTasks()
  const [creating, setCreating] = useState(false)
  const attention = useAttention()
  const sorted = useMemo(
    () => sortTasks(tasks.data ?? [], (t) => attention(t).mine.length > 0),
    [tasks.data, attention],
  )
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
        <StatusLegend className="mr-auto ml-4" />
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
        <ul aria-label="Tasks" className="flex flex-col divide-y divide-border border-y border-border">
          {sorted.map((task) => (
            <TaskRow key={task.id} task={task} attention={attention(task)} />
          ))}
        </ul>
      )}
      <CreateTaskDialog open={creating} onOpenChange={setCreating} />
    </div>
  )
}

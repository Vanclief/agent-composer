// PLACEHOLDER owned by the tasks feature: replace freely, keep the export.
import { useState } from 'react'
import { Plus } from 'lucide-react'
import { Link } from 'react-router'

import { useTasks } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { CreateTaskDialog } from '@/features/create-task/CreateTaskDialog'
import { usePageTitle } from '@/shell/page-title'

export function TasksPage() {
  usePageTitle('Tasks')
  const tasks = useTasks()
  const [creating, setCreating] = useState(false)
  const TaskIcon = conceptIcons.task

  return (
    <div className="flex flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold tracking-tight">TasksPage</h2>
        <Button onClick={() => setCreating(true)}>
          <Plus /> New task
        </Button>
      </div>
      <ErrorText error={tasks.error} />
      <ul className="flex flex-col divide-y divide-zinc-200 rounded-md border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800">
        {tasks.data?.map((task) => (
          <li key={task.id} className="flex items-center gap-3 px-3 py-2 text-sm">
            <TaskIcon className="size-4 text-zinc-500" />
            <Link to={`/tasks/${encodeURIComponent(task.name)}`} className="font-medium hover:underline">
              {task.title}
            </Link>
            <IconChip icon={conceptIcons.channel} title="Name">
              {task.name}
            </IconChip>
            <StatusPill status={task.status} detail={task.status_error} className="ml-auto" />
          </li>
        ))}
      </ul>
      <CreateTaskDialog open={creating} onOpenChange={setCreating} />
    </div>
  )
}

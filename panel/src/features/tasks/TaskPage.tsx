// /tasks/:name/*: the task header with its actions, then the terminals,
// messages and spec tabs rendered through <Outlet/>.
import { ArrowLeft } from 'lucide-react'
import { Link, NavLink, Outlet, useNavigate, useParams } from 'react-router'

import { useTask } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { TaskActions, useTaskActions } from './TaskActions'

const TABS = [
  { to: 'terminals', label: 'Terminals', icon: conceptIcons.terminal },
  { to: 'messages', label: 'Messages', icon: conceptIcons.message },
  { to: 'spec', label: 'Spec', icon: conceptIcons.raw },
]

export function TaskPage() {
  const { name = '' } = useParams()
  const task = useTask(name)
  const actions = useTaskActions()
  const navigate = useNavigate()
  usePageTitle(task.data?.title ?? name)
  const TaskIcon = conceptIcons.task

  if (!task.data) {
    if (task.error) {
      return (
        <div className="flex flex-col items-start gap-3 p-6">
          <ErrorText error={task.error} />
          <Link
            to="/tasks"
            className="inline-flex items-center gap-1.5 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
          >
            <ArrowLeft className="size-4" /> Tasks
          </Link>
        </div>
      )
    }
    return (
      <div className="flex flex-col gap-4 p-6" aria-label="Loading task">
        <Skeleton className="h-10 w-80" />
        <Skeleton className="h-8 w-72" />
      </div>
    )
  }

  const data = task.data
  return (
    <div className="flex h-full flex-col">
      <div data-testid="task-header" className="flex items-center gap-3 px-6 pt-6">
        <span className="grid size-10 shrink-0 place-items-center rounded-lg border border-zinc-200 text-zinc-500 dark:border-zinc-800">
          <TaskIcon className="size-5" />
        </span>
        <h2 className="truncate text-2xl font-semibold tracking-tight">{data.title}</h2>
        <IconChip icon={conceptIcons.channel} title="Name and channel" className="shrink-0">
          {data.name}
        </IconChip>
        <StatusPill status={data.status} detail={data.status_error} />
        <TaskActions task={data} actions={actions} onDeleted={() => navigate('/tasks')} className="ml-auto shrink-0" />
      </div>
      <ErrorText error={actions.error ?? task.error} className="mx-6 mt-4" />
      <nav aria-label="Task" className="mt-4 flex gap-1 border-b border-zinc-200 px-6 text-sm dark:border-zinc-800">
        {TABS.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                '-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2',
                isActive
                  ? 'border-zinc-900 font-medium dark:border-white'
                  : 'border-transparent text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100',
              )
            }
          >
            <Icon className="size-4" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="min-h-0 flex-1">
        <Outlet />
      </div>
    </div>
  )
}

// /tasks/:name/*: the task header with its actions, then the brief, compute
// and messages tabs rendered through <Outlet/>.
import { ArrowLeft } from 'lucide-react'
import { Link, NavLink, Outlet, useNavigate, useParams } from 'react-router'

import { useTask } from '@/api/hooks'
import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { TaskActions, useTaskActions } from './TaskActions'
import { AssigneeChip, ForChip, ManagerChip, OwnerChip } from './TaskChips'

const TABS: { to: string; label: string; concept: Concept }[] = [
  { to: 'brief', label: 'Brief', concept: 'brief' },
  { to: 'compute', label: 'Compute', concept: 'sandbox' },
  { to: 'messages', label: 'Messages', concept: 'message' },
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
      <div data-testid="task-header" className="flex items-start gap-3 px-6 pt-6">
        <span
          className={cn(
            'grid size-12 shrink-0 place-items-center rounded-xl border',
            conceptColors.task.bg,
            conceptColors.task.border,
            conceptColors.task.text,
          )}
        >
          <TaskIcon className="size-6" />
        </span>
        <div className="flex min-w-0 flex-col gap-1.5">
          <div className="flex min-w-0 items-center gap-3">
            <StatusPill status={data.status} />
            <h2 className="truncate text-2xl font-semibold tracking-tight">{data.title}</h2>
            <IconChip icon={conceptIcons.channel} concept="channel" title="Name and channel" className="shrink-0">
              {data.name}
            </IconChip>
          </div>
          <div data-testid="task-chips" className="flex flex-wrap items-center gap-1.5">
            <AssigneeChip task={data} />
            <ManagerChip task={data} />
            <OwnerChip task={data} />
            <ForChip task={data} />
            {data.notify ? (
              <span data-testid="notify-chip" className="inline-flex">
                <IconChip icon={conceptIcons.notify} concept="notify" title="Notify: toast and sound on start and done">
                  notify
                </IconChip>
              </span>
            ) : null}
          </div>
        </div>
        <TaskActions task={data} actions={actions} onDeleted={() => navigate('/tasks')} className="ml-auto shrink-0" />
      </div>
      <ErrorText error={actions.error ?? task.error} className="mx-6 mt-4" />
      <nav aria-label="Task" className="mt-4 flex gap-1 border-b border-border px-6 text-sm">
        {TABS.map(({ to, label, concept }) => {
          const Icon = conceptIcons[concept]
          return (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                cn(
                  '-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2',
                  isActive
                    ? 'border-zinc-900 font-medium dark:border-white'
                    : 'border-transparent text-muted-foreground hover:text-foreground',
                )
              }
            >
              {({ isActive }) => (
                <>
                  {/* The open tab's icon takes its concept color; the rest stay dim. */}
                  <Icon className={cn('size-4', isActive && conceptColors[concept].text)} />
                  {label}
                </>
              )}
            </NavLink>
          )
        })}
      </nav>
      <div className="min-h-0 flex-1">
        <Outlet />
      </div>
    </div>
  )
}

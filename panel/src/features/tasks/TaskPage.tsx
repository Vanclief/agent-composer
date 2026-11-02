// /tasks/:name/*: the task's facts in a rail (TaskRail) and, in the center,
// one box with "Compute | Messages" at its top switching what <Outlet/>
// renders. The URL holds the choice: /compute is the sandboxes, /messages
// and the bare /tasks/:name are the channel. Wide screens (>= 1200px) put
// the rail on the right, sticky; narrow ones stack it above the box.
import { ArrowLeft } from 'lucide-react'
import { Link, Outlet, useMatch, useNavigate, useParams } from 'react-router'

import { useTask } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { useTaskActions } from './TaskActions'
import { TaskRail } from './TaskRail'

export function TaskPage() {
  const { name = '' } = useParams()
  const task = useTask(name)
  const actions = useTaskActions()
  const navigate = useNavigate()
  usePageTitle(task.data?.title ?? name)

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

  return (
    <div className="h-full min-h-0 overflow-y-auto min-[1200px]:overflow-hidden">
      <div className="flex flex-col gap-4 p-4 min-[1200px]:h-full min-[1200px]:flex-row min-[1200px]:gap-6 min-[1200px]:p-6">
        <TaskRail
          task={task.data}
          actions={actions}
          error={actions.error ?? task.error}
          onDeleted={() => navigate('/tasks')}
          className={cn(
            // Narrow: a header card above the box.
            'rounded-xl border border-border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
            // Wide: a flat rail on the right that scrolls on its own.
            'min-[1200px]:order-2 min-[1200px]:w-[360px] min-[1200px]:shrink-0 min-[1200px]:overflow-y-auto',
            'min-[1200px]:rounded-none min-[1200px]:border-0 min-[1200px]:bg-transparent min-[1200px]:p-0 min-[1200px]:shadow-none',
            'min-[1200px]:dark:bg-transparent',
          )}
        />
        <TaskBox task={task.data} />
      </div>
    </div>
  )
}

// The center box: the segmented control, then the chosen view.
function TaskBox({ task }: { task: TaskView }) {
  const compute = useMatch('/tasks/:name/compute') !== null
  return (
    <section
      aria-label="Task work"
      className="flex h-[80vh] min-h-[32rem] min-w-0 shrink-0 flex-col overflow-hidden rounded-xl border border-border bg-white shadow-sm min-[1200px]:order-1 min-[1200px]:h-auto min-[1200px]:min-h-0 min-[1200px]:flex-1 dark:bg-zinc-900/60"
    >
      <div className="flex shrink-0 items-center border-b border-border px-3 py-2">
        <nav
          aria-label="Task view"
          className="inline-flex items-center gap-0.5 rounded-lg border border-border bg-zinc-100 p-0.5 text-sm dark:bg-zinc-950/60"
        >
          <Segment to="compute" label="Compute" concept="sandbox" active={compute} count={task.sandboxes.length} />
          <Segment to="messages" label="Messages" concept="message" active={!compute} />
        </nav>
      </div>
      <div className="flex min-h-0 flex-1 flex-col">
        <Outlet />
      </div>
    </section>
  )
}

function Segment({
  to,
  label,
  concept,
  active,
  count,
}: {
  to: string
  label: string
  concept: Concept
  active: boolean
  count?: number
}) {
  const Icon = conceptIcons[concept]
  return (
    <Link
      to={to}
      aria-current={active ? 'page' : undefined}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 transition-colors',
        active
          ? 'bg-white font-medium text-foreground shadow-sm dark:bg-zinc-800'
          : 'text-muted-foreground hover:text-foreground',
      )}
    >
      {/* The chosen view's icon takes its concept color; the other stays dim. */}
      <Icon aria-hidden className={cn('size-4', active && conceptColors[concept].text)} />
      {label}
      {count !== undefined && (
        <span
          title={`${count} ${count === 1 ? 'sandbox' : 'sandboxes'}`}
          className="text-xs text-muted-foreground tabular-nums"
        >
          {count}
        </span>
      )}
    </Link>
  )
}

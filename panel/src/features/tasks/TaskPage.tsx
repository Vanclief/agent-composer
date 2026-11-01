// PLACEHOLDER owned by the tasks feature: replace freely, but keep the tab
// routes (terminals, messages, spec) and the <Outlet/> that renders them.
import { NavLink, Outlet, useParams } from 'react-router'

import { useTask } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { StatusPill } from '@/components/status-pill'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

const TABS = [
  { to: 'terminals', label: 'Terminals', icon: conceptIcons.terminal },
  { to: 'messages', label: 'Messages', icon: conceptIcons.message },
  { to: 'spec', label: 'Spec', icon: conceptIcons.raw },
]

export function TaskPage() {
  const { name = '' } = useParams()
  const task = useTask(name)
  usePageTitle(task.data?.title ?? name)

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 px-6 pt-6">
        <h2 className="text-2xl font-semibold tracking-tight">TaskPage</h2>
        {task.data && <StatusPill status={task.data.status} detail={task.data.status_error} />}
      </div>
      <ErrorText error={task.error} className="mx-6 mt-4" />
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

// Sidebar nav, top bar with the page title, and the routed page.
import { useState } from 'react'
import { Hexagon, type LucideIcon } from 'lucide-react'
import { NavLink, Outlet } from 'react-router'

import { conceptIcons } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

import { PageTitleContext } from './page-title'
import { ThemeToggle } from './theme-toggle'

const NAV: { to: string; label: string; icon: LucideIcon }[] = [
  { to: '/tasks', label: 'Tasks', icon: conceptIcons.task },
  { to: '/messages', label: 'Messages', icon: conceptIcons.message },
]

export function Layout() {
  const [title, setTitle] = useState('')

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="flex w-56 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900/60">
        <div className="flex h-[54px] items-center gap-2 border-b border-zinc-200 px-4 font-semibold tracking-tight dark:border-zinc-800">
          <span className="grid size-6 place-items-center rounded bg-zinc-900 text-white dark:bg-white dark:text-zinc-900">
            <Hexagon className="size-4" />
          </span>
          agc
        </div>
        <nav aria-label="Main" className="flex-1 overflow-y-auto p-2">
          <ul className="flex flex-col gap-0.5">
            {NAV.map(({ to, label, icon: Icon }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors',
                      isActive
                        ? 'bg-zinc-200 font-medium text-zinc-900 dark:bg-zinc-800 dark:text-white'
                        : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
                    )
                  }
                >
                  <Icon className="size-4" />
                  {label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
        <div className="flex justify-end p-2">
          <ThemeToggle />
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-[54px] shrink-0 items-center border-b border-zinc-200 px-6 text-sm font-medium dark:border-zinc-800">
          <h1 data-testid="page-title" className="truncate">
            {title}
          </h1>
        </header>
        <div className="min-h-0 flex-1 overflow-auto">
          <PageTitleContext.Provider value={setTitle}>
            <Outlet />
          </PageTitleContext.Provider>
        </div>
      </main>
    </div>
  )
}

// Sidebar nav, top bar with the page title, and the routed page.
import { useCallback, useState } from 'react'
import { Hexagon } from 'lucide-react'
import { NavLink, Outlet } from 'react-router'

import { useLiveEvents, type LiveEvent } from '@/api/events'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ChannelRows } from '@/features/messages/ChannelRows'
import { useMessageNotifications } from '@/features/messages/notifications'
import { TalkBadges } from '@/features/messages/QuestionBadge'
import { useUnread } from '@/features/messages/unread'
import { cn } from '@/lib/utils'

import { MODES, ModeSwitch, useMode } from './mode'
import { PageTitleContext } from './page-title'
import { ThemeToggle } from './theme-toggle'
import { LiveDot, MeChip, SoundToggle } from './top-bar'
import { useTaskNotifications } from './notifications'

export function Layout() {
  const [title, setTitle] = useState('')
  const mode = useMode()
  const taskNotifications = useTaskNotifications()
  const messageNotifications = useMessageNotifications()
  useLiveEvents(
    useCallback(
      (event: LiveEvent) => {
        taskNotifications(event)
        messageNotifications(event)
      },
      [taskNotifications, messageNotifications],
    ),
  )
  const unread = useUnread()

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-white dark:bg-zinc-900/60">
        <div className="flex h-[54px] items-center gap-2 border-b border-border px-4 font-semibold tracking-tight">
          <span className="grid size-6 place-items-center rounded bg-zinc-900 text-white dark:bg-white dark:text-zinc-900">
            <Hexagon className="size-4" />
          </span>
          agc
        </div>
        <div className="border-b border-border p-2">
          <ModeSwitch mode={mode} badges={{ talk: <TalkBadges count={unread.total} mention={unread.mention} /> }} />
        </div>
        <nav aria-label="Main" className="flex-1 overflow-y-auto p-2">
          <ul className="flex flex-col gap-0.5">
            {MODES[mode].nav.map(({ to, label, concept }) => {
              const Icon = conceptIcons[concept]
              return (
                <li key={to}>
                  <NavLink
                    to={to}
                    className={({ isActive }) =>
                      cn(
                        'flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors',
                        isActive
                          ? 'bg-zinc-100 font-medium text-zinc-900 dark:bg-zinc-800 dark:text-white'
                          : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
                      )
                    }
                  >
                    <Icon className={cn('size-4', conceptColors[concept].text)} />
                    {label}
                  </NavLink>
                  {mode === 'talk' && to === '/messages' && <ChannelRows />}
                </li>
              )
            })}
          </ul>
        </nav>
        <div className="flex justify-end p-2">
          <ThemeToggle />
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-[54px] shrink-0 items-center border-b border-border px-6 text-sm font-medium">
          <h1 data-testid="page-title" className="truncate">
            {title}
          </h1>
          <div className="ml-auto flex shrink-0 items-center gap-1 pl-4">
            <LiveDot />
            <SoundToggle />
            <MeChip />
          </div>
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

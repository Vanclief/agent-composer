// Sidebar (Think, Work, Talk groups), top bar with the page title, and the routed page.
import { useCallback, useState } from 'react'
import { Hexagon } from 'lucide-react'
import { Outlet, useLocation } from 'react-router'

import { useLiveEvents, type LiveEvent } from '@/api/events'
import { followFromEvent, useFollow } from '@/features/messages/follow'
import { useMyName } from '@/features/messages/identity'
import { useMessageNotifications } from '@/features/messages/notifications'
import { TalkBadges } from '@/features/messages/QuestionBadge'

import { AREAS, areaOf } from './areas'
import { PageTitleContext } from './page-title'
import { Sidebar } from './sidebar'
import { ThemeToggle } from './theme-toggle'
import { LiveDot, MeChip, SoundToggle } from './top-bar'
import { useTaskNotifications } from './notifications'

export function Layout() {
  const [title, setTitle] = useState('')
  const area = areaOf(useLocation().pathname)
  const taskNotifications = useTaskNotifications()
  const messageNotifications = useMessageNotifications()
  const me = useMyName()
  useLiveEvents(
    useCallback(
      (event: LiveEvent) => {
        taskNotifications(event)
        messageNotifications(event)
        followFromEvent(event, me)
      },
      [taskNotifications, messageNotifications, me],
    ),
  )
  // Unread only across followed channels, my inbox, my DMs and mentions.
  const unread = useFollow()

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-white dark:bg-zinc-900/60">
        <div className="flex h-[54px] items-center gap-2 border-b border-border px-4 font-semibold tracking-tight">
          <span className="grid size-6 place-items-center rounded bg-zinc-900 text-white dark:bg-white dark:text-zinc-900">
            <Hexagon className="size-4" />
          </span>
          agc
        </div>
        <Sidebar badges={{ talk: <TalkBadges count={unread.total} mention={unread.mention} /> }} />
        <div className="flex justify-end p-2">
          <ThemeToggle />
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-[54px] shrink-0 items-center border-b border-border px-6 text-sm font-medium">
          {area && (
            <span data-testid="page-area" className="mr-1.5 shrink-0 text-muted-foreground">
              {AREAS[area].label} /
            </span>
          )}
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

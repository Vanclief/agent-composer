// Sidebar (Think, Work, Talk groups), top bar with the page title, and the routed page.
import { useCallback, useState } from 'react'
import { ChevronRight, Hexagon } from 'lucide-react'
import { Link, Outlet, useLocation } from 'react-router'

import { useLiveEvents, type LiveEvent } from '@/api/events'
import { followFromEvent, useFollow } from '@/features/messages/follow'
import { useMyName } from '@/features/messages/identity'
import { useMessageNotifications } from '@/features/messages/notifications'
import { TalkBadges } from '@/features/messages/QuestionBadge'
import { TasksBadge, useMyTaskCount } from '@/features/tasks/attention'

import { AREAS, areaOf } from './areas'
import { PageTitleContext, type PageTitle } from './page-title'
import { Sidebar } from './sidebar'
import { LiveDot, MeChip, SoundToggle } from './top-bar'
import { useTaskNotifications } from './notifications'

export function Layout() {
  const [title, setTitle] = useState<PageTitle>({ title: '' })
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
  // Tasks waiting on me (accept, answer, restart): the Tasks and Work badges.
  const mine = useMyTaskCount()

  return (
    <div className="flex h-screen overflow-hidden">
      <aside
        data-testid="sidebar"
        className="flex w-56 shrink-0 flex-col border-r border-border bg-white dark:bg-zinc-900/60"
      >
        <div className="flex h-[54px] items-center gap-2 border-b border-border px-4 font-semibold tracking-tight">
          <span className="grid size-6 place-items-center rounded bg-zinc-900 text-white dark:bg-white dark:text-zinc-900">
            <Hexagon className="size-4" />
          </span>
          agc
        </div>
        <Sidebar
          badges={{
            work: <TasksBadge count={mine} testid="work-badge" />,
            talk: <TalkBadges count={unread.total} mention={unread.mention} />,
          }}
          pageBadges={{ '/tasks': <TasksBadge count={mine} /> }}
        />
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-[54px] shrink-0 items-center border-b border-border px-6 text-sm font-medium">
          {area && (
            <>
              <Link to={AREAS[area].home} data-testid="page-area" className="shrink-0">
                {AREAS[area].label}
              </Link>
              <ChevronRight aria-hidden="true" data-testid="page-area-chevron" className="mx-1 size-4 shrink-0" />
            </>
          )}
          <h1 data-testid="page-title" className="truncate">
            {title.title}
            {title.title && title.id && ' '}
            {title.id && <span className="page-id">{title.id}</span>}
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

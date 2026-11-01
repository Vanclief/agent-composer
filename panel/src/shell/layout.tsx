// Sidebar nav, top bar with the page title, and the routed page.
import { useState } from 'react'
import { Hexagon, Lock } from 'lucide-react'
import { NavLink, Outlet } from 'react-router'

import { useChannels } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { MODES, ModeSwitch, useMode } from './mode'
import { PageTitleContext } from './page-title'
import { ThemeToggle } from './theme-toggle'

export function Layout() {
  const [title, setTitle] = useState('')
  const mode = useMode()

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
          <ModeSwitch mode={mode} />
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

// Talk mode: one row per channel under Channels, Slack style. The open
// channel takes the message tint; the count chip is its message count.
function ChannelRows() {
  const channels = useChannels()

  if (channels.isPending) {
    return (
      <ul data-testid="channel-rows-loading" className="mt-0.5 flex flex-col gap-0.5 pl-6">
        <li>
          <Skeleton className="h-7" />
        </li>
        <li>
          <Skeleton className="h-7" />
        </li>
      </ul>
    )
  }
  if (channels.error) {
    return <ErrorText error={channels.error} className="mt-0.5 ml-6 px-2 py-1.5 text-xs" />
  }
  if (channels.data.length === 0) {
    return <p className="mt-0.5 py-1.5 pr-2 pl-8 text-sm text-zinc-500 dark:text-zinc-500">No channels yet</p>
  }
  return (
    <ul className="mt-0.5 flex flex-col gap-0.5 pl-6">
      {channels.data.map((c) => {
        const name = c.channel.replace(/^#/, '')
        return (
          <li key={c.channel}>
            <NavLink
              to={`/messages/${encodeURIComponent(name)}`}
              title={c.read_only ? 'Read only' : undefined}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-sm transition-colors',
                  isActive
                    ? cn('font-medium', conceptColors.message.selected)
                    : 'border-transparent text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
                )
              }
            >
              {c.read_only && <Lock aria-label="Read only" className="size-3.5 shrink-0" />}
              <span className="truncate font-mono">#{name}</span>
              <span
                data-testid="channel-count"
                className="ml-auto rounded border border-border px-1.5 font-mono text-xs text-muted-foreground"
              >
                {c.count}
              </span>
            </NavLink>
          </li>
        )
      })}
    </ul>
  )
}

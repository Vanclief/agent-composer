// The two top-level modes: Think (evidence, beliefs, decisions, rules) and
// Work (tasks, messages). The active mode comes from the route; the last
// one used is kept in localStorage so / opens it again.
import { useEffect } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Link, Navigate, useLocation } from 'react-router'

import { conceptIcons } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

export type Mode = 'think' | 'work'

export const MODE_KEY = 'agc.panel.mode'

type NavItem = { to: string; label: string; icon: LucideIcon }

// Each mode's pages, first page first. The first page is where the mode
// switch and / go.
export const MODES: Record<Mode, { label: string; icon: LucideIcon; nav: NavItem[] }> = {
  think: {
    label: 'Think',
    icon: conceptIcons.think,
    nav: [
      { to: '/think/beliefs', label: 'Beliefs', icon: conceptIcons.belief },
      { to: '/think/decisions', label: 'Decisions', icon: conceptIcons.decision },
      { to: '/think/rules', label: 'Rules', icon: conceptIcons.rule },
      { to: '/think/evidence', label: 'Evidence', icon: conceptIcons.evidence },
    ],
  },
  work: {
    label: 'Work',
    icon: conceptIcons.work,
    nav: [
      { to: '/tasks', label: 'Tasks', icon: conceptIcons.task },
      { to: '/messages', label: 'Messages', icon: conceptIcons.message },
    ],
  },
}

function modeOf(pathname: string): Mode {
  if (pathname === '/think' || pathname.startsWith('/think/')) {
    return 'think'
  }
  return 'work'
}

function storedMode(): Mode {
  try {
    return localStorage.getItem(MODE_KEY) === 'think' ? 'think' : 'work'
  } catch {
    return 'work'
  }
}

// The mode of the current route, remembered as the last used.
export function useMode(): Mode {
  const { pathname } = useLocation()
  const mode = modeOf(pathname)
  useEffect(() => {
    try {
      localStorage.setItem(MODE_KEY, mode)
    } catch {
      // Storage disabled: / falls back to Work.
    }
  }, [mode])
  return mode
}

// / opens the first page of the last used mode.
export function HomeRedirect() {
  return <Navigate to={MODES[storedMode()].nav[0].to} replace />
}

// Segmented control: two icons with one-word labels. It sits at the top of
// the sidebar and never moves.
export function ModeSwitch({ mode }: { mode: Mode }) {
  return (
    <nav aria-label="Mode" className="grid grid-cols-2 gap-0.5 rounded-md bg-zinc-200/70 p-0.5 dark:bg-zinc-800/70">
      {(Object.keys(MODES) as Mode[]).map((m) => {
        const { label, icon: Icon, nav } = MODES[m]
        const active = m === mode
        return (
          <Link
            key={m}
            to={nav[0].to}
            aria-current={active ? 'page' : undefined}
            data-active={active}
            className={cn(
              'flex items-center justify-center gap-1.5 rounded px-2 py-1 text-sm transition-colors',
              active
                ? 'bg-white font-medium text-zinc-900 shadow-sm dark:bg-zinc-950 dark:text-white'
                : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100',
            )}
          >
            <Icon aria-hidden className="size-4" />
            {label}
          </Link>
        )
      })}
    </nav>
  )
}

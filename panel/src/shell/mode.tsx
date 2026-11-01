// The three top-level modes: Think (evidence, beliefs, decisions, rules),
// Work (tasks) and Talk (channels). The active mode comes from the route;
// the last one used is kept in localStorage so / opens it again.
import { useEffect } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Link, Navigate, useLocation } from 'react-router'

import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

export type Mode = 'think' | 'work' | 'talk'

export const MODE_KEY = 'agc.panel.mode'

// The layout draws each item with its concept's icon and color.
type NavItem = { to: string; label: string; concept: Concept }

// Each mode's pages, first page first. The first page is where the mode
// switch and / go. concept colors the mode's row in the switch.
export const MODES: Record<Mode, { label: string; icon: LucideIcon; concept: Concept; nav: NavItem[] }> = {
  think: {
    label: 'Think',
    icon: conceptIcons.think,
    concept: 'think',
    nav: [
      { to: '/think/beliefs', label: 'Beliefs', concept: 'belief' },
      { to: '/think/decisions', label: 'Decisions', concept: 'decision' },
      { to: '/think/rules', label: 'Rules', concept: 'rule' },
      { to: '/think/actions', label: 'Actions', concept: 'action' },
      { to: '/think/evidence', label: 'Evidence', concept: 'evidence' },
    ],
  },
  work: {
    label: 'Work',
    icon: conceptIcons.work,
    concept: 'task',
    nav: [
      { to: '/tasks', label: 'Tasks', concept: 'task' },
    ],
  },
  talk: {
    label: 'Talk',
    icon: conceptIcons.message,
    concept: 'message',
    nav: [
      { to: '/messages', label: 'Channels', concept: 'message' },
    ],
  },
}

function modeOf(pathname: string): Mode {
  if (pathname === '/think' || pathname.startsWith('/think/')) {
    return 'think'
  }
  if (pathname === '/messages' || pathname.startsWith('/messages/')) {
    return 'talk'
  }
  return 'work'
}

function storedMode(): Mode {
  try {
    const stored = localStorage.getItem(MODE_KEY)
    return (Object.keys(MODES) as (string | null)[]).includes(stored) ? (stored as Mode) : 'work'
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

// Vertical list, one row per mode: icon and one-word label, the active row
// in its mode's color. It sits at the top of the sidebar and never moves.
export function ModeSwitch({ mode }: { mode: Mode }) {
  return (
    <nav aria-label="Mode" className="flex flex-col gap-0.5">
      {(Object.keys(MODES) as Mode[]).map((m) => {
        const { label, icon: Icon, concept, nav } = MODES[m]
        const color = conceptColors[concept]
        const active = m === mode
        return (
          <Link
            key={m}
            to={nav[0].to}
            aria-current={active ? 'page' : undefined}
            data-active={active}
            className={cn(
              'flex w-full items-center gap-2 rounded-md border px-2 py-1.5 text-left text-sm transition-colors',
              active
                ? cn('font-medium', color.selected)
                : 'border-transparent text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
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

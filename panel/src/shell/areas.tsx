// The three areas: Think (evidence, beliefs, decisions, rules), Work (tasks
// and their compute) and Talk (channels). The sidebar shows all three at
// once as collapsible groups, in this order (rtsdesign rule 12: fixed
// positions); the route only highlights a page and opens its group. Adding
// an area is one entry in AREAS.
import { useCallback, useEffect, useSyncExternalStore } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Navigate } from 'react-router'

import { conceptIcons, type Concept } from '@/components/concept-icons'

export type Area = 'think' | 'work' | 'talk'

// The last page visited, which / opens.
export const LAST_PAGE_KEY = 'agc.panel.last'

const DEFAULT_PAGE = '/tasks'

// The layout draws each item with its concept's icon and color.
type NavItem = { to: string; label: string; concept: Concept }

// Each area's pages. A route belongs to the area one of whose pages shares
// its first path segment (/tasks/review-42 is Work). concept colors the
// area's header.
export const AREAS: Record<Area, { label: string; icon: LucideIcon; concept: Concept; nav: NavItem[] }> = {
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
      { to: '/compute', label: 'Compute', concept: 'sandbox' },
      { to: '/agents', label: 'Agents', concept: 'agent' },
      { to: '/accounts', label: 'Accounts', concept: 'account' },
    ],
  },
  talk: {
    label: 'Talk',
    icon: conceptIcons.message,
    concept: 'message',
    nav: [{ to: '/messages', label: 'Channels', concept: 'message' }],
  },
}

export const AREA_ORDER = Object.keys(AREAS) as Area[]

function firstSegment(path: string) {
  return path.split('/')[1] ?? ''
}

// The area a route belongs to, undefined for / and unknown paths.
export function areaOf(pathname: string): Area | undefined {
  const segment = firstSegment(pathname)
  return AREA_ORDER.find((area) => AREAS[area].nav.some((item) => firstSegment(item.to) === segment))
}

function read(key: string) {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function write(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Storage disabled: the state lasts until the reload.
  }
}

// Remembers path as the last page visited when it belongs to an area.
export function rememberPage(pathname: string, search: string) {
  if (areaOf(pathname)) {
    write(LAST_PAGE_KEY, pathname + search)
  }
}

// / opens the last page visited, else Tasks.
export function HomeRedirect() {
  const stored = read(LAST_PAGE_KEY)
  const to = stored && stored.startsWith('/') && areaOf(stored.split('?')[0]) ? stored : DEFAULT_PAGE
  return <Navigate to={to} replace />
}

// Open or closed per group, in localStorage agc.panel.group.<area>, default
// open. Subscribers re-render when any group changes.
export function groupKey(area: Area) {
  return `agc.panel.group.${area}`
}

const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function setGroupOpen(area: Area, open: boolean) {
  write(groupKey(area), open ? 'open' : 'closed')
  listeners.forEach((listener) => listener())
}

export function useGroupOpen(area: Area): [boolean, (open: boolean) => void] {
  const open = useSyncExternalStore(subscribe, () => read(groupKey(area)) !== 'closed')
  const set = useCallback((next: boolean) => setGroupOpen(area, next), [area])
  return [open, set]
}

// Navigating to a page opens its group if it was closed and remembers the
// page; the other groups stay as they were.
export function useRouteOpensGroup(pathname: string, search: string) {
  useEffect(() => {
    rememberPage(pathname, search)
    const area = areaOf(pathname)
    if (area && read(groupKey(area)) === 'closed') {
      setGroupOpen(area, true)
    }
  }, [pathname, search])
}

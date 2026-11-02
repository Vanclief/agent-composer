// The sidebar's three groups, Think, Work and Talk, always in that order.
// Each is a header row (concept icon in its color, label, badge, chevron)
// that toggles its pages; a collapsed group shows only its header and
// badge. The whole list scrolls as one column; headers do not stick.
import type { ReactNode } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { NavLink, useLocation } from 'react-router'

import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ChannelRows } from '@/features/messages/ChannelRows'
import { cn } from '@/lib/utils'

import { AREA_ORDER, AREAS, areaOf, useGroupOpen, useRouteOpensGroup, type Area } from './areas'

type PageBadges = Partial<Record<string, ReactNode>>

function Group({
  area,
  current,
  badge,
  pageBadges,
}: {
  area: Area
  current: boolean
  badge?: ReactNode
  pageBadges?: PageBadges
}) {
  const [open, setOpen] = useGroupOpen(area)
  const { label, icon: Icon, concept, nav } = AREAS[area]
  const Chevron = open ? ChevronDown : ChevronRight
  const pagesId = `area-pages-${area}`
  return (
    <div role="group" aria-label={label} data-testid={`area-${area}`} data-open={open}>
      <button
        type="button"
        data-testid={`area-header-${area}`}
        aria-expanded={open}
        aria-controls={pagesId}
        data-current={current}
        onClick={() => setOpen(!open)}
        className={cn(
          'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm font-medium transition-colors hover:bg-zinc-100 dark:hover:bg-zinc-800/60',
          current ? 'text-zinc-900 dark:text-white' : 'text-zinc-600 dark:text-zinc-300',
          // The current page is hidden in a collapsed group: its header stands in.
          current && !open && 'bg-zinc-100 dark:bg-zinc-800',
        )}
      >
        <Icon aria-hidden className={cn('size-4 shrink-0', conceptColors[concept].text)} />
        {label}
        <span data-testid={`area-badge-${area}`} className="ml-auto flex">
          {badge}
        </span>
        <Chevron aria-hidden className={cn('size-3.5 shrink-0 text-zinc-400', badge ? '' : 'ml-auto')} />
      </button>
      {open && (
        <ul id={pagesId} className="mt-0.5 flex flex-col gap-0.5 pl-3">
          {nav.map(({ to, label: page, concept: pageConcept }) => {
            const PageIcon = conceptIcons[pageConcept]
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
                  <PageIcon className={cn('size-4', conceptColors[pageConcept].text)} />
                  {page}
                  {pageBadges?.[to]}
                </NavLink>
                {to === '/messages' && <ChannelRows />}
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

// badges puts something at a header's right end (Talk: its unread count,
// Work: the tasks waiting on me); pageBadges does the same on a page link,
// keyed by its path ('/tasks').
export function Sidebar({
  badges,
  pageBadges,
}: {
  badges?: Partial<Record<Area, ReactNode>>
  pageBadges?: PageBadges
}) {
  const { pathname, search } = useLocation()
  useRouteOpensGroup(pathname, search)
  const current = areaOf(pathname)
  return (
    <nav aria-label="Main" className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-2">
      {AREA_ORDER.map((area) => (
        <Group key={area} area={area} current={area === current} badge={badges?.[area]} pageBadges={pageBadges} />
      ))}
    </nav>
  )
}

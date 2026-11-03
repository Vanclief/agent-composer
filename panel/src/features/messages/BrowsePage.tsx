// /messages/browse: every channel as a flat row (rows, not cards), in
// groups: "Questions for you" (an open question there waits for me),
// "Running tasks" (a task not done, its status verbatim), "Done tasks"
// (collapsed until opened) and "Other" (my inbox, DMs, the rest). A row is
// the kind icon, name, task status, last activity, its unread and
// open-question counts and the Follow / Following toggle. The filter box
// narrows by name as you type; each group header counts what matches.
// Opening a row opens the channel, which follows it.
import { useState } from 'react'
import { useQueries } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Search } from 'lucide-react'
import { Link } from 'react-router'

import { queryKeys, useTasks } from '@/api/hooks'
import { msg, type ChannelInfo } from '@/api/msg'
import type { TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { Input } from '@/components/ui/input'
import { StatusPill } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { channelPath } from './follow'
import { FollowToggle } from './FollowToggle'
import { useMyName } from './identity'
import { RowBadges } from './QuestionBadge'
import { useOpenQuestions } from './questions'
import { isDMOf, useUnread } from './unread'

type Kind = 'inbox' | 'dm' | 'task' | 'other'

const KIND: Record<Kind, { concept: Concept; label: string }> = {
  inbox: { concept: 'inbox', label: 'Your inbox' },
  dm: { concept: 'human', label: 'Direct' },
  task: { concept: 'task', label: 'Task channel' },
  other: { concept: 'channel', label: 'Channel' },
}

// "just now", "5m ago", "3h ago", "2d ago".
function ago(iso: string, now = Date.now()) {
  const s = Math.max(0, Math.round((now - Date.parse(iso)) / 1000))
  if (Number.isNaN(s)) {
    return iso
  }
  if (s < 60) {
    return 'just now'
  }
  if (s < 3600) {
    return `${Math.floor(s / 60)}m ago`
  }
  if (s < 86400) {
    return `${Math.floor(s / 3600)}h ago`
  }
  return `${Math.floor(s / 86400)}d ago`
}

type GroupKey = 'questions' | 'running' | 'done' | 'other'

const GROUPS: { key: GroupKey; label: string }[] = [
  { key: 'questions', label: 'Questions for you' },
  { key: 'running', label: 'Running tasks' },
  { key: 'done', label: 'Done tasks' },
  { key: 'other', label: 'Other' },
  // Archived channels go here behind a toggle once GET /api/msg/channels
  // offers them (?archived=1, sibling task channel-archive); it does not yet.
]

export function BrowsePage() {
  usePageTitle('Browse channels')
  const me = useMyName()
  const unread = useUnread()
  const tasks = useTasks()
  const questions = useOpenQuestions()
  const [filter, setFilter] = useState('')
  const [doneOpen, setDoneOpen] = useState(false)
  const { channelsQuery } = unread
  const taskOf = new Map<string, TaskView>((tasks.data ?? []).map((t) => [`#${t.name}`, t]))
  const needle = filter.trim().toLowerCase()
  const shown = unread.channels.filter((c) => c.channel.toLowerCase().includes(needle))

  // The newest message of each channel, for its last activity.
  const last = useQueries({
    queries: shown.map((c) => ({
      queryKey: queryKeys.lastMessage(c.channel, c.last_id),
      queryFn: ({ signal }: { signal: AbortSignal }) =>
        msg.read({ channel: c.channel, after: c.last_id - 1, limit: 1 }, signal),
      enabled: c.last_id > 0,
      staleTime: Infinity,
    })),
  })
  const lastAt = new Map(shown.map((c, i) => [c.channel, last[i]?.data?.messages.at(-1)?.created_at]))

  function kindOf(channel: string): Kind {
    if (channel === `#${me}`) {
      return 'inbox'
    }
    if (isDMOf(channel, me)) {
      return 'dm'
    }
    return taskOf.has(channel) ? 'task' : 'other'
  }

  function groupOf(channel: string): GroupKey {
    if (questions.of(channel).length > 0) {
      return 'questions'
    }
    const task = taskOf.get(channel)
    if (!task) {
      return 'other'
    }
    return task.status === 'done' ? 'done' : 'running'
  }

  // Each group newest activity first: message ids grow across channels.
  const grouped = new Map<GroupKey, ChannelInfo[]>(GROUPS.map((g) => [g.key, []]))
  for (const c of [...shown].sort((a, b) => b.last_id - a.last_id)) {
    grouped.get(groupOf(c.channel))?.push(c)
  }

  function row(c: ChannelInfo) {
    const kind = KIND[kindOf(c.channel)]
    const Icon = conceptIcons[kind.concept]
    const at = lastAt.get(c.channel)
    const task = taskOf.get(c.channel)
    const u = unread.of(c.channel)
    return (
      <li
        key={c.channel}
        data-testid="browse-row"
        data-channel={c.channel}
        className="flex items-center gap-3 border-b border-border px-4 py-1.5 hover:bg-muted/40"
      >
        <Icon aria-label={kind.label} className={cn('size-4 shrink-0', conceptColors[kind.concept].text)} />
        <Link to={channelPath(c.channel)} className="min-w-0 truncate font-mono text-sm hover:underline">
          {c.channel}
        </Link>
        {task ? (
          <StatusPill status={task.status} />
        ) : (
          <span className="text-xs text-muted-foreground">{kind.label}</span>
        )}
        <span data-testid="browse-last" className="ml-auto text-xs text-muted-foreground" title={at}>
          {c.last_id === 0 ? 'no messages' : at ? ago(at) : ''}
        </span>
        <span className="flex w-20 shrink-0 justify-end">
          <RowBadges count={u.count} mention={u.mention} questions={questions.of(c.channel).length} />
        </span>
        <FollowToggle channel={c.channel} />
      </li>
    )
  }

  return (
    <div className="flex flex-col">
      <div className="sticky top-0 z-10 border-b border-border bg-background px-4 py-2">
        <label className="relative block max-w-md">
          <Search aria-hidden className="absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter channels"
            aria-label="Filter channels"
            className="h-8 pl-7 font-mono text-sm"
            autoFocus
          />
        </label>
      </div>
      {channelsQuery.isPending ? (
        <div className="flex flex-col gap-1 p-4">
          <Skeleton className="h-9" />
          <Skeleton className="h-9" />
        </div>
      ) : channelsQuery.error ? (
        <ErrorText error={channelsQuery.error} className="m-4" />
      ) : shown.length === 0 ? (
        <div className="p-6">
          <EmptySlot reason={needle ? `No channel matches "${filter.trim()}"` : 'No channels yet'} />
        </div>
      ) : (
        <div data-testid="browse-rows" className="flex flex-col">
          {GROUPS.map(({ key, label }) => {
            const list = grouped.get(key) ?? []
            if (list.length === 0) {
              return null
            }
            const collapsible = key === 'done'
            const open = !collapsible || doneOpen
            const Chevron = open ? ChevronDown : ChevronRight
            return (
              <section key={key} role="group" aria-label={label} data-testid="browse-group" data-group={key}>
                <h2 className="border-b border-border bg-muted/30 px-4 py-1 text-[11px] font-medium tracking-wide text-zinc-500 uppercase">
                  {collapsible ? (
                    <button
                      type="button"
                      data-testid="browse-group-toggle"
                      aria-expanded={open}
                      onClick={() => setDoneOpen(!doneOpen)}
                      className="flex items-center gap-1 uppercase hover:text-foreground"
                    >
                      <Chevron aria-hidden className="size-3.5" />
                      {label}
                      <span data-testid="browse-group-count" className="font-mono">
                        {list.length}
                      </span>
                    </button>
                  ) : (
                    <span className="flex items-center gap-1">
                      {label}
                      <span data-testid="browse-group-count" className="font-mono">
                        {list.length}
                      </span>
                    </span>
                  )}
                </h2>
                {open && <ul className="flex flex-col">{list.map(row)}</ul>}
              </section>
            )
          })}
        </div>
      )}
    </div>
  )
}

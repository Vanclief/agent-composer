// Talk mode's channel list under Channels, mounted by the shell
// (src/shell/layout.tsx). Inbox (#<me>) is pinned first, even before it
// has a message; then the Tasks group (channels named after a task), then
// Other. Each row has its unread badge and is bold while unread.
import { Lock } from 'lucide-react'
import { NavLink } from 'react-router'

import { useTasks } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { UnreadBadge } from './UnreadBadge'
import { useUnread, type ChannelUnread } from './unread'

function rowClass(isActive: boolean, unread: boolean) {
  return cn(
    'flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-sm transition-colors',
    isActive
      ? cn('font-medium', conceptColors.message.selected)
      : 'border-transparent text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
    unread && 'font-semibold text-zinc-900 dark:text-white',
  )
}

function ChannelRow({ channel, readOnly, unread }: { channel: string; readOnly: boolean; unread: ChannelUnread }) {
  const name = channel.replace(/^#/, '')
  return (
    <li>
      <NavLink
        to={`/messages/${encodeURIComponent(name)}`}
        title={readOnly ? 'Read only' : undefined}
        data-channel={channel}
        data-unread={unread.count > 0}
        className={({ isActive }) => rowClass(isActive, unread.count > 0)}
      >
        {readOnly && <Lock aria-label="Read only" className="size-3.5 shrink-0" />}
        <span className="truncate font-mono">#{name}</span>
        <UnreadBadge count={unread.count} mention={unread.mention} />
      </NavLink>
    </li>
  )
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <li role="group" aria-label={label} data-testid="channel-group" className="mt-2">
      <span className="block px-2 pb-0.5 text-[11px] font-medium tracking-wide text-zinc-500 uppercase">{label}</span>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </li>
  )
}

export function ChannelRows() {
  const unread = useUnread()
  const tasks = useTasks()
  const { channelsQuery, inbox } = unread
  const InboxIcon = conceptIcons.inbox
  const inboxUnread = unread.of(inbox)

  const taskNames = new Set((tasks.data ?? []).map((t) => `#${t.name}`))
  const rest = unread.channels.filter((c) => c.channel !== inbox)
  const taskChannels = rest.filter((c) => taskNames.has(c.channel))
  const other = rest.filter((c) => !taskNames.has(c.channel))

  return (
    <ul data-testid="channel-rows" className="mt-0.5 flex flex-col gap-0.5 pl-6">
      <li>
        <NavLink
          to={`/messages/${encodeURIComponent(unread.me)}`}
          title={`Your inbox, ${inbox}`}
          data-testid="inbox-row"
          data-channel={inbox}
          data-unread={inboxUnread.count > 0}
          className={({ isActive }) => rowClass(isActive, inboxUnread.count > 0)}
        >
          <InboxIcon aria-hidden className={cn('size-3.5 shrink-0', conceptColors.inbox.text)} />
          <span className="truncate">Inbox</span>
          <UnreadBadge count={inboxUnread.count} mention={inboxUnread.mention} />
        </NavLink>
      </li>
      {channelsQuery.isPending ? (
        <li data-testid="channel-rows-loading" className="flex flex-col gap-0.5">
          <Skeleton className="h-7" />
          <Skeleton className="h-7" />
        </li>
      ) : channelsQuery.error ? (
        <li>
          <ErrorText error={channelsQuery.error} className="px-2 py-1.5 text-xs" />
        </li>
      ) : (
        <>
          {taskChannels.length > 0 && (
            <Group label="Tasks">
              {taskChannels.map((c) => (
                <ChannelRow key={c.channel} channel={c.channel} readOnly={c.read_only} unread={unread.of(c.channel)} />
              ))}
            </Group>
          )}
          {other.length > 0 && (
            <Group label="Other">
              {other.map((c) => (
                <ChannelRow key={c.channel} channel={c.channel} readOnly={c.read_only} unread={unread.of(c.channel)} />
              ))}
            </Group>
          )}
        </>
      )}
    </ul>
  )
}

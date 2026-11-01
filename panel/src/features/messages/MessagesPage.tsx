// Every channel: a rail of channel tiles and the selected channel's feed.
// The :channel param is the channel without its #.
import { NavLink, useParams } from 'react-router'

import { useChannels } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { ChannelView } from './ChannelView'

function ChannelRail() {
  const channels = useChannels()
  const Channel = conceptIcons.channel

  return (
    <nav
      aria-label="Channels"
      className="flex w-56 shrink-0 flex-col gap-1 overflow-y-auto border-r border-zinc-200 p-2 dark:border-zinc-800"
    >
      <ErrorText error={channels.error} />
      {channels.data?.length === 0 && <EmptySlot reason="No channels" />}
      {channels.data?.map((c) => (
        <NavLink
          key={c.channel}
          to={`/messages/${encodeURIComponent(c.channel.slice(1))}`}
          title={`${c.count} messages`}
          className={({ isActive }) =>
            cn(
              'flex items-center gap-2 rounded-md border px-2 py-1.5 text-sm transition-colors',
              isActive
                ? 'border-zinc-300 bg-zinc-200 font-medium text-zinc-900 dark:border-zinc-700 dark:bg-zinc-800 dark:text-white'
                : 'border-transparent text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
            )
          }
        >
          <Channel aria-hidden className="size-4 shrink-0 text-zinc-500" />
          <span className="truncate font-mono">{c.channel.slice(1)}</span>
          <span
            data-testid="channel-count"
            className="ml-auto rounded border border-zinc-300 px-1.5 font-mono text-xs text-zinc-600 dark:border-zinc-700 dark:text-zinc-400"
          >
            {c.count}
          </span>
        </NavLink>
      ))}
    </nav>
  )
}

export function MessagesPage() {
  const { channel: slug } = useParams()
  const channel = slug ? `#${slug}` : undefined
  usePageTitle(channel ? `Messages ${channel}` : 'Messages')

  return (
    <div className="flex h-full min-h-0">
      <ChannelRail />
      <div className="flex min-w-0 flex-1 flex-col">
        {channel ? (
          <ChannelView key={channel} channel={channel} />
        ) : (
          <div className="flex flex-1 items-center justify-center p-6">
            <EmptySlot reason="Pick a channel" />
          </div>
        )}
      </div>
    </div>
  )
}

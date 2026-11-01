// Every channel: a rail of channel tiles and the selected channel's feed.
// The :channel param is the channel without its #.
import { NavLink, useParams } from 'react-router'

import { useChannels } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { ChannelView } from './ChannelView'

function ChannelRail() {
  const channels = useChannels()
  const Channel = conceptIcons.channel

  return (
    <nav aria-label="Channels" className="flex w-56 shrink-0 flex-col gap-1 overflow-y-auto border-r border-border p-2">
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
              // The open channel takes the channel tint; the rest stay dim.
              isActive
                ? [conceptColors.channel.bg, conceptColors.channel.border, 'font-medium text-foreground']
                : 'border-transparent text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
            )
          }
        >
          <Channel aria-hidden className={cn('size-4 shrink-0', conceptColors.channel.text)} />
          <span className="truncate font-mono">{c.channel.slice(1)}</span>
          <span
            data-testid="channel-count"
            className="ml-auto rounded border border-border px-1.5 font-mono text-xs text-muted-foreground"
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

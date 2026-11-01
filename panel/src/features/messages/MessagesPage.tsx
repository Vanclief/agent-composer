// PLACEHOLDER owned by the messages feature: replace freely, keep the
// export. The :channel param is the channel without its #.
import { Link, useParams } from 'react-router'

import { useChannelMessages, useChannels } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { usePageTitle } from '@/shell/page-title'

import { MessageList } from './MessageList'

export function MessagesPage() {
  const { channel: slug } = useParams()
  const channel = slug ? `#${slug}` : undefined
  usePageTitle(channel ? `Messages ${channel}` : 'Messages')
  const channels = useChannels()
  const feed = useChannelMessages(channel)

  return (
    <div className="flex flex-col gap-4 p-6">
      <h2 className="text-2xl font-semibold tracking-tight">MessagesPage</h2>
      <ErrorText error={channels.error} />
      <div className="flex flex-wrap gap-1.5">
        {channels.data?.map((c) => (
          <Link key={c.channel} to={`/messages/${encodeURIComponent(c.channel.slice(1))}`}>
            <IconChip icon={conceptIcons.channel}>
              {c.channel.slice(1)} · {c.count}
            </IconChip>
          </Link>
        ))}
      </div>
      <ErrorText error={feed.error} />
      {channel && <MessageList messages={feed.messages} />}
    </div>
  )
}

// The open channel's feed. The channel list lives in the Talk sidebar
// (src/shell/layout.tsx). The :channel param is the channel without its #.
import { useParams } from 'react-router'

import { EmptySlot } from '@/components/empty-slot'
import { usePageTitle } from '@/shell/page-title'

import { ChannelView } from './ChannelView'

export function MessagesPage() {
  const { channel: slug } = useParams()
  const channel = slug ? `#${slug}` : undefined
  usePageTitle(channel ? `Messages ${channel}` : 'Messages')

  return (
    <div className="flex h-full min-h-0 flex-col">
      {channel ? (
        <ChannelView key={channel} channel={channel} />
      ) : (
        <div className="flex flex-1 items-center justify-center p-6">
          <EmptySlot reason="Pick a channel on the left" />
        </div>
      )}
    </div>
  )
}

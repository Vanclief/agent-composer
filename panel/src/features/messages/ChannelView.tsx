// One channel's live feed and post box, filling the height it is given.
import { useChannelMessages } from '@/api/hooks'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'

import { Composer } from './Composer'
import { useIdentity } from './identity'
import { MessageList } from './MessageList'

// channel is "#name".
export function ChannelView({ channel }: { channel: string }) {
  const feed = useChannelMessages(channel)
  const identity = useIdentity()
  const empty = !feed.isPending && !feed.error && feed.messages.length === 0

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <ErrorText error={feed.error} className="m-3 mb-0" />
      {empty ? (
        <div className="flex min-h-0 flex-1 items-center justify-center p-6">
          <EmptySlot reason="No messages" />
        </div>
      ) : (
        <MessageList messages={feed.messages} />
      )}
      <Composer channel={channel} identity={identity} />
    </div>
  )
}

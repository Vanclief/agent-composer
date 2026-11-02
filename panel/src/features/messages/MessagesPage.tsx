// The open channel's feed. The channel list lives in the Talk sidebar
// (src/shell/layout.tsx). The :channel param is the channel without its #.
import { useParams } from 'react-router'

import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { ChannelView } from './ChannelView'
import { useMyName } from './identity'

const INBOX_LINE = 'Your inbox: agents and tasks escalate here'

// #<me> (ADR 0009 item 5): the ordinary channel view under one line.
function InboxHeader() {
  const Icon = conceptIcons.inbox
  return (
    <p
      data-testid="inbox-header"
      className="flex items-center gap-1.5 border-b border-border px-4 py-2 text-xs text-muted-foreground"
    >
      <Icon aria-hidden className={cn('size-3.5', conceptColors.inbox.text)} />
      {INBOX_LINE}
    </p>
  )
}

export function MessagesPage() {
  const { channel: slug } = useParams()
  const channel = slug ? `#${slug}` : undefined
  const me = useMyName()
  const inbox = slug === me
  usePageTitle(channel ? `${inbox ? 'Inbox' : 'Messages'} ${channel}` : 'Messages')

  return (
    <div className="flex h-full min-h-0 flex-col">
      {channel ? (
        <ChannelView
          key={channel}
          channel={channel}
          header={inbox ? <InboxHeader /> : undefined}
          emptyReason={inbox ? 'No messages yet: agents and tasks escalate here' : undefined}
        />
      ) : (
        <div className="flex flex-1 items-center justify-center p-6">
          <EmptySlot reason="Pick a channel on the left" />
        </div>
      )}
    </div>
  )
}

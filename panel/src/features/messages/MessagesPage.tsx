// The open channel's feed. The channel list lives in the Talk sidebar
// (src/shell/layout.tsx). The :channel param is the channel without its #
// (a DM, @<me>/<task>, keeps its @). Opening a channel follows it; the
// header's Following toggle unfollows it.
import { useEffect } from 'react'
import { useParams } from 'react-router'

import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { ChannelView } from './ChannelView'
import { channelOfSlug, dmTask, follow } from './follow'
import { FollowToggle } from './FollowToggle'
import { useMyName } from './identity'
import { isDMOf } from './unread'

const INBOX_LINE = 'Asks: questions and escalations waiting on you'

// #<me> (ADR 0009 item 5), shown as Asks: the ordinary channel view under one line.
function InboxHeader() {
  const Icon = conceptIcons.question
  return (
    <p
      data-testid="inbox-header"
      className="flex items-center gap-1.5 border-b border-border px-4 py-2 text-xs text-muted-foreground"
    >
      <span data-slot="channel-tab" className="inline-flex items-center gap-1.5">
        <Icon aria-hidden className={cn('size-3.5', conceptColors.question.text)} />
        {INBOX_LINE}
      </span>
    </p>
  )
}

// Any other channel: its name (a DM as Direct and its task) and the toggle.
function FollowHeader({ channel, dm }: { channel: string; dm: boolean }) {
  const Icon = dm ? conceptIcons.human : conceptIcons.channel
  const color = dm ? conceptColors.human : conceptColors.channel
  return (
    <div data-testid="channel-header" className="flex items-center gap-1.5 border-b border-border px-4 py-1.5 text-xs">
      <span data-slot="channel-tab" className="inline-flex items-center gap-1.5">
        <Icon aria-hidden className={cn('size-3.5', color.text)} />
        <span className="font-mono">{channel}</span>
      </span>
      {dm && <span className="text-muted-foreground">Direct · task {dmTask(channel)}</span>}
      <FollowToggle channel={channel} className="ml-auto" />
    </div>
  )
}

export function MessagesPage() {
  const { channel: slug } = useParams()
  const channel = slug ? channelOfSlug(slug) : undefined
  const me = useMyName()
  const inbox = slug === me
  const dm = channel ? isDMOf(channel, me) : false
  usePageTitle(channel ? `${inbox ? 'Asks' : dm ? 'Direct' : 'Messages'} ${channel}` : 'Messages')

  useEffect(() => {
    if (channel) {
      follow(channel)
    }
  }, [channel])

  return (
    <div className="flex h-full min-h-0 flex-col">
      {channel ? (
        <ChannelView
          key={channel}
          channel={channel}
          header={inbox ? <InboxHeader /> : <FollowHeader channel={channel} dm={dm} />}
          emptyReason={inbox ? 'Nothing asked yet: questions waiting on you arrive here' : undefined}
        />
      ) : (
        <div className="flex flex-1 items-center justify-center p-6">
          <EmptySlot reason="Pick a channel on the left" />
        </div>
      )}
    </div>
  )
}

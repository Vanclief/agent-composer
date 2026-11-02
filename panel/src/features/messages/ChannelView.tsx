// One channel's live feed and post box, filling the height it is given.
// While it is open, every message it shows counts as read (unread.ts).
import { useEffect, type ReactNode } from 'react'

import { useChannelMessages } from '@/api/hooks'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { Avatar } from '@/components/avatar'
import { cn } from '@/lib/utils'

import { Composer } from './Composer'
import { useIdentity } from './identity'
import { membersOf, type Member } from './members'
import { MessageList } from './MessageList'
import { presenceLabel, usePresence, type Presence } from './presence'
import { markRead } from './unread'

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

// Who is in the channel, humans first, then a count of each kind. Each
// member is its avatar with a presence dot (presence.ts) and its name; the
// tooltip says "@name · online" or "@name · last seen 3 min ago".
function MembersStrip({ members, presence }: { members: Member[]; presence: Map<string, Presence> }) {
  const humans = members.filter((m) => m.human).length
  return (
    <div
      role="group"
      aria-label="Members"
      data-testid="members"
      className="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-border px-4 py-2"
    >
      {members.map((m) => {
        const p = presence.get(m.name)
        const kind = m.human ? 'Human' : `Agent in task ${m.task}`
        return (
          <span
            key={`${m.task}/${m.name}`}
            data-member={m.name}
            title={`@${m.name} · ${presenceLabel(p)} (${kind})`}
            className={cn(
              'inline-flex items-center gap-1.5 font-mono text-xs',
              p?.online ? '' : 'text-muted-foreground',
            )}
          >
            <Avatar name={m.name} size="sm" human={m.human} presence={p?.online ?? false} />@{m.name}
          </span>
        )
      })}
      <span data-testid="members-count" className="ml-auto font-mono text-xs text-muted-foreground">
        {plural(members.length - humans, 'agent')} · {plural(humans, 'human')}
      </span>
    </div>
  )
}

// channel is "#name". header goes above the members; emptyReason is the
// empty slot's text when the channel has no messages.
export function ChannelView({
  channel,
  header,
  emptyReason = 'No messages',
}: {
  channel: string
  header?: ReactNode
  emptyReason?: string
}) {
  const feed = useChannelMessages(channel)
  const identity = useIdentity()
  const empty = !feed.isPending && !feed.error && feed.messages.length === 0
  const members = membersOf(feed.messages)
  const presence = usePresence(channel)
  const lastID = feed.messages.at(-1)?.id

  useEffect(() => {
    if (lastID !== undefined) {
      markRead(channel, lastID)
    }
  }, [channel, lastID])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {header}
      {members.length > 0 && <MembersStrip members={members} presence={presence} />}
      <ErrorText error={feed.error} className="m-3 mb-0" />
      {empty ? (
        <div className="flex min-h-0 flex-1 items-center justify-center p-6">
          <EmptySlot reason={emptyReason} />
        </div>
      ) : (
        <MessageList messages={feed.messages} />
      )}
      <Composer channel={channel} identity={identity} />
    </div>
  )
}

// One channel's live feed and post box, filling the height it is given,
// with the thread pane on the right when one is open. While it is open,
// every message it shows counts as read (unread.ts); open questions for me
// are pinned at the top ("Waiting for you").
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { ArrowDown } from 'lucide-react'
import { useSearchParams } from 'react-router'

import { useChannelMessages } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { Avatar } from '@/components/avatar'
import { cn } from '@/lib/utils'

import { Composer } from './Composer'
import { ChannelContext } from './context'
import { useIdentity, useMyName } from './identity'
import { membersOf, type Member } from './members'
import { MessageList } from './MessageList'
import { answerOf, useOpenQuestions, useQuestions } from './questions'
import { ThreadPane } from './ThreadPane'
import { buildThreads } from './threads'
import { presenceLabel, usePresence, type Presence } from './presence'
import { useNow } from './time'
import { markRead, useReadMap } from './unread'

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

// "Waiting for you: N": open questions here addressed to me. Each click
// scrolls to the next one.
function WaitingStrip({ ids, onJump }: { ids: number[]; onJump: (id: number) => void }) {
  const [next, setNext] = useState(0)
  const Icon = conceptIcons.question
  if (ids.length === 0) {
    return null
  }
  return (
    <button
      type="button"
      data-testid="waiting-strip"
      onClick={() => {
        onJump(ids[next % ids.length])
        setNext(next + 1)
      }}
      title="Open questions addressed to you. Click to scroll to the next one."
      className={cn(
        'flex shrink-0 items-center gap-2 border-b px-4 py-1.5 text-left text-xs font-medium',
        conceptColors.question.bg,
        conceptColors.question.border,
        conceptColors.question.text,
      )}
    >
      <Icon aria-hidden className="size-4" />
      <span>
        Waiting for you: <span data-testid="waiting-count">{ids.length}</span>
      </span>
      <ArrowDown aria-hidden className="ml-auto size-3.5" />
    </button>
  )
}

// channel is "#name". header goes above the members; emptyReason is the
// empty slot's text when the channel has no messages. ?thread=<id> opens
// that message's thread on the right.
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
  const me = useMyName()
  const now = useNow()
  const read = useReadMap()
  // The read cursor as it was when the channel opened, before this view
  // moved it: the "Unread" line sits after it.
  const [openedAt] = useState(() => read[channel])
  const [params, setParams] = useSearchParams()
  const root = useRef<HTMLDivElement>(null)
  const empty = !feed.isPending && !feed.error && feed.messages.length === 0
  const members = membersOf(feed.messages)
  const presence = usePresence(channel)
  const lastID = feed.messages.at(-1)?.id
  const threads = useMemo(() => buildThreads(feed.messages), [feed.messages])
  const channelQuestions = useQuestions({ channel })
  const open = useOpenQuestions()

  const threadParam = Number(params.get('thread'))
  const threadID = Number.isInteger(threadParam) && threadParam > 0 ? threadParam : undefined

  useEffect(() => {
    if (lastID !== undefined) {
      markRead(channel, lastID)
    }
  }, [channel, lastID])

  const questions = useMemo(() => new Map((channelQuestions.data ?? []).map((q) => [q.id, q])), [channelQuestions.data])

  // Open for me per the server, and not answered in the feed meanwhile.
  const waiting = open
    .of(channel)
    .filter((q) => {
      const loaded = threads.byID.get(q.id)
      return !loaded || !answerOf(loaded, threads.replies(q.id))
    })
    .map((q) => q.id)

  const unreadID = openedAt === undefined ? undefined : threads.top.find((m) => m.id > openedAt && m.from !== me)?.id

  const value = useMemo(
    () => ({
      channel,
      identity,
      me,
      threads,
      questions,
      now,
      openThread: (id: number) =>
        setParams(
          (p) => {
            p.set('thread', String(id))
            return p
          },
          { replace: true },
        ),
    }),
    [channel, identity, me, threads, questions, now, setParams],
  )

  function closeThread() {
    setParams(
      (p) => {
        p.delete('thread')
        return p
      },
      { replace: true },
    )
  }

  function jumpTo(id: number) {
    const el = root.current?.querySelector<HTMLElement>(`[data-testid="message-list"] [data-message-id="${id}"]`)
    if (!el) {
      return
    }
    el.scrollIntoView({ block: 'center', behavior: 'smooth' })
    el.animate([{ backgroundColor: 'rgb(236 72 153 / 0.2)' }, { backgroundColor: 'transparent' }], {
      duration: 1600,
      easing: 'ease-out',
    })
  }

  return (
    <ChannelContext.Provider value={value}>
      <div ref={root} className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          {header}
          {members.length > 0 && <MembersStrip members={members} presence={presence} />}
          <WaitingStrip ids={waiting} onJump={jumpTo} />
          <ErrorText error={feed.error} className="m-3 mb-0" />
          {empty ? (
            <div className="flex min-h-0 flex-1 items-center justify-center p-6">
              <EmptySlot reason={emptyReason} />
            </div>
          ) : (
            <MessageList messages={threads.top} unreadID={unreadID} />
          )}
          <Composer channel={channel} identity={identity} />
        </div>
        {threadID !== undefined && <ThreadPane key={threadID} id={threadID} onClose={closeThread} />}
      </div>
    </ChannelContext.Provider>
  )
}

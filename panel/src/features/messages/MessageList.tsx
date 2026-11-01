// A channel's messages, newest at the bottom. Sticks to the bottom while the
// reader is there; otherwise a "New messages" pill appears when more arrive.
// Reads like a chat log: consecutive messages from one participant within
// GROUP_GAP_MS form a group under one header (icon, name, task, time); the
// rest of the group is bodies only, their time shown in the gutter on hover.
import { useLayoutEffect, useRef, useState } from 'react'
import { ArrowDown, Reply } from 'lucide-react'
import { Link } from 'react-router'

import type { Message } from '@/api/msg'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
import { cn } from '@/lib/utils'

// How close to the bottom (px) still counts as at the bottom.
const BOTTOM_SLACK = 32
// The longest pause that still continues a group.
const GROUP_GAP_MS = 5 * 60_000
// How much of a replied-to message its stub quotes.
const QUOTE_CHARS = 60

function groupByAuthor(messages: Message[]) {
  const groups: Message[][] = []
  for (const m of messages) {
    const last = groups.at(-1)
    const prev = last?.at(-1)
    if (
      last &&
      prev &&
      prev.participant_id === m.participant_id &&
      Date.parse(m.created_at) - Date.parse(prev.created_at) <= GROUP_GAP_MS
    ) {
      last.push(m)
    } else {
      groups.push([m])
    }
  }
  return groups
}

function clock(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// A human or an agent (its task's icon), in the gutter left of the header.
function SenderIcon({ message }: { message: Message }) {
  const human = !message.task
  const Icon = human ? conceptIcons.human : conceptIcons.task
  return (
    <Icon
      aria-hidden
      data-concept={human ? 'human' : 'task'}
      className={cn('ml-auto size-4', human ? conceptColors.human.text : conceptColors.task.text)}
    />
  )
}

// @name in mono, the agent's task as a link, and the first time.
function Header({ message }: { message: Message }) {
  const human = !message.task
  return (
    <div className="flex items-center gap-2">
      <span
        data-testid="author"
        data-kind={human ? 'human' : 'agent'}
        title={human ? 'Human' : `Agent in task ${message.task}`}
        className="font-mono text-sm font-semibold text-foreground"
      >
        @{message.from}
      </span>
      {!human && (
        <Link to={`/tasks/${encodeURIComponent(message.task)}`} title={`Task ${message.task}`}>
          <IconChip icon={conceptIcons.task} concept="task" className="hover:brightness-125">
            {message.task}
          </IconChip>
        </Link>
      )}
      <time
        dateTime={message.created_at}
        title={new Date(message.created_at).toLocaleString()}
        className="font-mono text-xs text-muted-foreground"
      >
        {clock(message.created_at)}
      </time>
    </div>
  )
}

// One line quoting the message replied to, or its id when not loaded.
function ReplyStub({ id, target }: { id: number; target?: Message }) {
  let text = `replying to #${id}`
  if (target) {
    const quote = target.body.length > QUOTE_CHARS ? target.body.slice(0, QUOTE_CHARS) + '…' : target.body
    text = `replying to @${target.from}: ${quote}`
  }
  return (
    <div
      data-testid="reply-stub"
      title={`#${id}`}
      className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground"
    >
      <Reply aria-hidden className="size-3 shrink-0" />
      <span className="truncate">{text}</span>
    </div>
  )
}

export function MessageList({ messages }: { messages: Message[] }) {
  const byID = new Map(messages.map((m) => [m.id, m]))
  const scroller = useRef<HTMLDivElement>(null)
  const stick = useRef(true)
  const [atBottom, setAtBottom] = useState(true)
  const lastID = messages.at(-1)?.id ?? 0
  const [seenID, setSeenID] = useState(lastID)

  // New messages while at the bottom: follow them.
  useLayoutEffect(() => {
    const el = scroller.current
    if (el && stick.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [lastID])

  function onScroll() {
    const el = scroller.current
    if (!el) {
      return
    }
    const bottom = el.scrollHeight - el.scrollTop - el.clientHeight < BOTTOM_SLACK
    stick.current = bottom
    setAtBottom(bottom)
    if (bottom) {
      setSeenID(lastID)
    }
  }

  function toBottom() {
    const el = scroller.current
    if (el) {
      el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    }
  }

  return (
    <div className="relative min-h-0 flex-1">
      <div ref={scroller} onScroll={onScroll} data-testid="message-list" className="h-full overflow-y-auto px-4 py-3">
        <ol className="flex flex-col gap-2 text-sm">
          {groupByAuthor(messages).map((group) => (
            <li key={group[0].id} data-testid="message-group" className="flex flex-col">
              {group.map((m, i) => (
                <div
                  key={m.id}
                  data-testid="message"
                  className="group flex gap-2 rounded-md px-1 py-0.5 hover:bg-zinc-100 dark:hover:bg-zinc-800/40"
                >
                  {/* The gutter: the sender's icon by the header, the time on hover after. */}
                  <div className="flex w-11 shrink-0 justify-end pt-0.5">
                    {i === 0 ? (
                      <SenderIcon message={m} />
                    ) : (
                      <time
                        dateTime={m.created_at}
                        title={new Date(m.created_at).toLocaleString()}
                        className="font-mono text-[10px] text-muted-foreground opacity-0 group-hover:opacity-100"
                      >
                        {clock(m.created_at)}
                      </time>
                    )}
                  </div>
                  <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                    {i === 0 && <Header message={m} />}
                    {m.reply_to !== undefined && <ReplyStub id={m.reply_to} target={byID.get(m.reply_to)} />}
                    <p title={`#${m.id}`} className="break-words whitespace-pre-wrap">
                      {m.body}
                    </p>
                  </div>
                </div>
              ))}
            </li>
          ))}
        </ol>
      </div>
      {!atBottom && lastID > seenID && (
        <button
          type="button"
          onClick={toBottom}
          className={cn(
            'absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-1 rounded-full px-3 py-1 text-xs font-medium',
            conceptColors.message.solid,
          )}
        >
          New messages <ArrowDown className="size-3.5" />
        </button>
      )}
    </div>
  )
}

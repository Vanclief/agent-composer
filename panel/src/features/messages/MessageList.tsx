// A channel's top-level messages, newest at the bottom. Sticks to the
// bottom while the reader is there; otherwise a "New messages" pill
// appears when more arrive. Reads like a chat log: a day separator where
// the day changes, an "Unread" line before the first message that was
// unread when the channel opened (with "Jump to unread" while it is above
// the fold), and consecutive messages from one participant within
// GROUP_GAP_MS grouped under one header. agc's posts are compact system
// lines and questions are cards; both stand alone. Replies are not here:
// their parent's footer opens the thread pane.
import { useLayoutEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp } from 'lucide-react'

import type { Message } from '@/api/msg'
import { conceptColors } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

import { useChannel } from './context'
import { isSystem, MessageRow, QuestionCard, SystemLine } from './parts'
import { isQuestion } from './questions'
import { dayKey, dayLabel } from './time'

// How close to the bottom (px) still counts as at the bottom.
const BOTTOM_SLACK = 32
// The longest pause that still continues a group.
const GROUP_GAP_MS = 5 * 60_000

type Item =
  | { type: 'day'; key: string; label: string }
  | { type: 'unread'; key: string }
  | { type: 'system' | 'question'; key: string; message: Message }
  | { type: 'group'; key: string; messages: Message[] }

function itemsOf(messages: Message[], unreadID: number | undefined, now: number) {
  const items: Item[] = []
  let day = ''
  let group: Message[] | undefined
  for (const m of messages) {
    const d = dayKey(m.created_at)
    if (d !== day) {
      day = d
      group = undefined
      items.push({ type: 'day', key: `day-${d}`, label: dayLabel(m.created_at, now) })
    }
    if (m.id === unreadID) {
      group = undefined
      items.push({ type: 'unread', key: 'unread' })
    }
    if (isSystem(m) || isQuestion(m)) {
      group = undefined
      items.push({ type: isSystem(m) ? 'system' : 'question', key: String(m.id), message: m })
      continue
    }
    const prev = group?.at(-1)
    if (
      group &&
      prev &&
      prev.participant_id === m.participant_id &&
      Date.parse(m.created_at) - Date.parse(prev.created_at) <= GROUP_GAP_MS
    ) {
      group.push(m)
      continue
    }
    group = [m]
    items.push({ type: 'group', key: String(m.id), messages: group })
  }
  return items
}

function Divider({ label, testid, className }: { label: string; testid: string; className?: string }) {
  return (
    <li
      data-testid={testid}
      role="separator"
      aria-label={label}
      className={cn('flex items-center gap-2 py-1', className)}
    >
      <span className="h-px flex-1 bg-current opacity-30" />
      <span className="text-[11px] font-medium tracking-wide uppercase">{label}</span>
      <span className="h-px flex-1 bg-current opacity-30" />
    </li>
  )
}

export function GroupRows({ messages, inThread }: { messages: Message[]; inThread?: boolean }) {
  return (
    <li data-testid="message-group" className="flex flex-col">
      {messages.map((m, i) => (
        <MessageRow key={m.id} message={m} first={i === 0} inThread={inThread} />
      ))}
    </li>
  )
}

// unreadID is the first message unread when the channel opened.
export function MessageList({ messages, unreadID }: { messages: Message[]; unreadID?: number }) {
  const { now } = useChannel()
  const scroller = useRef<HTMLDivElement>(null)
  const stick = useRef(true)
  const [atBottom, setAtBottom] = useState(true)
  const [unreadAbove, setUnreadAbove] = useState(false)
  const lastID = messages.at(-1)?.id ?? 0
  const [seenID, setSeenID] = useState(lastID)

  // New messages while at the bottom: follow them.
  useLayoutEffect(() => {
    const el = scroller.current
    if (el && stick.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [lastID])

  function unreadLine() {
    return scroller.current?.querySelector<HTMLElement>('[data-testid="unread-line"]') ?? null
  }

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
    const line = unreadLine()
    setUnreadAbove(line !== null && line.offsetTop < el.scrollTop)
  }

  function toBottom() {
    const el = scroller.current
    if (el) {
      el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    }
  }

  function toUnread() {
    unreadLine()?.scrollIntoView({ block: 'start', behavior: 'smooth' })
  }

  return (
    <div className="relative min-h-0 flex-1">
      <div
        ref={scroller}
        onScroll={onScroll}
        data-testid="message-list"
        className="relative h-full overflow-y-auto px-4 py-3"
      >
        <ol className="flex flex-col gap-2 text-sm">
          {itemsOf(messages, unreadID, now).map((item) => {
            switch (item.type) {
              case 'day':
                return (
                  <Divider key={item.key} testid="day-separator" label={item.label} className="text-muted-foreground" />
                )
              case 'unread':
                return (
                  <Divider
                    key={item.key}
                    testid="unread-line"
                    label="Unread"
                    className={cn('scroll-mt-2', conceptColors.message.text)}
                  />
                )
              case 'system':
                return (
                  <li key={item.key}>
                    <SystemLine message={item.message} />
                  </li>
                )
              case 'question':
                return (
                  <li key={item.key}>
                    <QuestionCard message={item.message} />
                  </li>
                )
              default:
                return <GroupRows key={item.key} messages={item.messages} />
            }
          })}
        </ol>
      </div>
      {unreadAbove && (
        <button
          type="button"
          onClick={toUnread}
          className={cn(
            'absolute top-3 left-1/2 flex -translate-x-1/2 items-center gap-1 rounded-full border px-3 py-1 text-xs font-medium shadow-sm',
            conceptColors.message.bg,
            conceptColors.message.border,
            conceptColors.message.text,
            'bg-background',
          )}
        >
          Jump to unread <ArrowUp className="size-3.5" />
        </button>
      )}
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

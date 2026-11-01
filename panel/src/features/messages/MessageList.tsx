// A channel's messages, newest at the bottom. Sticks to the bottom while the
// reader is there; otherwise a "New messages" pill appears when more arrive.
// Consecutive messages from one participant form a group under one header.
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { ArrowDown } from 'lucide-react'

import type { Message } from '@/api/msg'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

// How close to the bottom (px) still counts as at the bottom.
const BOTTOM_SLACK = 32
// How often relative times refresh.
const TICK_MS = 30_000

function groupByAuthor(messages: Message[]) {
  const groups: Message[][] = []
  for (const m of messages) {
    const last = groups.at(-1)
    if (last && last[0].participant_id === m.participant_id) {
      last.push(m)
    } else {
      groups.push([m])
    }
  }
  return groups
}

function relativeTime(iso: string, now: number) {
  const seconds = Math.max(0, Math.round((now - Date.parse(iso)) / 1000))
  if (seconds < 60) {
    return 'now'
  }
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) {
    return `${minutes}m`
  }
  const hours = Math.round(minutes / 60)
  if (hours < 24) {
    return `${hours}h`
  }
  return `${Math.round(hours / 24)}d`
}

function useNow() {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), TICK_MS)
    return () => window.clearInterval(id)
  }, [])
  return now
}

function AuthorChip({ message }: { message: Message }) {
  const human = !message.task
  const Icon = human ? conceptIcons.human : conceptIcons.task
  return (
    <span
      data-testid="author"
      data-kind={human ? 'human' : 'agent'}
      title={human ? 'Human' : `Agent in task ${message.task}`}
      className={cn(
        'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-xs font-medium',
        // A human is lit in the messaging color; an agent's chip stays
        // neutral with its task icon in the task color.
        human
          ? [conceptColors.human.bg, conceptColors.human.border, conceptColors.human.text]
          : 'border-border text-zinc-800 dark:text-zinc-200',
      )}
    >
      <Icon aria-hidden className={cn('size-3.5', human ? conceptColors.human.text : conceptColors.task.text)} />@
      {message.from}
      {!human && <span className="font-mono font-normal text-muted-foreground">{message.task}</span>}
    </span>
  )
}

export function MessageList({ messages }: { messages: Message[] }) {
  const now = useNow()
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
        <ol className="flex flex-col gap-3 text-sm">
          {groupByAuthor(messages).map((group) => (
            <li key={group[0].id} className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <AuthorChip message={group[0]} />
                <span
                  title={new Date(group[0].created_at).toLocaleString()}
                  className="rounded-md border border-border px-1.5 py-0.5 font-mono text-xs text-muted-foreground"
                >
                  {relativeTime(group[0].created_at, now)}
                </span>
              </div>
              {group.map((m) => (
                <p
                  key={m.id}
                  title={`#${m.id} · ${new Date(m.created_at).toLocaleString()}`}
                  className={cn(
                    'border-l-2 pl-2 break-words whitespace-pre-wrap',
                    m.task ? 'border-border' : 'border-pink-500/60',
                  )}
                >
                  {m.body}
                </p>
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

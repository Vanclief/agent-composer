// Who will answer: under a message whose turn_for names persistent agents
// (set by agc's wake hook, shown as given, never re-derived), one row per
// agent still owing a reply: the reply icon, the agent's avatar and name
// (linking to its page), WILL ANSWER and its state verbatim. A row goes
// away once that agent posts in the same thread after the message. In the
// feed, a top-level message also shows the rows of its thread's replies.
import { Reply } from 'lucide-react'
import { Link } from 'react-router'

import { useAgents } from '@/api/hooks'
import type { Message } from '@/api/msg'
import { Avatar } from '@/components/avatar'
import { StatusPill } from '@/components/status-pill'

import { useChannel } from './context'
import type { Threads } from './threads'

// The agents in m.turn_for that have not posted in m's thread after m.
export function waitingOn(threads: Threads, m: Message): string[] {
  const names = m.turn_for ?? []
  if (names.length === 0) {
    return []
  }
  const root = threads.rootOf(m.id)
  const head = threads.byID.get(root)
  const thread = head ? [head, ...threads.replies(root)] : threads.replies(root)
  return names.filter((name) => !thread.some((t) => t.id > m.id && t.from === name))
}

// The agents a top-level message's row shows: its own, then its replies',
// each agent once.
export function waitingInThread(threads: Threads, m: Message): string[] {
  const out: string[] = []
  for (const t of [m, ...threads.replies(m.id)]) {
    for (const name of waitingOn(threads, t)) {
      if (!out.includes(name)) {
        out.push(name)
      }
    }
  }
  return out
}

// inThread: the message is shown in the thread pane, so only its own rows.
export function WillAnswer({ message, inThread }: { message: Message; inThread?: boolean }) {
  const { threads } = useChannel()
  const agents = useAgents()
  const top = !inThread && threads.rootOf(message.id) === message.id
  const names = top ? waitingInThread(threads, message) : waitingOn(threads, message)
  if (names.length === 0) {
    return null
  }
  return (
    <ul data-testid="will-answer-rows" className="flex flex-col gap-0.5 py-0.5">
      {names.map((name) => {
        const agent = agents.data?.find((a) => a.name === name)
        return (
          <li
            key={name}
            data-testid="will-answer"
            data-agent={name}
            className="flex items-center gap-1.5 text-xs text-muted-foreground"
          >
            <Reply aria-hidden className="size-3.5 shrink-0" />
            <Link
              to={`/agents/${encodeURIComponent(name)}`}
              title={`Agent ${name}`}
              className="inline-flex items-center gap-1.5 font-mono text-foreground lowercase hover:underline"
            >
              <Avatar name={name} size="xs" />
              {name}
            </Link>
            <span className="text-[11px] font-semibold tracking-[0.08em] uppercase font-stretch-condensed">
              Will answer
            </span>
            {agent && <StatusPill status={agent.state} />}
          </li>
        )
      })}
    </ul>
  )
}

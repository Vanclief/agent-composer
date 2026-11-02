// Threads: a reply (reply_to) belongs to the thread of the top-level
// message it answers, directly or through other replies. The feed shows
// top-level messages only; a thread's replies, oldest first, are read in
// the thread pane. A reply whose parent is not loaded stays top level.
import type { Message } from '@/api/msg'

export type Threads = {
  // Messages shown in the feed.
  top: Message[]
  // A top-level message's replies, oldest first.
  replies: (id: number) => Message[]
  byID: Map<number, Message>
}

const NONE: Message[] = []

export function buildThreads(messages: Message[]): Threads {
  const byID = new Map(messages.map((m) => [m.id, m]))
  const rootOf = new Map<number, number>()

  function root(m: Message): number {
    const known = rootOf.get(m.id)
    if (known !== undefined) {
      return known
    }
    let r = m.id
    const parent = m.reply_to === undefined ? undefined : byID.get(m.reply_to)
    // Ids grow, so a parent is always older: no cycles.
    if (parent && parent.id < m.id) {
      r = root(parent)
    }
    rootOf.set(m.id, r)
    return r
  }

  const top: Message[] = []
  const replies = new Map<number, Message[]>()
  for (const m of messages) {
    const r = root(m)
    if (r === m.id) {
      top.push(m)
      continue
    }
    const list = replies.get(r)
    if (list) {
      list.push(m)
    } else {
      replies.set(r, [m])
    }
  }
  return { top, replies: (id) => replies.get(id) ?? NONE, byID }
}

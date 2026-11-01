// Who is in a channel, derived from the messages loaded so far. The one
// place to swap for the participants endpoint later.
import type { Message } from '@/api/msg'

export type Member = {
  name: string
  // Empty for humans.
  task: string
  human: boolean
}

// Distinct senders, humans first, then by name.
export function membersOf(messages: Message[]): Member[] {
  const byKey = new Map<string, Member>()
  for (const m of messages) {
    const key = `${m.task}/${m.from}`
    if (!byKey.has(key)) {
      byKey.set(key, { name: m.from, task: m.task, human: m.task === '' })
    }
  }
  return [...byKey.values()].sort((a, b) => Number(b.human) - Number(a.human) || a.name.localeCompare(b.name))
}

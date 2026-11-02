// Who in a channel is online, from GET /api/msg/participants?channel=.
// Polled every 15s; message.posted already invalidates the same query
// (src/api/events.ts), so a post refreshes it at once.
import { useQuery } from '@tanstack/react-query'

import { queryKeys } from '@/api/hooks'
import { msg, type Participant } from '@/api/msg'

export const PRESENCE_POLL_MS = 15_000

export type Presence = { online: boolean; last_seen_at: string | null }

// One entry per name: a name held by several participants (one left, one
// joined again) is online when any of them is, last seen at the latest.
export function presenceByName(participants: Participant[]) {
  const map = new Map<string, Presence>()
  for (const p of participants) {
    const seen = p.last_seen_at ?? null
    const prev = map.get(p.name)
    if (!prev) {
      map.set(p.name, { online: Boolean(p.online), last_seen_at: seen })
      continue
    }
    const later = !prev.last_seen_at || (seen !== null && seen > prev.last_seen_at) ? seen : prev.last_seen_at
    map.set(p.name, { online: prev.online || Boolean(p.online), last_seen_at: later })
  }
  return map
}

// channel is "#name". The map is empty until the first answer.
export function usePresence(channel: string | undefined) {
  const query = useQuery({
    queryKey: queryKeys.participants(channel ?? ''),
    queryFn: async ({ signal }) => (await msg.participants(channel ?? '', signal)).participants,
    enabled: Boolean(channel),
    refetchInterval: PRESENCE_POLL_MS,
  })
  return presenceByName(query.data ?? [])
}

// "online", "last seen 3 min ago", "last seen just now" or "offline".
export function presenceLabel(presence: Presence | undefined, now = Date.now()) {
  if (!presence) {
    return 'offline'
  }
  if (presence.online) {
    return 'online'
  }
  if (!presence.last_seen_at) {
    return 'offline'
  }
  const minutes = Math.floor((now - new Date(presence.last_seen_at).getTime()) / 60_000)
  if (minutes < 1) {
    return 'last seen just now'
  }
  if (minutes < 60) {
    return `last seen ${minutes} min ago`
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `last seen ${hours} h ago`
  }
  return `last seen ${Math.floor(hours / 24)} d ago`
}

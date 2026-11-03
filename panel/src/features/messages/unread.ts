// Unread state, kept in the browser. localStorage agc.panel.read maps each
// channel ("#name") to the id of the last message read there; opening a
// channel (ChannelView) moves it to the newest message shown. A channel is
// unread while its last_id (GET /api/msg/channels, refreshed by every
// message.posted event) is past that id: the messages after it are read
// once to count them, leaving out my own and everything before my own last
// message there (I answered, so I read it), and to see whether one mentions
// @<me>. Never-opened channels count every message not before my last one.
import { useSyncExternalStore } from 'react'
import { useQueries } from '@tanstack/react-query'

import { queryKeys, useChannels } from '@/api/hooks'
import { msg, type ChannelInfo, type Message } from '@/api/msg'

import { useMyName } from './identity'

export const READ_KEY = 'agc.panel.read'
// The most messages read past the cursor; the badge shows 500+ beyond.
export const UNREAD_LIMIT = 500

type ReadMap = Record<string, number>

function load(): ReadMap {
  try {
    const parsed = JSON.parse(localStorage.getItem(READ_KEY) ?? '{}') as unknown
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as ReadMap) : {}
  } catch {
    return {}
  }
}

let readMap = load()
const listeners = new Set<() => void>()

function notify() {
  for (const listener of listeners) {
    listener()
  }
}

// Another tab read a channel.
function onStorage(event: StorageEvent) {
  if (event.key === READ_KEY || event.key === null) {
    readMap = load()
    notify()
  }
}

function subscribe(listener: () => void) {
  if (listeners.size === 0) {
    window.addEventListener('storage', onStorage)
  }
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
    if (listeners.size === 0) {
      window.removeEventListener('storage', onStorage)
    }
  }
}

// Marks everything up to id in channel ("#name") read. The cursor only
// moves forward.
export function markRead(channel: string, id: number) {
  if ((readMap[channel] ?? 0) >= id) {
    return
  }
  readMap = { ...readMap, [channel]: id }
  try {
    localStorage.setItem(READ_KEY, JSON.stringify(readMap))
  } catch {
    // Storage disabled: read state lasts for this page only.
  }
  notify()
}

export function useReadMap() {
  return useSyncExternalStore(subscribe, () => readMap)
}

function escape(text: string) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Whether body mentions @name as a whole word (@franco, not @francos or
// someone@franco).
export function mentions(body: string, name: string) {
  return new RegExp(`(^|[^\\w@.-])@${escape(name)}(?![\\w-])`).test(body)
}

// A DM of me: "@<me>/<task>" (the backend's direct-message channels).
export function isDMOf(channel: string, me: string) {
  return channel.startsWith(`@${me}/`)
}

export type ChannelUnread = { count: number; mention: boolean }

const NONE: ChannelUnread = { count: 0, mention: false }

// The last count per channel and the cursor it was counted from. While a
// new last_id loads, the count from the same cursor stays on screen
// instead of blinking out.
const previous = new Map<string, { after: number; unread: ChannelUnread }>()

export type Unread = {
  // The signed-in name, and its inbox channel "#<me>".
  me: string
  inbox: string
  channels: ChannelInfo[]
  channelsQuery: ReturnType<typeof useChannels>
  of: (channel: string) => ChannelUnread
  total: number
  // Some unread message anywhere mentions @<me>.
  mention: boolean
  // Per channel, the messages read past the cursor: the scan follow.ts
  // follows channels from (followsMe).
  scanned: Map<string, Message[]>
}

export function useUnread(): Unread {
  const me = useMyName()
  const read = useReadMap()
  const channelsQuery = useChannels()
  const channels = channelsQuery.data ?? []
  const pending = channels.filter((c) => c.last_id > (read[c.channel] ?? 0))

  const results = useQueries({
    queries: pending.map((c) => {
      const after = read[c.channel] ?? 0
      return {
        queryKey: queryKeys.unread(c.channel, after, c.last_id),
        queryFn: ({ signal }: { signal: AbortSignal }) =>
          msg.read({ channel: c.channel, after, limit: UNREAD_LIMIT }, signal),
        staleTime: Infinity,
      }
    }),
  })

  const byChannel = new Map<string, ChannelUnread>()
  const scanned = new Map<string, Message[]>()
  pending.forEach((c, i) => {
    const after = read[c.channel] ?? 0
    const data = results[i]?.data
    if (!data) {
      const kept = previous.get(c.channel)
      if (kept && kept.after === after && kept.unread.count > 0) {
        byChannel.set(c.channel, kept.unread)
      }
      return
    }
    const mine = data.messages.filter((m) => m.from === me).at(-1)?.id ?? 0
    const unread = data.messages.filter((m) => m.id > Math.max(after, mine) && m.from !== me)
    scanned.set(
      c.channel,
      data.messages.filter((m) => m.id > after),
    )
    const value = { count: unread.length, mention: unread.some((m) => mentions(m.body, me)) }
    previous.set(c.channel, { after, unread: value })
    if (value.count > 0) {
      byChannel.set(c.channel, value)
    }
  })

  let total = 0
  let mention = false
  for (const u of byChannel.values()) {
    total += u.count
    mention ||= u.mention
  }

  return {
    me,
    inbox: `#${me}`,
    channels,
    channelsQuery,
    of: (channel) => byChannel.get(channel) ?? NONE,
    total,
    mention,
    scanned,
  }
}

// Which channels the signed-in person follows, kept in the browser like the
// read cursors (unread.ts). Only followed channels are in the Talk sidebar.
// localStorage agc.panel.follow maps "#name" to { on, at }: on is whether it
// is followed, at the newest message id that decision covers. A message
// newer than at that is addressed to me (it mentions @<me>, or it is in one
// of my DMs) or that I posted follows the channel again, so unfollowing a
// channel I am mentioned in lasts until the next mention. Opening a channel
// follows it. My inbox #<me> is always followed; a DM of mine (@<me>/<task>)
// is followed until I unfollow it.
import { useEffect, useSyncExternalStore } from 'react'

import type { LiveEvent } from '@/api/events'

import { useMyName } from './identity'
import { addressesMe, isDMOf, useUnread, type Unread } from './unread'

export { isDMOf }

export const FOLLOW_KEY = 'agc.panel.follow'

export type FollowEntry = { on: boolean; at: number }
type FollowMap = Record<string, FollowEntry>

function load(): FollowMap {
  try {
    const parsed = JSON.parse(localStorage.getItem(FOLLOW_KEY) ?? '{}') as unknown
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as FollowMap) : {}
  } catch {
    return {}
  }
}

let followMap = load()
const listeners = new Set<() => void>()

function notify() {
  for (const listener of listeners) {
    listener()
  }
}

// Another tab followed or unfollowed a channel.
function onStorage(event: StorageEvent) {
  if (event.key === FOLLOW_KEY || event.key === null) {
    followMap = load()
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

function save(channel: string, entry: FollowEntry) {
  followMap = { ...followMap, [channel]: entry }
  try {
    localStorage.setItem(FOLLOW_KEY, JSON.stringify(followMap))
  } catch {
    // Storage disabled: following lasts for this page only.
  }
  notify()
}

// The task half of a DM's name, "@franco/yc-pitch" → "yc-pitch".
export function dmTask(channel: string) {
  return channel.slice(channel.indexOf('/') + 1)
}

// The route of a channel: "#review-42" → /messages/review-42, a DM keeps
// its @ ("@franco/yc-pitch" → /messages/%40franco%2Fyc-pitch).
export function channelPath(channel: string) {
  return `/messages/${encodeURIComponent(channel.replace(/^#/, ''))}`
}

// The channel of a /messages/:channel param: a DM as is, else with its #.
export function channelOfSlug(slug: string) {
  return slug.startsWith('@') ? slug : `#${slug}`
}

export function isFollowing(map: FollowMap, channel: string, me: string) {
  if (channel === `#${me}`) {
    return true
  }
  return map[channel]?.on ?? isDMOf(channel, me)
}

// Opening a channel follows it, covering every message up to at.
export function follow(channel: string, at = 0) {
  const entry = followMap[channel]
  if (entry?.on) {
    return
  }
  save(channel, { on: true, at: Math.max(at, entry?.at ?? 0) })
}

// The header and Browse toggle. at is the channel's newest message id, so
// only a newer mention follows it again.
export function unfollow(channel: string, at: number) {
  save(channel, { on: false, at: Math.max(at, followMap[channel]?.at ?? 0) })
}

// A message with this id addressed to me, or mine, arrived in channel.
// Nothing changes for one an unfollow already covered.
export function autoFollow(channel: string, id: number) {
  const entry = followMap[channel]
  if (entry && (entry.on || entry.at >= id)) {
    return
  }
  save(channel, { on: true, at: id })
}

// The message.posted half, wired next to the notifications in the shell.
export function followFromEvent(event: LiveEvent, me: string) {
  if (event.type !== 'message.posted') {
    return
  }
  if (addressesMe(event.message, me)) {
    autoFollow(event.message.channel, event.message.id)
  }
}

export function useFollowMap() {
  return useSyncExternalStore(subscribe, () => followMap)
}

export type Follow = {
  unread: Unread
  following: (channel: string) => boolean
  // Unread across followed channels plus my inbox, my DMs and anything
  // mentioning me: the Talk mode badge.
  total: number
  mention: boolean
}

// Following plus the unread state it filters. Every channel the unread scan
// found addressed to me past its cursor is followed (autoFollow keeps it
// idempotent, so several mounts are harmless).
export function useFollow(): Follow {
  const me = useMyName()
  const map = useFollowMap()
  const unread = useUnread()
  const { addressed } = unread

  useEffect(() => {
    for (const [channel, id] of addressed) {
      autoFollow(channel, id)
    }
  }, [addressed])

  const following = (channel: string) => isFollowing(map, channel, me)
  let total = 0
  let mention = false
  for (const c of unread.channels) {
    const u = unread.of(c.channel)
    if (u.count > 0 && (following(c.channel) || u.mention || isDMOf(c.channel, me))) {
      total += u.count
      mention ||= u.mention
    }
  }
  return { unread, following, total, mention }
}

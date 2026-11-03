// Which channels the signed-in person follows, kept in the browser like the
// read cursors (unread.ts). Only followed channels are in the Talk sidebar;
// every channel is in Browse.
//
// localStorage agc.panel.follow maps "#name" to { on, at, done }: on is
// whether it is followed, at the newest message id that decision covers,
// done that the channel already fell away once because its task is done.
//
// The rule, in plain words. A channel follows itself only when:
//   - I open it, or I post in it;
//   - a person posts in it and mentions @<me>;
//   - an agent asks me a question there: a message of kind question that
//     mentions @<me>, or is in my inbox #<me> or one of my DMs @<me>/<task>.
// Nothing agc posts ever follows a channel, even when it names me (every
// task's opening message names its owner); an agent's plain message does
// not either, even one that mentions me: it still toasts, but only a
// question asks for me. Only messages newer than at count, so unfollowing
// lasts until the next one of these.
//
// My inbox #<me> is always followed; a DM of mine is followed until I
// unfollow it. When a task is done its channel is unfollowed once (done),
// unless an open question there still waits for me; opening it follows it
// again, and it stays until I unfollow it.
import { useEffect, useSyncExternalStore } from 'react'

import type { LiveEvent } from '@/api/events'
import { useTasks } from '@/api/hooks'
import type { Message } from '@/api/msg'

import { useMyName } from './identity'
import { useOpenQuestions } from './questions'
import { isDMOf, mentions, useUnread, type Unread } from './unread'

export { isDMOf }

export const FOLLOW_KEY = 'agc.panel.follow'

export type FollowEntry = { on: boolean; at: number; done?: boolean }
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
  save(channel, { ...entry, on: true, at: Math.max(at, entry?.at ?? 0) })
}

// The header and Browse toggle. at is the channel's newest message id, so
// only a newer mention follows it again.
export function unfollow(channel: string, at: number) {
  const entry = followMap[channel]
  save(channel, { ...entry, on: false, at: Math.max(at, entry?.at ?? 0) })
}

// The participant agc posts as (msgmodels.SystemName).
export const SYSTEM = 'agc'

type Followable = Pick<Message, 'channel' | 'from' | 'body' | 'kind' | 'human' | 'task'>

// A person posted it. Servers that predate human: no task and not agc.
function byPerson(message: Followable) {
  return message.human ?? (message.task === '' && message.from !== SYSTEM)
}

// Whether message follows its channel for me: the rule at the top.
export function followsMe(message: Followable, me: string) {
  if (message.from === me) {
    return true
  }
  if (message.from === SYSTEM && !message.human) {
    return false
  }
  if (byPerson(message)) {
    return mentions(message.body, me)
  }
  const toMe = mentions(message.body, me) || message.channel === `#${me}` || isDMOf(message.channel, me)
  return message.kind === 'question' && toMe
}

// A message with this id that follows the channel for me arrived in it.
// Nothing changes for one an unfollow already covered.
export function autoFollow(channel: string, id: number) {
  const entry = followMap[channel]
  if (entry && (entry.on || entry.at >= id)) {
    return
  }
  save(channel, { ...entry, on: true, at: id })
}

// A done task's channel falls away, once: at covers every message so far,
// so only a newer one that follows (or opening it) brings it back.
export function dropDone(channel: string, at: number) {
  const entry = followMap[channel]
  if (entry?.done) {
    return
  }
  save(channel, { on: false, at: Math.max(at, entry?.at ?? 0), done: true })
}

// The message.posted half, wired next to the notifications in the shell.
export function followFromEvent(event: LiveEvent, me: string) {
  if (event.type !== 'message.posted') {
    return
  }
  if (followsMe(event.message, me)) {
    autoFollow(event.message.channel, event.message.id)
  }
}

export function useFollowMap() {
  return useSyncExternalStore(subscribe, () => followMap)
}

export type Follow = {
  unread: Unread
  following: (channel: string) => boolean
  // Unread messages in the channels I follow (my inbox and DMs included):
  // the Talk group badge, next to the open questions for me.
  total: number
  mention: boolean
}

// Following plus the unread state it filters. Every channel whose unread
// scan holds a message that follows it for me is followed (autoFollow keeps
// it idempotent, so several mounts are harmless), and every done task's
// channel without an open question for me falls away once.
export function useFollow(): Follow {
  const me = useMyName()
  const map = useFollowMap()
  const unread = useUnread()
  const tasks = useTasks()
  const questions = useOpenQuestions()
  const { scanned, channels, channelsQuery } = unread

  useEffect(() => {
    for (const [channel, messages] of scanned) {
      const last = messages.filter((m) => followsMe(m, me)).at(-1)
      if (last) {
        autoFollow(channel, last.id)
      }
    }
  }, [scanned, me])

  const done = (tasks.data ?? []).filter((t) => t.status === 'done').map((t) => `#${t.name}`)
  const ready = questions.loaded && channelsQuery.isSuccess
  useEffect(() => {
    if (!ready) {
      return
    }
    for (const channel of done) {
      if (questions.of(channel).length > 0) {
        continue
      }
      dropDone(channel, channels.find((c) => c.channel === channel)?.last_id ?? 0)
    }
  })

  const following = (channel: string) => isFollowing(map, channel, me)
  let total = 0
  let mention = false
  for (const c of channels) {
    const u = unread.of(c.channel)
    if (u.count > 0 && following(c.channel)) {
      total += u.count
      mention ||= u.mention
    }
  }
  return { unread, following, total, mention }
}

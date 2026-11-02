// Whether the GET /api/events stream is connected, as a tiny external
// store: src/api/events.ts writes it, the top bar's dot and the query
// polling read it. Its own module so hooks.ts and events.ts don't import
// each other.
import { useSyncExternalStore } from 'react'

// Polling is the fallback: every 5s without the stream, every 30s with it
// (the events invalidate queries as things change).
export const POLL_MS = 5000
export const LIVE_POLL_MS = 30_000

let connected = false
const listeners = new Set<() => void>()

export function setLiveConnected(value: boolean) {
  if (value === connected) {
    return
  }
  connected = value
  for (const listener of listeners) {
    listener()
  }
}

export function isLiveConnected() {
  return connected
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function useLiveConnected() {
  return useSyncExternalStore(subscribe, isLiveConnected)
}

// refetchInterval for queries the events keep fresh. TanStack reads it
// again after every fetch, so a change applies from the next one.
export function livePollMs() {
  return connected ? LIVE_POLL_MS : POLL_MS
}

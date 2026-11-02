// The live feed, GET /api/events: one EventSource for the whole app,
// opened by the shell (useLiveEvents in src/shell/layout.tsx). Every event
// invalidates the queries it changes, so each page refreshes itself; the
// shell also turns task.created and task.done into notifications.
//
// Server-sent events, each `event: <type>`, `data: <json>` and `id: <n>`,
// plus a comment every 20s as a heartbeat:
//   task.created    { task }
//   task.done       { task, summary }   summary: the agent's closing message in #<task>, may be ""
//   sandbox.status  { sandbox }         every status change; status "deleted" on delete
//   message.posted  { message }
//   agent.state     { agent }           every agent state change: wake (waking, then awake) and sleep
import { useEffect, useRef } from 'react'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'

import { addPostedMessage, queryKeys } from './hooks'
import { isLiveConnected, setLiveConnected } from './live'
import type { Message } from './msg'
import type { SandboxView, TaskView } from './tasks'
import type { AgentView } from './agents'

export const EVENTS_URL = '/api/events'

export type LiveEvent =
  | { type: 'task.created'; task: TaskView }
  | { type: 'task.done'; task: TaskView; summary: string }
  | { type: 'sandbox.status'; sandbox: SandboxView }
  | { type: 'message.posted'; message: Message }
  | { type: 'agent.state'; agent: AgentView }

const TYPES: LiveEvent['type'][] = ['task.created', 'task.done', 'sandbox.status', 'message.posted', 'agent.state']

// When the browser gives up on the stream (an HTTP error, a wrong content
// type), reopen it after 1s, doubling up to 30s. Network drops are retried
// by EventSource itself.
const RETRY_MIN_MS = 1000
const RETRY_MAX_MS = 30_000

// Invalidates what the event changed. A task's status follows its
// sandboxes, so task and sandbox events refresh both roots (each root
// covers its single-item keys too).
function invalidate(queryClient: QueryClient, event: LiveEvent) {
  switch (event.type) {
    case 'task.created':
      void queryClient.invalidateQueries({ queryKey: queryKeys.tasks })
      void queryClient.invalidateQueries({ queryKey: queryKeys.sandboxes })
      // A new task has a new channel.
      void queryClient.invalidateQueries({ queryKey: queryKeys.channels })
      return
    case 'task.done':
    case 'sandbox.status':
      void queryClient.invalidateQueries({ queryKey: queryKeys.tasks })
      void queryClient.invalidateQueries({ queryKey: queryKeys.sandboxes })
      return
    case 'message.posted':
      addPostedMessage(queryClient, event.message)
      void queryClient.invalidateQueries({ queryKey: queryKeys.channels })
      void queryClient.invalidateQueries({ queryKey: queryKeys.participants(event.message.channel) })
      return
    // An agent's state follows its home sandbox: refresh both roots.
    case 'agent.state':
      void queryClient.invalidateQueries({ queryKey: queryKeys.agents })
      void queryClient.invalidateQueries({ queryKey: queryKeys.sandboxes })
      return
  }
}

// Everything the stream keeps fresh, refetched after a gap in it.
function invalidateAll(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: queryKeys.tasks })
  void queryClient.invalidateQueries({ queryKey: queryKeys.sandboxes })
  void queryClient.invalidateQueries({ queryKey: queryKeys.channels })
  void queryClient.invalidateQueries({ queryKey: queryKeys.agents })
}

function parse(type: LiveEvent['type'], data: string): LiveEvent | null {
  try {
    return { type, ...(JSON.parse(data) as object) } as LiveEvent
  } catch {
    return null
  }
}

// Opens the stream for as long as the calling component is mounted (the
// shell). onEvent sees every event after its queries are invalidated.
export function useLiveEvents(onEvent?: (event: LiveEvent) => void) {
  const queryClient = useQueryClient()
  const onEventRef = useRef(onEvent)
  useEffect(() => {
    onEventRef.current = onEvent
  })

  useEffect(() => {
    let source: EventSource | null = null
    let retry: number | undefined
    let delay = RETRY_MIN_MS
    // Set after a drop: the reopened stream may have missed events.
    let gap = false
    let closed = false

    function open() {
      source = new EventSource(EVENTS_URL)
      source.onopen = () => {
        delay = RETRY_MIN_MS
        setLiveConnected(true)
        if (gap) {
          gap = false
          invalidateAll(queryClient)
        }
      }
      source.onerror = () => {
        if (isLiveConnected()) {
          // Back to 5s polling at once, not after the next 30s poll.
          invalidateAll(queryClient)
        }
        setLiveConnected(false)
        gap = true
        if (source?.readyState === EventSource.CLOSED && !closed) {
          source = null
          retry = window.setTimeout(open, delay)
          delay = Math.min(delay * 2, RETRY_MAX_MS)
        }
      }
      for (const type of TYPES) {
        source.addEventListener(type, (message) => {
          const event = parse(type, (message as MessageEvent<string>).data)
          if (!event) {
            return
          }
          invalidate(queryClient, event)
          onEventRef.current?.(event)
        })
      }
    }

    open()
    return () => {
      closed = true
      window.clearTimeout(retry)
      source?.close()
      setLiveConnected(false)
    }
  }, [queryClient])
}

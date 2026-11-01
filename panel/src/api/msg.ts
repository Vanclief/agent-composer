// Messaging, /api/msg/*. Not in docs/openapi.yaml, so typed by hand from
// messaging/httpapi/httpapi.go and models/messaging (ADR 0005). Errors are
// {"error":"..."}; apiFetch turns them into ApiError with that text.
import { apiFetch } from './client'

// A name agc handed out on join. The id is the identity; the name is only
// unique while the participant is active.
export type Participant = {
  id: string
  name: string
  // Empty for humans.
  task: string
  human: boolean
  joined_at: string
  left_at?: string
}

export type Message = {
  // Also the read cursor: ids are totally ordered per channel.
  id: number
  channel: string
  from: string
  participant_id: string
  // Empty when a human posted.
  task: string
  reply_to?: number
  body: string
  created_at: string
}

export type ChannelInfo = {
  channel: string
  read_only: boolean
  last_id: number
  count: number
}

// Answer to read and wait. cursor is the id to pass as `after` next time.
export type MessagesResponse = {
  messages: Message[]
  cursor: number
}

export type PostRequest = {
  participant_id: string
  // "#name"
  channel: string
  body: string
  reply_to?: number
}

export type ReadParams = {
  channel: string
  after?: number
  // Default 50, max 500.
  limit?: number
}

export type WaitParams = {
  channel: string
  after?: number
  // Go duration such as "25s". Default 30s.
  timeout?: string
}

function query(params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined) {
      search.set(key, String(value))
    }
  }
  return search.toString()
}

export const msg = {
  // Rejected with 409 "name @x is already in use" while someone holds it.
  join(name: string) {
    return apiFetch<Participant>('POST', '/msg/join', { name })
  },
  leave(participantID: string) {
    return apiFetch<{ participant_id: string; left: boolean }>('POST', '/msg/leave', {
      participant_id: participantID,
    })
  },
  post(request: PostRequest) {
    return apiFetch<Message>('POST', '/msg/post', request)
  },
  // Messages with id > after, oldest first.
  read(params: ReadParams, signal?: AbortSignal) {
    return apiFetch<MessagesResponse>('GET', '/msg/read?' + query(params), undefined, signal)
  },
  // Long poll: answers as soon as a message with id > after exists, or with
  // no messages when the timeout passes.
  wait(params: WaitParams, signal?: AbortSignal) {
    return apiFetch<MessagesResponse>('GET', '/msg/wait?' + query(params), undefined, signal)
  },
  channels() {
    return apiFetch<{ channels: ChannelInfo[] }>('GET', '/msg/channels')
  },
}

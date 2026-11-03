// Messaging, /api/msg/*. Not in docs/openapi.yaml, so typed by hand from
// messaging/httpapi/httpapi.go and models/messaging (ADR 0005). Errors are
// {"error":"..."}; apiFetch turns them into ApiError with that text.
// Routes: POST join, leave, post; GET read, wait, channels, participants,
// questions.
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
  // Presence, being added by the backend; optional until every server
  // sends it. last_seen_at is null when the participant was never seen.
  online?: boolean
  last_seen_at?: string | null
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
  // A question waits for an answer: a reply_to it from someone else.
  // Absent on servers that predate kinds; treat it as 'message'.
  kind?: MessageKind
  body: string
  created_at: string
  // A person posted it (not an agent, not agc). Absent on servers that
  // predate it.
  human?: boolean
}

export type MessageKind = 'message' | 'question'

// GET /api/msg/questions: a question with who answered it first.
// Hand-typed ahead of the Go side.
export type Question = Message & {
  // Empty while open.
  answered_by: string
  answered_at: string | null
}

export type QuestionsParams = {
  // "#name"; omitted: every channel.
  channel?: string
  // 1: only questions nobody answered yet.
  open?: 1
  // Only questions addressed to this person: in their inbox, or in task
  // channels they manage (not those they only own).
  for?: string
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
  kind?: MessageKind
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
  // Everyone who has posted in channel ("#name"): active first, then those
  // who left, each by name.
  participants(channel: string, signal?: AbortSignal) {
    return apiFetch<{ participants: Participant[] }>(
      'GET',
      '/msg/participants?' + query({ channel }),
      undefined,
      signal,
    )
  },
  questions(params: QuestionsParams, signal?: AbortSignal) {
    return apiFetch<{ questions: Question[] }>('GET', '/msg/questions?' + query(params), undefined, signal)
  },
  // GET /api/msg/participants?all=1: everyone who ever joined, in any
  // channel. Humans only (Service.AllParticipants).
  allParticipants(signal?: AbortSignal) {
    return apiFetch<{ participants: Participant[] }>('GET', '/msg/participants?all=1', undefined, signal)
  },
}

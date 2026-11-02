// What every message in an open channel needs to know: the channel, who
// posts, the threads, the questions' answered state, the clock, and how to
// open a thread. Provided by ChannelView.
import { createContext, useContext } from 'react'

import type { Question } from '@/api/msg'

import type { Identity } from './identity'
import type { Threads } from './threads'

export type ChannelContextValue = {
  channel: string
  identity: Identity
  me: string
  threads: Threads
  // From GET /api/msg/questions?channel=, by id.
  questions: Map<number, Question>
  now: number
  openThread: (id: number) => void
}

export const ChannelContext = createContext<ChannelContextValue | null>(null)

export function useChannel() {
  const value = useContext(ChannelContext)
  if (!value) {
    throw new Error('useChannel outside ChannelView')
  }
  return value
}

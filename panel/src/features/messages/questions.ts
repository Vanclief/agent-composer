// Questions (messages of kind question) and who answered them. A question
// is answered by the first reply_to it from someone other than the asker;
// GET /api/msg/questions says so for questions outside the loaded feed.
// The open questions addressed to me are counted next to the unread badges
// (QuestionBadge) and pinned at the top of their channel ("Waiting for
// you"). They refresh whenever a message is posted anywhere: the channels'
// last ids are part of the query key, like unread.ts.
import { useQuery } from '@tanstack/react-query'

import { queryKeys, useChannels } from '@/api/hooks'
import { livePollMs } from '@/api/live'
import { msg, type Message, type Question, type QuestionsParams } from '@/api/msg'

import { useMyName } from './identity'

export function isQuestion(m: Message) {
  return m.kind === 'question'
}

// The answer to question among its thread's replies: the first one from
// someone other than the asker.
export function answerOf(question: Message, replies: Message[]) {
  return replies.find((r) => r.participant_id !== question.participant_id)
}

function useVersion() {
  const channels = useChannels()
  return (channels.data ?? []).reduce((sum, c) => sum + c.last_id, 0)
}

export function useQuestions(params: QuestionsParams, enabled = true) {
  const version = useVersion()
  return useQuery({
    queryKey: queryKeys.questions(params, version),
    queryFn: async ({ signal }) => (await msg.questions(params, signal)).questions,
    enabled,
    refetchInterval: livePollMs,
    placeholderData: (previous) => previous,
  })
}

export type OpenQuestions = {
  all: Question[]
  of: (channel: string) => Question[]
  total: number
  // The first answer arrived (later refetches keep the previous one).
  loaded: boolean
}

// Open questions addressed to me, by channel. My own questions never wait
// for me.
export function useOpenQuestions(): OpenQuestions {
  const me = useMyName()
  const query = useQuestions({ open: 1, for: me })
  const all = (query.data ?? []).filter((q) => q.from !== me)
  return {
    all,
    of: (channel) => all.filter((q) => q.channel === channel),
    total: all.length,
    loaded: query.isSuccess,
  }
}

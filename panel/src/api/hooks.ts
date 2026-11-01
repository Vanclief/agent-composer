// react-query hooks for every API the panel uses. Query keys live in
// queryKeys only; invalidate through them, never with ad-hoc arrays.
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, unwrap, type CreateTaskRequest, type Task } from './client'
import { msg, type Message, type MessagesResponse, type PostRequest } from './msg'
import { terminals, type StartTerminalRequest } from './terminals'
import { knowledge } from './knowledge'
import { keepPreviousData } from '@tanstack/react-query'

// How often the tasks list refetches, so statuses stay live.
export const TASKS_POLL_MS = 5000
// First page of a channel, the wait timeout, and how many messages a
// channel keeps in memory.
const READ_LIMIT = 100
const WAIT_TIMEOUT = '25s'
const KEEP_MESSAGES = 500
const WAIT_RETRY_MS = 5000

// Every key is under its resource's root, so invalidating
// queryKeys.tasks also refreshes each task and its terminals.
export const queryKeys = {
  tasks: ['tasks'] as const,
  task: (ref: string) => ['tasks', ref] as const,
  terminals: (task: string) => ['tasks', task, 'terminals'] as const,
  specs: ['specs'] as const,
  spec: (name: string) => ['specs', name] as const,
  secrets: ['secrets'] as const,
  channels: ['msg', 'channels'] as const,
  messages: (channel: string) => ['msg', 'messages', channel] as const,
  knowledge: ['knowledge'] as const,
  why: (id: string) => ['knowledge', 'why', id] as const,
  doubt: (id: string) => ['knowledge', 'doubt', id] as const,
  knowledgeWithout: (without: string) => ['knowledge', 'without', without] as const,
  conflicts: ['knowledge', 'conflicts'] as const,
}

// ---------- tasks ----------

export function useTasks() {
  return useQuery({
    queryKey: queryKeys.tasks,
    queryFn: async () => (await unwrap(api.GET('/tasks'))).tasks,
    refetchInterval: TASKS_POLL_MS,
  })
}

// ref is a task name or UUID.
export function useTask(ref: string | undefined) {
  return useQuery({
    queryKey: queryKeys.task(ref ?? ''),
    queryFn: () => unwrap(api.GET('/tasks/{id}', { params: { path: { id: ref ?? '' } } })),
    enabled: Boolean(ref),
    refetchInterval: TASKS_POLL_MS,
  })
}

function useTaskMutation<TVariables, TData>(mutationFn: (variables: TVariables) => Promise<TData>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.tasks }),
  })
}

export function useCreateTask() {
  return useTaskMutation((request: CreateTaskRequest) => unwrap(api.POST('/tasks', { body: request })))
}

export function useStopTask() {
  return useTaskMutation((ref: string): Promise<Task> =>
    unwrap(api.POST('/tasks/{id}/stop', { params: { path: { id: ref } } })),
  )
}

export function useStartTask() {
  return useTaskMutation((ref: string): Promise<Task> =>
    unwrap(api.POST('/tasks/{id}/start', { params: { path: { id: ref } } })),
  )
}

export function useDeleteTask() {
  return useTaskMutation((ref: string) => unwrap(api.DELETE('/tasks/{id}', { params: { path: { id: ref } } })))
}

// ---------- saved specs ----------

export function useSpecs() {
  return useQuery({
    queryKey: queryKeys.specs,
    queryFn: async () => (await unwrap(api.GET('/specs'))).specs,
  })
}

export function useSpec(name: string | undefined) {
  return useQuery({
    queryKey: queryKeys.spec(name ?? ''),
    queryFn: () => unwrap(api.GET('/specs/{name}', { params: { path: { name: name ?? '' } } })),
    enabled: Boolean(name),
  })
}

// name may be free text; the response carries the slug it was saved as.
export function useSaveSpec() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, yaml }: { name: string; yaml: string }) =>
      unwrap(api.PUT('/specs/{name}', { params: { path: { name } }, body: { yaml } })),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.specs }),
  })
}

export function useDeleteSpec() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => unwrap(api.DELETE('/specs/{name}', { params: { path: { name } } })),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.specs }),
  })
}

// ---------- secrets ----------

export function useSecrets() {
  return useQuery({
    queryKey: queryKeys.secrets,
    queryFn: async () => (await unwrap(api.GET('/secrets'))).secrets,
  })
}

// ---------- terminals ----------

// Only a running task has terminals; pass enabled=false otherwise, or the
// query fails with the backend's "start it to use terminals" error.
export function useTerminals(task: string | undefined, enabled = true) {
  return useQuery({
    queryKey: queryKeys.terminals(task ?? ''),
    queryFn: async () => (await terminals.list(task ?? '')).terminals,
    enabled: Boolean(task) && enabled,
    refetchInterval: TASKS_POLL_MS,
  })
}

export function useStartTerminal(task: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (request: StartTerminalRequest) => terminals.start(task, request),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.terminals(task) }),
  })
}

// ---------- messaging ----------

export function useChannels() {
  return useQuery({
    queryKey: queryKeys.channels,
    queryFn: async () => (await msg.channels()).channels,
    refetchInterval: TASKS_POLL_MS,
  })
}

export function useJoin() {
  return useMutation({ mutationFn: (name: string) => msg.join(name) })
}

export function useLeave() {
  return useMutation({ mutationFn: (participantID: string) => msg.leave(participantID) })
}

// Appends messages to a channel's cache, skipping ids it already has
// (a post and the wait that reports it may both arrive).
function appendMessages(old: MessagesResponse | undefined, added: Message[], cursor: number): MessagesResponse {
  const messages = old?.messages ?? []
  const seen = new Set(messages.map((m) => m.id))
  const merged = [...messages, ...added.filter((m) => !seen.has(m.id))].slice(-KEEP_MESSAGES)
  return { messages: merged, cursor: Math.max(old?.cursor ?? 0, cursor) }
}

// Posting shows the message at once; the wait loop dedupes it.
export function usePostMessage() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (request: PostRequest) => msg.post(request),
    onSuccess: (message) => {
      queryClient.setQueryData<MessagesResponse>(queryKeys.messages(message.channel), (old) =>
        old ? appendMessages(old, [message], old.cursor) : old,
      )
    },
  })
}

// useChannelMessages is a channel's live feed: one read, then a wait long
// poll that appends new messages as they are posted. channel is "#name".
// error is the last read or wait failure, verbatim; the wait retries every
// 5s and clears it on success.
export function useChannelMessages(channel: string | undefined) {
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: queryKeys.messages(channel ?? ''),
    queryFn: ({ signal }) => msg.read({ channel: channel ?? '', limit: READ_LIMIT }, signal),
    enabled: Boolean(channel),
    // The wait loop below keeps it current.
    staleTime: Infinity,
  })
  const cursor = query.data?.cursor
  const [waitError, setWaitError] = useState<Error | null>(null)
  const [round, setRound] = useState(0)

  useEffect(() => {
    if (!channel || cursor === undefined) {
      return
    }
    const controller = new AbortController()
    let retry: number | undefined
    msg
      .wait({ channel, after: cursor, timeout: WAIT_TIMEOUT }, controller.signal)
      .then((response) => {
        setWaitError(null)
        if (response.messages.length === 0) {
          setRound((n) => n + 1)
          return
        }
        queryClient.setQueryData<MessagesResponse>(queryKeys.messages(channel), (old) =>
          appendMessages(old, response.messages, response.cursor),
        )
      })
      .catch((error: Error) => {
        if (controller.signal.aborted) {
          return
        }
        setWaitError(error)
        retry = window.setTimeout(() => setRound((n) => n + 1), WAIT_RETRY_MS)
      })
    return () => {
      controller.abort()
      window.clearTimeout(retry)
    }
  }, [channel, cursor, round, queryClient])

  return {
    messages: query.data?.messages ?? [],
    isPending: query.isPending,
    error: query.error ?? waitError,
  }
}

// ---------- knowledge ----------

// Evidence, beliefs, decisions and rules as the files in git say, with
// labels computed by the server.
export function useKnowledge() {
  return useQuery({
    queryKey: queryKeys.knowledge,
    queryFn: ({ signal }) => knowledge.get(signal),
  })
}

// id followed down its because chain to the evidence.
export function useWhy(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.why(id ?? ''),
    queryFn: ({ signal }) => knowledge.why(id ?? '', signal),
    enabled: Boolean(id),
  })
}

// What rests on id, a belief or a piece of evidence.
export function useDoubt(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.doubt(id ?? ''),
    queryFn: ({ signal }) => knowledge.doubt(id ?? '', signal),
    enabled: Boolean(id),
  })
}

// The knowledge as the files say, or judged as if the belief or evidence
// without were gone (a what-if; nothing is written). The previous answer
// stays on screen while the next one loads, so the page does not flash.
export function useKnowledgeView(without: string | undefined) {
  return useQuery({
    queryKey: without ? queryKeys.knowledgeWithout(without) : queryKeys.knowledge,
    queryFn: ({ signal }) => knowledge.get(signal, without ? [without] : undefined),
    placeholderData: keepPreviousData,
  })
}

// Contested contradictions, each side with what it rests on.
export function useConflicts() {
  return useQuery({
    queryKey: queryKeys.conflicts,
    queryFn: ({ signal }) => knowledge.conflicts(signal),
  })
}

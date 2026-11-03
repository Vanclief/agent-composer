// Persistent agents (the ADR is being written): an agent is born numbered
// from a role (reviewer-3), and Franco keeps the ones worth keeping by
// giving them a name (ada). Hand-typed from the proposal until the Go side
// and the OpenAPI spec exist; then these can become aliases of
// components['schemas'].
import { apiFetch } from './client'

// Read from the home sandbox by the server: awake while it runs, asleep
// while stopped, waking while it starts. Show it verbatim.
export type AgentState = 'awake' | 'asleep' | 'waking'

export type AgentTask = {
  // The task's name, also its channel.
  name: string
  // The task's status, verbatim.
  status: string
}

export type AgentConversation = {
  // "#name"; the agent's DM with its owner comes first.
  channel: string
  last_at: string
}

// A rule the agent's template names, from the knowledge dir (ADR 0016 item 2).
export type AgentRule = {
  id: string
  // The rule's text as written.
  rule: string
}

export type AgentView = {
  // Unique: numbered from the role (reviewer-3) until kept, then its name.
  name: string
  // The template it was made from (Reviewer, Orchestrator, Writer), free text.
  role: string
  // False while numbered, true once named.
  kept: boolean
  harness: string
  model: string
  owner: string
  state: AgentState
  // The home sandbox's name; null before the first wake.
  home: string | null
  // The rules it follows, in template order; the mock does not serve them yet.
  rules?: AgentRule[]
  // The tasks it manages.
  tasks: AgentTask[]
  // The channels it reads, the DM with the owner first.
  conversations: AgentConversation[]
  last_active_at: string | null
  created_at: string
}

export type CreateAgentRequest = {
  role: string
  // Empty takes the server's defaults (ADR 0009).
  harness?: string
  model?: string
}

export type KeepAgentRequest = {
  // Lowercase letters, digits and dashes.
  name: string
  // Omitted keeps the role.
  role?: string
}

const enc = encodeURIComponent

export const agents = {
  list(signal?: AbortSignal) {
    return apiFetch<{ agents: AgentView[] }>('GET', '/agents', undefined, signal)
  },
  get(name: string, signal?: AbortSignal) {
    return apiFetch<AgentView>('GET', `/agents/${enc(name)}`, undefined, signal)
  },
  // Returns the numbered agent, asleep, with no home yet.
  create(request: CreateAgentRequest) {
    return apiFetch<AgentView>('POST', '/agents', request)
  },
  // Names the agent (or renames a kept one) and may change its role.
  keep(name: string, request: KeepAgentRequest) {
    return apiFetch<AgentView>('POST', `/agents/${enc(name)}/keep`, request)
  },
  // asleep → waking → awake.
  wake(name: string) {
    return apiFetch<AgentView>('POST', `/agents/${enc(name)}/wake`)
  },
  // awake → asleep.
  sleep(name: string) {
    return apiFetch<AgentView>('POST', `/agents/${enc(name)}/sleep`)
  },
}

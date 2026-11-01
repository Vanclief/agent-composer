// Knowledge (ADR 0006): evidence, beliefs, decisions and rules, read from
// the YAML files in git. Hand-typed: /api/knowledge is not in openapi.yaml
// yet. Everything is read-only; labels are computed by the server each time.
import { apiFetch } from './client'

// How a belief, decision or rule is grounded, verbatim from the server.
export type KnowledgeLabel = 'grounded' | 'opinion' | 'in_doubt'

export type Retraction = { who: string; when: string; why: string }

export type Evidence = {
  id: string
  text: string
  who: string
  when: string
  // URL, file path or command; may be empty.
  source: string
  retracted: Retraction | null
  // Ids of the beliefs that cite this evidence.
  cited_by: string[]
  file: string
}

export type Belief = {
  id: string
  text: string
  because: string[]
  evidence: string[]
  label: KnowledgeLabel
  // Leaves of the because tree with no evidence, and with only retracted
  // evidence (and that evidence).
  opinions: string[]
  doubted: string[]
  retracted: string[]
  file: string
}

export type Decision = {
  id: string
  text: string
  because: string[]
  adr: string
  decided: { who: string; when: string }
  label: KnowledgeLabel
  opinions: string[]
  doubted: string[]
  file: string
}

export type Rule = {
  subject: string
  id: string
  text: string
  // Shell command run by `agc rules check`; empty for a manual rule.
  check: string
  because: string[]
  kind: 'mechanical' | 'manual'
  label: KnowledgeLabel
  opinions: string[]
  doubted: string[]
  file: string
}

export type Knowledge = {
  // Where the files were read from.
  dir: string
  evidence: Evidence[]
  beliefs: Belief[]
  decisions: Decision[]
  rules: Rule[]
}

export type WhyEvidence = {
  id: string
  text: string
  who: string
  when: string
  retracted: Retraction | null
}

// An item followed down its because chain to the evidence.
export type WhyNode = {
  id: string
  text: string
  evidence: WhyEvidence[]
  because: WhyNode[]
}

// Everything resting on a belief or a piece of evidence.
export type Doubted = {
  beliefs: Belief[]
  decisions: Decision[]
  rules: Rule[]
}

export const knowledge = {
  get: (signal?: AbortSignal) => apiFetch<Knowledge>('GET', '/knowledge', undefined, signal),
  why: (id: string, signal?: AbortSignal) =>
    apiFetch<WhyNode>('GET', `/knowledge/why/${encodeURIComponent(id)}`, undefined, signal),
  doubt: (id: string, signal?: AbortSignal) =>
    apiFetch<Doubted>('GET', `/knowledge/doubt/${encodeURIComponent(id)}`, undefined, signal),
}

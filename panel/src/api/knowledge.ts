// Knowledge (ADR 0006): evidence, beliefs, decisions, rules and actions,
// read from the YAML files in git, typed from docs/openapi.yaml.
// Everything is read-only; labels are computed by the server each time.
import { api, unwrap } from './client'
import type { components } from './schema'

type Schemas = components['schemas']

// How a belief, decision, rule or action is grounded, verbatim from the
// server.
export type KnowledgeLabel = Schemas['KnowledgeLabel']
export type Retraction = Schemas['Retraction']
export type Observation = Schemas['KnowledgeObservation']
export type Evidence = Schemas['Evidence']
export type Belief = Schemas['Belief']
export type Decision = Schemas['Decision']
export type Rule = Schemas['Rule']
// Something to do because of a decision; open while done is null.
export type Action = Schemas['KnowledgeAction']
// Everything loaded; without echoes the ids set aside with ?without=.
export type Knowledge = Schemas['KnowledgeSet']

// An item followed down its because chain to the evidence. or holds the
// other sets of reasons; evidence in one is a node of kind evidence with
// who, when, translated_by and retracted set.
export type WhyNode = Schemas['Reason']
export type WhyKind = WhyNode['kind']
export type WhyEvidence = WhyNode['evidence'][number]

// Everything resting on a belief, piece of evidence, decision or rule;
// standing rests on it in some sets of reasons but still has another.
export type Doubted = Schemas['Doubted']
export type Standing = Schemas['KnowledgeStanding']

// A contested contradiction, with the live evidence and opinions each side
// comes down to. The server never picks a side.
export type Conflict = Schemas['KnowledgeConflict']
export type ConflictSide = Schemas['KnowledgeConflictSide']

export const knowledge = {
  get: (signal?: AbortSignal, without?: string[]) =>
    unwrap(
      api.GET('/knowledge', {
        params: { query: without && without.length > 0 ? { without: without.join(',') } : {} },
        signal,
      }),
    ),
  why: (id: string, signal?: AbortSignal) =>
    unwrap(api.GET('/knowledge/why/{id}', { params: { path: { id } }, signal })),
  doubt: (id: string, signal?: AbortSignal) =>
    unwrap(api.GET('/knowledge/doubt/{id}', { params: { path: { id } }, signal })),
  conflicts: (signal?: AbortSignal) => unwrap(api.GET('/knowledge/conflicts', { signal })),
}

// GET /api/defaults (ADR 0009): what fills in a task when it leaves a field
// empty, the server's owner (the human the panel signs in as by default)
// and the reserved system participant. Typed ahead of the Go side.
import { apiFetch } from './client'

export type Defaults = {
  harness: string
  model: string
  owner: string
  system: string
}

export const defaults = {
  get(signal?: AbortSignal) {
    return apiFetch<Defaults>('GET', '/defaults', undefined, signal)
  },
}

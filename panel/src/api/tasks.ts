// Tasks and sandboxes (ADR 0008). A task is work to be done, in words; a
// sandbox is the compute a task is allocated, one microsandbox VM.
// Hand-typed from the ADR until `npm run types` regenerates schema.d.ts with
// these routes; then these can become aliases of components['schemas'].
import { apiFetch, type TaskSpec } from './client'

// Computed by the server: done when done_at is set, otherwise running when
// one of its sandboxes runs, otherwise open. Show it verbatim.
export type TaskStatus = 'open' | 'running' | 'done'

// starting until booted, failed with status_error when the boot failed,
// otherwise the runtime's word. Show it verbatim.
export type SandboxStatus = 'starting' | 'failed' | 'running' | 'stopped' | 'unknown'

export type SandboxView = {
  id: string
  task_id: string
  // The task's name.
  task: string
  // The first sandbox is named after its task, later ones <task>-2, <task>-3.
  name: string
  // The environment block: image, cpus, memory, repo, ref, setup, secrets,
  // env, volumes, channels.
  spec: TaskSpec
  // The program the sandbox runs as its first terminal (job "agent"); empty
  // when none.
  command: string
  // ADR 0009: the exact TASK.md the agent received, verbatim.
  instructions: string
  status: SandboxStatus
  // Why the status is what it is, e.g. the boot error of a failed sandbox.
  status_error?: string
  booted_at: string | null
  stopped_at: string | null
  created_at: string
}

export type TaskView = {
  id: string
  title: string
  // Slug derived from the title; also the channel #<name>.
  name: string
  // What to do and what done looks like, markdown.
  brief: string
  // A harness id such as claude_code or codex_cli, or a person's name;
  // empty means unassigned.
  assignee: string
  // ADR 0009: the harness model when the assignee is a harness, else empty.
  model: string
  // ADR 0009: who created the task and runs it day to day (an agent or a
  // person), and the human it answers to; agc addresses both when it opens.
  manager: string
  owner: string
  // Id of the knowledge action or decision the task serves, or empty.
  for: string
  status: TaskStatus
  done_at: string | null
  created_at: string
  updated_at: string
  // Newest first.
  sandboxes: SandboxView[]
}

// The environment block inline, plus the command to run once booted.
export type SandboxRequest = TaskSpec & { command?: string }

export type CreateTaskRequest = {
  title: string
  // Defaults to the slug of the title.
  name?: string
  brief: string
  // Empty fields take the server's configured defaults (ADR 0009):
  // assignee claude_code, model claude-opus-5-5, owner from config.
  // manager is the creator's messaging name; the panel sends the person's.
  assignee: string
  model?: string
  manager: string
  owner?: string
  for: string
  // Allocates a sandbox right after the task is written.
  sandbox?: SandboxRequest
}

export type DeleteResponse = { deleted: boolean; name: string }

const enc = encodeURIComponent

// ref is an id or a name everywhere.
export const tasks = {
  list(signal?: AbortSignal) {
    return apiFetch<{ tasks: TaskView[] }>('GET', '/tasks', undefined, signal)
  },
  get(ref: string, signal?: AbortSignal) {
    return apiFetch<TaskView>('GET', `/tasks/${enc(ref)}`, undefined, signal)
  },
  // Returns at once; a sandbox in the request comes back as starting.
  create(request: CreateTaskRequest) {
    return apiFetch<TaskView>('POST', '/tasks', request)
  },
  done(ref: string) {
    return apiFetch<TaskView>('POST', `/tasks/${enc(ref)}/done`)
  },
  // 201 with the new sandbox, status starting.
  allocate(ref: string, request: SandboxRequest) {
    return apiFetch<SandboxView>('POST', `/tasks/${enc(ref)}/sandboxes`, request)
  },
  // Deletes the task's sandboxes, then the task.
  delete(ref: string) {
    return apiFetch<DeleteResponse>('DELETE', `/tasks/${enc(ref)}`)
  },
}

export const sandboxes = {
  list(signal?: AbortSignal) {
    return apiFetch<{ sandboxes: SandboxView[] }>('GET', '/sandboxes', undefined, signal)
  },
  get(ref: string, signal?: AbortSignal) {
    return apiFetch<SandboxView>('GET', `/sandboxes/${enc(ref)}`, undefined, signal)
  },
  stop(ref: string) {
    return apiFetch<SandboxView>('POST', `/sandboxes/${enc(ref)}/stop`)
  },
  start(ref: string) {
    return apiFetch<SandboxView>('POST', `/sandboxes/${enc(ref)}/start`)
  },
  delete(ref: string) {
    return apiFetch<DeleteResponse>('DELETE', `/sandboxes/${enc(ref)}`)
  },
}

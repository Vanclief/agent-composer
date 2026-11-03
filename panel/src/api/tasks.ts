// Tasks and sandboxes (ADR 0008). A task is work to be done, in words; a
// sandbox is the compute a task is allocated, one microsandbox VM.
// Hand-typed from the ADR until `npm run types` regenerates schema.d.ts with
// these routes; then these can become aliases of components['schemas'].
import { apiFetch, type TaskSpec } from './client'

// Computed by the server (ADR 0010 item 3): done when done_at is set, else
// review when delivered_at is set, else running when one of its sandboxes
// runs, else open. Show it verbatim.
export type TaskStatus = 'open' | 'running' | 'review' | 'done'

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
  // The named volume agc-<name> with the harness homes and their
  // transcripts, kept after delete; empty for sandboxes booted before
  // volumes (ADR 0008 "Disk and transcripts").
  volume: string
  status: SandboxStatus
  // Why the status is what it is, e.g. the boot error of a failed sandbox.
  status_error?: string
  booted_at: string | null
  stopped_at: string | null
  created_at: string
  // What stopping or deleting would lose, read live from the clone (ADR
  // 0008 item 6). GET /sandboxes/{id} sets it, and the list with ?work=1,
  // when the sandbox is running and has a repo; null otherwise or when the
  // check failed.
  work: SandboxWork | null
}

export type SandboxWork = {
  // The clone has uncommitted changes.
  dirty: boolean
  // Commits on no remote branch, with or without an upstream.
  unpushed: number
  checked_at: string
}

// The agent's answer to a criterion.
export type Met = 'yes' | 'no' | 'partly'

// What the agent says it delivered, read against the task's accept lines.
export type Report = {
  // One per task.accept line, same order.
  criteria: { met: Met; note: string }[]
  // The branch and commit the work is on; absent when there is no repo work.
  changes?: { branch: string; sha: string; files?: number }
  checks: { name: string; passed: boolean; note: string }[]
  left_out: { what: string; why: string }[]
  // What waits on the reviewer.
  needs: string[]
  // Screenshots, before and after for a change a person sees; url is where
  // agc serves each (root-relative /api/tasks/<task>/shots/<label>.png
  // unless AGC_PUBLIC_URL is set). Absent when the delivery has none.
  shots?: { label: string; file: string; url?: string }[]
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
  // Whether the panel notifies (toast and sound) when the task starts and
  // when it is done.
  notify: boolean
  status: TaskStatus
  done_at: string | null
  // ADR 0010: what accepted means, one criterion per entry, in order; agc
  // does not evaluate them. May be empty.
  accept: string[]
  // When the agent last delivered (agc task deliver); null when it has not,
  // or a reject sent it back.
  delivered_at: string | null
  // The last delivery's summary; kept after a reject.
  summary: string
  // The last delivery's structured report (agc task deliver --report);
  // null when the agent delivered a summary alone or has not delivered.
  report: Report | null
  // Who accepted the task; empty until done.
  accepted_by: string
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
  manager?: string
  owner?: string
  for: string
  // Toast and sound when the task starts and is done; default true.
  notify?: boolean
  // ADR 0010: the acceptance criteria, one per entry.
  accept?: string[]
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
  // ADR 0010 item 2: the agent says the work is ready for review.
  deliver(ref: string, summary: string) {
    return apiFetch<TaskView>('POST', `/tasks/${enc(ref)}/deliver`, { summary })
  },
  // Sets done_at and accepted_by, then stops the task's running sandboxes.
  accept(ref: string, by: string) {
    return apiFetch<TaskView>('POST', `/tasks/${enc(ref)}/accept`, { by })
  },
  // Clears delivered_at and sends the task back to its agent with why.
  reject(ref: string, by: string, why: string) {
    return apiFetch<TaskView>('POST', `/tasks/${enc(ref)}/reject`, { by, why })
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

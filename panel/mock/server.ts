// Mock agc API for developing the panel without agc or a VM.
//
//   npm run mock                 # http://127.0.0.1:8790/api
//   npm run mock -- --port 9000
//
// Everything lives in memory and resets on restart. Shapes and error
// messages follow the real handlers: tasks, specs and secrets answer errors
// as {"error":{"code","message","request_id"}}, messaging (/api/msg/*) as
// {"error":"..."}. Tasks and sandboxes follow ADR 0008: a task is work in
// words, a sandbox the compute it is allocated. The terminal WebSocket
// echoes what you type and answers resize frames, so a terminal can be
// exercised end to end.
import { randomBytes, randomUUID } from 'node:crypto'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import { pathToFileURL } from 'node:url'

import { WebSocketServer, type WebSocket } from 'ws'

type TaskSpec = {
  image?: string
  cpus?: number
  memory?: number
  repo?: string
  ref?: string
  setup?: string
  secrets?: string[]
  env?: Record<string, string>
  volumes?: string[]
  channels?: string[]
}

// A task row; status is computed in taskView, never stored.
type Task = {
  id: string
  title: string
  name: string
  brief: string
  assignee: string
  // ADR 0009: the harness model when the assignee is a harness; the creator
  // who manages the task day to day; the human it answers to.
  model: string
  manager: string
  owner: string
  for: string
  // Whether the panel notifies (toast and sound) when the task starts and
  // when it is done.
  notify: boolean
  done_at: string | null
  // ADR 0010: criteria, the last delivery and who accepted.
  accept: string[]
  delivered_at: string | null
  summary: string
  // The last delivery's report; null with a summary alone.
  report: Report | null
  accepted_by: string
  created_at: string
  updated_at: string
}

type Report = {
  criteria: { met: 'yes' | 'no' | 'partly'; note: string }[]
  changes?: { branch: string; sha: string; files?: number }
  checks: { name: string; passed: boolean; note: string }[]
  left_out: { what: string; why: string }[]
  needs: string[]
}

// A sandbox row plus the runtime's word for it (running, stopped,
// unknown), which the real server reads live.
type Sandbox = {
  id: string
  task_id: string
  name: string
  spec: TaskSpec
  command: string
  // ADR 0009: the exact TASK.md text written into the sandbox.
  instructions: string
  boot_error: string | null
  booted_at: string | null
  stopped_at: string | null
  created_at: string
  runtime: 'running' | 'stopped' | 'unknown'
}

type SandboxView = Omit<Sandbox, 'boot_error' | 'runtime'> & {
  task: string
  status: string
  status_error?: string
  work: SandboxWork | null
}

// What stopping or deleting would lose, read from the clone (ADR 0008
// item 6). Only GET /api/sandboxes/:id and the list with ?work=1 set it.
type SandboxWork = { dirty: boolean; unpushed: number; checked_at: string }

type TaskView = Task & { status: string; sandboxes: SandboxView[] }

type Terminal = { id: string; command: string[]; running: boolean }

type Participant = {
  id: string
  name: string
  task: string
  human: boolean
  joined_at: string
  left_at?: string
  // Presence, computed when listed (presenceOf); not stored.
  online?: boolean
  last_seen_at?: string | null
}

type Message = {
  id: number
  channel: string
  from: string
  participant_id: string
  task: string
  reply_to?: number
  kind: 'message' | 'question'
  body: string
  created_at: string
}

type Waiter = { channel: string; after: number; wake: () => void }

// What GET /api/secrets reports, sorted like the server. CUSTOM has no
// known hosts, so a spec must bind it as CUSTOM@host.
const SECRETS = [
  { name: 'CLAUDE_CODE_OAUTH_TOKEN', hosts: ['api.anthropic.com'] },
  { name: 'CUSTOM', hosts: [] as string[] },
  { name: 'GITHUB_TOKEN', hosts: ['api.github.com', 'github.com'] },
]

const TASK_NAME_RE = /^[a-z][a-z0-9-]{0,62}$/
const PARTICIPANT_NAME_RE = /^[a-z][a-z0-9_-]{0,31}$/
const CHANNEL_RE = /^#[a-z0-9][a-z0-9_-]{0,63}$/
const BOOT_MS = 2000
const DEFAULT_WAIT_MS = 30_000
// GET /api/events sends a comment this often so proxies keep it open.
const HEARTBEAT_MS = 20_000

class HttpError extends Error {
  status: number
  body: unknown

  constructor(status: number, body: unknown) {
    super(JSON.stringify(body))
    this.status = status
    this.body = body
  }
}

const CODES: Record<number, string> = {
  400: 'EINVALID',
  404: 'ENOTFOUND',
  409: 'ECONFLICT',
}

// Answers 201 instead of 200, like allocating a sandbox.
class Created {
  body: unknown

  constructor(body: unknown) {
    this.body = body
  }
}

// Tasks, specs and secrets errors, shaped like the REST handlers.
function apiError(status: number, message: string) {
  const error = { code: CODES[status] ?? 'EINTERNAL', message, request_id: 'mock-' + hex(4) }
  return new HttpError(status, { error })
}

// Messaging errors, shaped like messaging/httpapi.
function msgError(status: number, message: string) {
  return new HttpError(status, { error: message })
}

function hex(bytes: number) {
  return randomBytes(bytes).toString('hex')
}

function iso(ms = Date.now()) {
  return new Date(ms).toISOString()
}

// Same rule as the server: "Review facebook ads" becomes review-facebook-ads.
export function slugify(title: string) {
  let slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  if (/^[0-9]/.test(slug)) {
    slug = 'task-' + slug
  }
  return slug.slice(0, 63).replace(/-+$/, '')
}

// Go duration strings as the wait endpoint takes them: 500ms, 25s, 1m.
function parseDuration(value: string) {
  const match = /^(\d+(?:\.\d+)?)(ms|s|m)$/.exec(value)
  if (!match) {
    throw msgError(400, 'timeout must be a duration such as 30s')
  }
  const unit = { ms: 1, s: 1000, m: 60_000 }[match[2] as 'ms' | 's' | 'm']
  return Number(match[1]) * unit
}

function parseInteger(value: string | null, name: string) {
  if (!value) {
    return 0
  }
  if (!/^-?\d+$/.test(value)) {
    throw msgError(400, `${name} must be an integer`)
  }
  return Number(value)
}

function createState() {
  const tasks = new Map<string, Task>()
  const sandboxes = new Map<string, Sandbox>()
  // When a starting sandbox finishes booting.
  const readyAt = new Map<string, number>()
  // Uncommitted changes and unpushed commits by sandbox name.
  const clones = new Map<string, { dirty: boolean; unpushed: number }>()
  // Terminal jobs by sandbox name.
  const terminals = new Map<string, Terminal[]>()
  const specs = new Map<string, string>()
  const participants = new Map<string, Participant>()
  const messages: Message[] = []
  let waiters: Waiter[] = []
  let nextMessageId = 1
  // GET /api/events listeners, each handed ready-to-write SSE frames.
  const subscribers = new Set<(frame: string) => void>()
  let nextEventId = 1

  // One server-sent event to every open /api/events stream; id is the
  // event number.
  function emit(type: string, data: unknown) {
    const frame = `id: ${nextEventId}\nevent: ${type}\ndata: ${JSON.stringify(data)}\n\n`
    nextEventId += 1
    for (const write of subscribers) {
      write(frame)
    }
  }

  function emitSandbox(sandbox: Sandbox) {
    emit('sandbox.status', { sandbox: sandboxView(sandbox) })
  }

  function addTask(title: string, name: string, fields: Partial<Task>, ageMs: number) {
    const at = iso(Date.now() - ageMs)
    const task: Task = {
      id: randomUUID(),
      title,
      name,
      brief: '',
      assignee: '',
      model: '',
      manager: '',
      owner: '',
      for: '',
      notify: true,
      done_at: null,
      accept: [],
      delivered_at: null,
      summary: '',
      report: null,
      accepted_by: '',
      created_at: at,
      updated_at: at,
      ...fields,
    }
    tasks.set(name, task)
    return task
  }

  // ADR 0009 item 3: the TASK.md an agent receives, byte for byte.
  function instructionsFor(task: Task, sandboxName: string, spec: TaskSpec) {
    const repo = spec.repo
      ? `The repository is cloned at /home/agent/${spec.repo.split('/').pop()}.`
      : 'There is no repository.'
    const agent = task.model ? `${task.assignee} (${task.model})` : task.assignee
    return (
      `# ${task.title}\n\n${task.brief}\n\n## How this task works\n\n` +
      `You are ${agent} working for the task \`${task.name}\` inside a sandbox (\`${sandboxName}\`).\n` +
      `Your manager is ${task.manager}; the owner is ${task.owner}. ${repo} Work counts only once it leaves this sandbox:\n\n` +
      `- Commit on a branch named \`agc/${task.name}\` and push it.\n` +
      `- Post what you changed and what you verified in \`#${task.name}\` with\n` +
      `  \`agc msg post '#${task.name}' <text>\`; your manager reads that channel and\n` +
      `  answers there. Ask your manager there too, then keep working on what\n` +
      `  does not depend on the answer.\n` +
      `- If something needs a decision only ${task.owner} can make, post it in\n` +
      `  \`#${task.owner}\` (their inbox) and say so in \`#${task.name}\`; do not wait idle.\n` +
      `- Do not modify files outside the repository, and never print secret values.\n`
    )
  }

  // The first sandbox of a task is named after it, later ones <task>-2, -3.
  function nextSandboxName(task: Task) {
    if (!sandboxes.has(task.name)) {
      return task.name
    }
    let n = 2
    while (sandboxes.has(`${task.name}-${n}`)) {
      n += 1
    }
    return `${task.name}-${n}`
  }

  // A booted, running sandbox; seeds adjust it.
  function addSandbox(task: Task, name: string, spec: TaskSpec, command: string, ageMs: number) {
    const at = iso(Date.now() - ageMs)
    const sandbox: Sandbox = {
      id: randomUUID(),
      task_id: task.id,
      name,
      spec,
      command,
      instructions: instructionsFor(task, name, spec),
      boot_error: null,
      booted_at: at,
      stopped_at: null,
      created_at: at,
      runtime: 'running',
    }
    sandboxes.set(name, sandbox)
    terminals.set(name, [])
    return sandbox
  }

  // Allocation: the row is written as starting and boots for two seconds;
  // then the command, if any, runs as the job "agent".
  function allocate(task: Task, spec: TaskSpec, command: string) {
    const sandbox = addSandbox(task, nextSandboxName(task), spec, command, 0)
    sandbox.booted_at = null
    readyAt.set(sandbox.name, Date.now() + BOOT_MS)
    emitSandbox(sandbox)
    // Booting is lazy (sandboxView); this reads it once the boot is over,
    // so streams see starting -> running without anyone polling.
    setTimeout(() => {
      if (sandboxes.get(sandbox.name) === sandbox) {
        emitSandbox(sandbox)
      }
    }, BOOT_MS + 20).unref()
    return sandbox
  }

  function boot(sandbox: Sandbox) {
    const ready = readyAt.get(sandbox.name)
    if (sandbox.booted_at !== null || sandbox.boot_error !== null || ready === undefined || Date.now() < ready) {
      return
    }
    readyAt.delete(sandbox.name)
    sandbox.booted_at = iso()
    sandbox.runtime = 'running'
    const command = sandbox.command.trim()
    if (command) {
      terminals.get(sandbox.name)?.push({ id: 'agent', command: command.split(/\s+/), running: true })
    }
  }

  function taskName(sandbox: Sandbox) {
    for (const task of tasks.values()) {
      if (task.id === sandbox.task_id) {
        return task.name
      }
    }
    return ''
  }

  // Status as the server computes it: starting until booted, failed with
  // the boot error, otherwise the runtime's word.
  function sandboxView(sandbox: Sandbox): SandboxView {
    boot(sandbox)
    const { boot_error, runtime, ...row } = sandbox
    const view: SandboxView = { ...row, task: taskName(sandbox), status: runtime, work: null }
    if (boot_error !== null) {
      view.status = 'failed'
      view.status_error = boot_error
    } else if (sandbox.booted_at === null) {
      view.status = 'starting'
    }
    return view
  }

  // The clone's work, as the server reads it: only when running with a repo.
  function withWork(view: SandboxView): SandboxView {
    if (view.status !== 'running' || !view.spec.repo) {
      return view
    }
    const clone = clones.get(view.name) ?? { dirty: false, unpushed: 0 }
    return { ...view, work: { ...clone, checked_at: iso() } }
  }

  function sandboxesOf(task: Task) {
    const list = [...sandboxes.values()].filter((b) => b.task_id === task.id).map(sandboxView)
    return list.sort((a, b) => b.created_at.localeCompare(a.created_at))
  }

  // done when done_at is set, otherwise review when delivered_at is set
  // (ADR 0010), otherwise running when a sandbox runs, otherwise open.
  function taskView(task: Task): TaskView {
    const boxes = sandboxesOf(task)
    let status = 'open'
    if (task.done_at) {
      status = 'done'
    } else if (task.delivered_at) {
      status = 'review'
    } else if (boxes.some((b) => b.status === 'running')) {
      status = 'running'
    }
    return { ...task, status, sandboxes: boxes }
  }

  // A task is found by UUID or by name, like TaskIdParam.
  function findTask(ref: string) {
    const byName = tasks.get(ref)
    if (byName) {
      return byName
    }
    for (const task of tasks.values()) {
      if (task.id === ref) {
        return task
      }
    }
    throw apiError(404, `task ${ref} not found`)
  }

  // A sandbox is found by UUID or by name too.
  function findSandbox(ref: string) {
    const byName = sandboxes.get(ref)
    if (byName) {
      return byName
    }
    for (const sandbox of sandboxes.values()) {
      if (sandbox.id === ref) {
        return sandbox
      }
    }
    throw apiError(404, `sandbox ${ref} not found`)
  }

  function deleteSandbox(sandbox: Sandbox) {
    emit('sandbox.status', { sandbox: { ...sandboxView(sandbox), status: 'deleted' } })
    sandboxes.delete(sandbox.name)
    terminals.delete(sandbox.name)
    readyAt.delete(sandbox.name)
  }

  function join(name: string, task: string, human: boolean) {
    const p: Participant = { id: 'p_' + hex(8), name, task, human, joined_at: iso() }
    participants.set(p.id, p)
    return p
  }

  function post(channel: string, p: Participant, body: string, replyTo?: number, kind: Message['kind'] = 'message') {
    const m: Message = {
      id: nextMessageId,
      channel,
      from: p.name,
      participant_id: p.id,
      task: p.task,
      kind,
      body,
      created_at: iso(),
    }
    if (replyTo !== undefined) {
      m.reply_to = replyTo
    }
    nextMessageId += 1
    messages.push(m)
    emit('message.posted', { message: m })
    const woken = waiters.filter((w) => w.channel === channel)
    waiters = waiters.filter((w) => w.channel !== channel)
    for (const w of woken) {
      w.wake()
    }
    return m
  }

  function after(channel: string, cursor: number, limit: number) {
    return messages.filter((m) => m.channel === channel && m.id > cursor).slice(0, limit)
  }

  function seed() {
    const base: TaskSpec = {
      image: 'agc-agent',
      cpus: 2,
      memory: 4096,
      repo: 'github.com/vanclief/agent-composer',
      ref: 'master',
      setup: 'go mod download\n',
      secrets: ['CLAUDE_CODE_OAUTH_TOKEN', 'GITHUB_TOKEN'],
      env: { GOFLAGS: '-mod=mod' },
      volumes: ['go-mod-cache:/home/agent/go/pkg/mod:rw'],
      channels: ['#releases', '#alerts:read'],
    }
    const review = addTask(
      'Review 42',
      'review-42',
      {
        brief:
          'Review PR 42 (attach rulesets to task specs).\n\n' +
          '- Run the tests and read the diff against the rules.\n' +
          '- Post findings in #review-42.\n\n' +
          'Done when every finding is posted and the PR is approved or has changes requested.',
        assignee: 'claude_code',
        model: 'claude-opus-5-5',
        manager: 'agent-composer-da',
        owner: 'franco',
        for: 'attach-rulesets-to-task-specs',
        accept: [
          '`go test ./...` passes',
          'every finding is posted in #review-42',
          'the PR is approved or has changes requested',
        ],
        delivered_at: iso(Date.now() - 5 * 60_000),
        summary: 'Reviewed PR 42: two findings, both fixed on agc/review-42.',
        report: {
          criteria: [
            { met: 'yes', note: 'All packages pass on agc/review-42.' },
            { met: 'partly', note: 'Two of three posted; the flaky one is only in the summary.' },
            { met: 'no', note: 'Not approved: needs a maintainer with write access.' },
          ],
          changes: { branch: 'agc/review-42', sha: '3f9c2ab71e04d5c8b6a1f0e2d4c7b9a8e5f1d203', files: 4 },
          checks: [
            { name: 'go test ./...', passed: true, note: '312 tests' },
            { name: 'go vet ./...', passed: false, note: 'workflow/loop.go: unreachable code' },
          ],
          left_out: [{ what: 'The flaky third failure', why: 'Did not reproduce in 20 runs.' }],
          needs: ['Approve PR 42 on GitHub', 'Decide whether the vet warning blocks the merge'],
        },
      },
      40 * 60_000,
    )
    // The sandbox running now, and an older one that was stopped.
    addSandbox(review, 'review-42', base, 'claude', 30 * 60_000)
    const older = addSandbox(review, 'review-42-2', base, 'claude', 38 * 60_000)
    older.runtime = 'stopped'
    older.stopped_at = iso(Date.now() - 32 * 60_000)
    terminals.set('review-42', [
      { id: 'agent', command: ['claude'], running: true },
      { id: 'job_' + hex(4), command: ['bash'], running: false },
    ])

    // A human's task: no sandbox.
    addTask(
      'Flaky CI',
      'flaky-ci',
      {
        brief: 'Find why CI fails one run in five and fix it.',
        accept: ['CI passes ten runs in a row'],
        assignee: 'franco',
        manager: 'franco',
        owner: 'franco',
      },
      3 * 3600_000,
    )

    const docs = addTask(
      'Docs pass',
      'docs-pass',
      {
        brief: 'Bring docs/ up to date with the REST API.',
        accept: ['every route in docs/openapi.yaml is described in docs/'],
        assignee: 'codex_cli',
        model: 'gpt-6.1-sol',
        manager: 'franco',
        owner: 'franco',
        notify: false,
        done_at: iso(Date.now() - 86_400_000),
        accepted_by: 'franco',
        summary: 'Brought docs/ up to date with the REST API.',
        delivered_at: iso(Date.now() - 90_000_000),
      },
      2 * 86_400_000,
    )
    const docsBox = addSandbox(docs, 'docs-pass', { ...base, cpus: 1, memory: 2048, ref: 'docs' }, '', 2 * 86_400_000)
    docsBox.runtime = 'stopped'
    docsBox.stopped_at = iso(Date.now() - 86_400_000)

    const fail = addTask(
      'Boot fail',
      'boot-fail',
      {
        brief: 'Review the private repo.',
        assignee: 'claude_code',
        model: 'claude-opus-5-5',
        manager: 'franco',
        owner: 'franco',
      },
      20 * 60_000,
    )
    const failBox = addSandbox(
      fail,
      'boot-fail',
      { ...base, repo: 'github.com/vanclief/no-such-repo' },
      'claude',
      20 * 60_000,
    )
    failBox.booted_at = null
    failBox.boot_error = 'clone failed: repository not found'

    specs.set(
      'backend-review',
      'image: agc-agent\ncpus: 2\nmemory: 4096\nrepo: github.com/vanclief/agent-composer\nref: master\n',
    )

    const reviewer = join('reviewer', 'review-42', false)
    const tester = join('tester', 'review-42', false)
    const franco = join('franco', '', true)
    // ADR 0009 item 4: agc opens the channel addressing the manager and the
    // owner, then posts the instructions of sandbox review-42 verbatim, 40
    // minutes ago.
    const agc = join('agc', '', false)
    const opened = iso(Date.now() - 40 * 60_000)
    post(
      '#review-42',
      agc,
      '@agent-composer-da (manager), @franco (owner): task `review-42` is open: Review 42.\n' +
        'Agent: claude_code (claude-opus-5-5) on sandbox `review-42`, starting.\n' +
        "The agent's instructions are the next message. Reply here to talk to it; " +
        'it escalates to #franco when only the owner can decide. Post /accept to accept the task, /reject <why> to send it back.',
    ).created_at = opened
    post('#review-42', agc, sandboxes.get('review-42')?.instructions ?? '').created_at = opened
    const first = post('#review-42', reviewer, 'Cloned the repo and ran the tests. 3 failures in workflow/.')
    post('#review-42', franco, 'Look at the loop node first.', first.id)
    post('#review-42', tester, 'Reproduced 2 of 3 locally. The third looks flaky.')
    post('#review-42', reviewer, 'Fixed the loop node. Pushing to review-42.')
    post('#releases', reviewer, 'v0.4.0 tagged.')
    // ADR 0009 item 5: franco's inbox. agc's two opening messages for
    // review-42 (the first mentions @franco), then the review-42 agent
    // escalating a decision only the owner can make.
    for (const opening of messages.filter((m) => m.channel === '#review-42' && m.from === 'agc')) {
      post('#franco', agc, opening.body).created_at = opened
    }
    post(
      '#franco',
      reviewer,
      '@franco decision needed on review-42: the third workflow/ failure is a flaky test, not the loop node. ' +
        'Quarantine it and merge the fix, or block the merge until the flake is fixed too?',
    )
    // Questions (kind question): an open one in #review-42 to @franco, an
    // open one in #flaky-ci, and one in #review-42 franco answered. The
    // seeded messages in #review-42 are spread over the last half hour.
    const ago = (m: Message, minutes: number) => {
      m.created_at = iso(Date.now() - minutes * 60_000)
      return m
    }
    const asked = ago(
      post(
        '#review-42',
        tester,
        'Is the third `workflow/` failure in scope for this review, or does it belong to #flaky-ci?',
        undefined,
        'question',
      ),
      9,
    )
    ago(post('#review-42', franco, 'Out of scope: #flaky-ci owns it.', asked.id), 8)
    ago(
      post(
        '#review-42',
        reviewer,
        'The loop node test now passes:\n\n```sh\ngo test ./workflow -run TestLoopNode -count=10\nok  \tgithub.com/vanclief/agent-composer/workflow\t1.84s\n```\n\n' +
          'Diff is on [agc/review-42](https://github.com/vanclief/agent-composer/tree/agc/review-42).',
      ),
      6,
    )
    ago(
      post(
        '#review-42',
        reviewer,
        '@franco should I squash the two fix commits into one before you merge, or keep them separate?',
        undefined,
        'question',
      ),
      4,
    )
    const fixer = join('fixer', 'flaky-ci', false)
    post(
      '#flaky-ci',
      fixer,
      'Which runner should I reproduce the flake on: the rig, or a GitHub Actions runner?',
      undefined,
      'question',
    )
    const reviewChannel = messages.filter((m) => m.channel === '#review-42' && m.from !== 'agc')
    for (const [i, minutes] of [30, 28, 25, 12].entries()) {
      ago(reviewChannel[i], minutes)
    }
    // Direct-message channels @<human>/<task> (ahead of the backend): an
    // unread agent message in @franco/yc-pitch, and @franco/review-42,
    // read: franco answered last, and a message of mine reads everything
    // before it (src/features/messages/unread.ts).
    const pitcher = join('pitcher', 'yc-pitch', false)
    post('@franco/review-42', reviewer, 'PR 42 is ready for your eyes when you have a minute.')
    post('@franco/review-42', franco, 'Thanks, looking after lunch.')
    post('@franco/yc-pitch', pitcher, 'Draft 3 of the YC pitch is up: should the demo lead or close?')
    // The seeded human has left, so the panel can join as @franco.
    franco.left_at = iso()
  }

  seed()

  // Stops a sandbox (also a starting one) as POST /sandboxes/:id/stop does.
  function stopSandbox(sandbox: Sandbox) {
    sandbox.runtime = 'stopped'
    sandbox.stopped_at = iso()
    readyAt.delete(sandbox.name)
    if (sandbox.booted_at === null && sandbox.boot_error === null) {
      // A starting sandbox stops before its boot finishes.
      sandbox.booted_at = sandbox.stopped_at
    }
    for (const job of terminals.get(sandbox.name) ?? []) {
      job.running = false
    }
    emitSandbox(sandbox)
  }

  // The reserved participant agc posts as.
  function agc() {
    for (const p of participants.values()) {
      if (p.name === 'agc' && !p.left_at) {
        return p
      }
    }
    return join('agc', '', false)
  }

  // The work in each clone: review-42 has uncommitted changes and 2
  // unpushed commits, every other clone is clean.
  clones.set('review-42', { dirty: true, unpushed: 2 })

  return {
    tasks,
    sandboxes,
    terminals,
    specs,
    participants,
    messages,
    taskView,
    sandboxView,
    sandboxesOf,
    stopSandbox,
    agc,
    withWork,
    clones,
    findTask,
    findSandbox,
    addTask,
    allocate,
    deleteSandbox,
    join,
    post,
    after,
    addWaiter(w: Waiter) {
      waiters.push(w)
    },
    removeWaiter(w: Waiter) {
      waiters = waiters.filter((x) => x !== w)
    },
    emit,
    emitSandbox,
    // Returns the unsubscribe.
    subscribe(write: (frame: string) => void) {
      subscribers.add(write)
      return () => {
        subscribers.delete(write)
      }
    },
    // The agent's closing words for task.done: the last message in
    // #<task> from someone neither human nor agc, else "".
    summary(task: Task) {
      for (let i = messages.length - 1; i >= 0; i -= 1) {
        const m = messages[i]
        if (m.channel !== '#' + task.name || m.from === 'agc') {
          continue
        }
        if (participants.get(m.participant_id)?.human === false) {
          return m.body
        }
      }
      return ''
    },
  }
}

type State = ReturnType<typeof createState>

function send(res: ServerResponse, status: number, body: unknown) {
  const data = JSON.stringify(body)
  res.writeHead(status, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(data) })
  res.end(data)
}

async function readJSON(req: IncomingMessage, fail: (status: number, message: string) => HttpError) {
  const chunks: Buffer[] = []
  for await (const chunk of req) {
    chunks.push(chunk as Buffer)
  }
  const text = Buffer.concat(chunks).toString('utf8')
  if (!text.trim()) {
    return {} as Record<string, unknown>
  }
  let body: unknown
  try {
    body = JSON.parse(text)
  } catch {
    throw fail(400, 'request body must be a JSON object')
  }
  if (typeof body !== 'object' || body === null || Array.isArray(body)) {
    throw fail(400, 'request body must be a JSON object')
  }
  return body as Record<string, unknown>
}

function str(value: unknown) {
  if (value === undefined || value === null) {
    return ''
  }
  return String(value)
}

async function tasksAPI(state: State, method: string, rest: string[], req: IncomingMessage) {
  if (rest.length === 0 && method === 'GET') {
    const list = [...state.tasks.values()].map(state.taskView)
    list.sort((a, b) => b.created_at.localeCompare(a.created_at))
    return { tasks: list }
  }
  if (rest.length === 0 && method === 'POST') {
    return createTask(state, await readJSON(req, apiError))
  }

  const task = state.findTask(rest[0])
  if (rest.length === 1 && method === 'GET') {
    return state.taskView(task)
  }
  if (rest.length === 1 && method === 'DELETE') {
    for (const sandbox of [...state.sandboxes.values()]) {
      if (sandbox.task_id === task.id) {
        state.deleteSandbox(sandbox)
      }
    }
    state.tasks.delete(task.name)
    return { deleted: true, name: task.name }
  }
  // ADR 0010 item 2: the agent delivers; the task goes to review.
  if (rest.length === 2 && method === 'POST' && rest[1] === 'deliver') {
    const body = await readJSON(req, apiError)
    const summary = str(body.summary).trim()
    if (!summary) {
      throw apiError(400, 'summary is required')
    }
    if (task.done_at) {
      throw apiError(409, `task ${task.name} is already done`)
    }
    task.delivered_at = iso()
    task.summary = summary
    task.report = body.report && typeof body.report === 'object' ? (body.report as Report) : null
    task.updated_at = task.delivered_at
    state.post('#' + task.name, state.agc(), `Delivered: ${summary}`)
    return state.taskView(task)
  }
  // ADR 0010 item 4: sets done_at and accepted_by, posts in #<task>, then
  // stops every running or starting sandbox of the task.
  if (rest.length === 2 && method === 'POST' && rest[1] === 'accept') {
    const body = await readJSON(req, apiError)
    if (task.done_at) {
      throw apiError(409, `task ${task.name} is already done`)
    }
    const by = str(body.by).trim() || task.owner
    task.done_at = iso()
    task.accepted_by = by
    task.updated_at = task.done_at
    state.post('#' + task.name, state.agc(), `Accepted by ${by}.`)
    for (const view of state.sandboxesOf(task)) {
      if (view.status === 'running' || view.status === 'starting') {
        state.stopSandbox(state.findSandbox(view.name))
      }
    }
    const view = state.taskView(task)
    state.emit('task.done', { task: view, summary: task.summary || state.summary(task) })
    return view
  }
  // ADR 0010 item 5: clears delivered_at and sends the task back with why.
  if (rest.length === 2 && method === 'POST' && rest[1] === 'reject') {
    const body = await readJSON(req, apiError)
    const by = str(body.by).trim() || task.owner
    const why = str(body.why).trim()
    if (!why) {
      throw apiError(400, 'why is required')
    }
    if (task.done_at) {
      throw apiError(409, `task ${task.name} is already done`)
    }
    task.delivered_at = null
    task.updated_at = iso()
    state.post('#' + task.name, state.agc(), `Rejected by ${by}: ${why}`)
    return state.taskView(task)
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'sandboxes') {
    const request = sandboxRequest(await readJSON(req, apiError))
    const sandbox = state.allocate(task, request.spec, request.command)
    return new Created(state.sandboxView(sandbox))
  }
  throw apiError(404, 'no such endpoint')
}

// The environment block inline plus command, validated like the server.
function sandboxRequest(body: Record<string, unknown>) {
  const { command, ...spec } = body
  const known = new Set(SECRETS.map((s) => s.name))
  for (const entry of (spec.secrets as unknown[] | undefined) ?? []) {
    const secret = String(entry).split('@')[0]
    if (!known.has(secret)) {
      throw apiError(
        400,
        `secret ${secret} is not defined; add it to ~/.agent_composer/config/secrets.env and try again`,
      )
    }
  }
  for (const channel of (spec.channels as unknown[] | undefined) ?? []) {
    if (!CHANNEL_RE.test(String(channel).replace(/:read$/, ''))) {
      throw apiError(400, `channel "${channel}" is invalid: start with # and use lowercase letters, digits, _ or -`)
    }
  }
  return { spec: spec as TaskSpec, command: str(command).trim() }
}

// ADR 0009 item 1: the server fills in the configured defaults (harness
// claude_code, model claude-opus-5-5, manager franco) when a field is empty.
const HARNESSES = ['claude_code', 'codex_cli', 'pi']
function taskFields(body: Record<string, unknown>) {
  const assignee = str(body.assignee).trim() || 'claude_code'
  const isHarness = HARNESSES.includes(assignee)
  let model = str(body.model).trim()
  if (model === '' && isHarness) {
    model = 'claude-opus-5-5'
  }
  return {
    brief: str(body.brief),
    assignee,
    model,
    manager: str(body.manager).trim() || 'franco',
    owner: str(body.owner).trim() || 'franco',
    for: str(body.for),
    // Default true: only an explicit false turns notifications off.
    notify: body.notify !== false,
    accept: Array.isArray(body.accept) ? body.accept.map(str).filter((c) => c.trim() !== '') : [],
  }
}

// Writes the row and returns at once; a sandbox in the request is
// allocated after the insert and comes back as starting.
function createTask(state: State, body: Record<string, unknown>) {
  const title = str(body.title).trim()
  const name = str(body.name) || slugify(title)
  if (!name) {
    throw apiError(400, `title "${title}" needs at least one letter or digit`)
  }
  if (!TASK_NAME_RE.test(name)) {
    throw apiError(400, `name "${name}" is invalid: use 1-63 lowercase letters, digits or -, starting with a letter`)
  }
  if (state.tasks.has(name)) {
    throw apiError(409, `task ${name} already exists`)
  }
  const sandbox = body.sandbox
  if (sandbox !== undefined && sandbox !== null && (typeof sandbox !== 'object' || Array.isArray(sandbox))) {
    throw apiError(400, 'sandbox must be an object')
  }
  const request = sandbox ? sandboxRequest(sandbox as Record<string, unknown>) : undefined
  const task = state.addTask(title || name, name, taskFields(body), 0)
  if (request) {
    state.allocate(task, request.spec, request.command)
  }
  const view = state.taskView(task)
  state.emit('task.created', { task: view })
  return view
}

async function sandboxesAPI(state: State, method: string, rest: string[], req: IncomingMessage, work = false) {
  if (rest.length === 0 && method === 'GET') {
    const list = [...state.sandboxes.values()].map(state.sandboxView).map((v) => (work ? state.withWork(v) : v))
    list.sort((a, b) => b.created_at.localeCompare(a.created_at))
    return { sandboxes: list }
  }

  const sandbox = state.findSandbox(rest[0])
  const view = state.sandboxView(sandbox)
  if (rest.length === 1 && method === 'GET') {
    return state.withWork(view)
  }
  if (rest.length === 1 && method === 'DELETE') {
    state.deleteSandbox(sandbox)
    return { deleted: true, name: sandbox.name }
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'stop') {
    if (view.status !== 'running') {
      throw apiError(409, `sandbox ${sandbox.name} is ${view.status}, not running`)
    }
    state.stopSandbox(sandbox)
    return state.sandboxView(sandbox)
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'start') {
    if (view.status !== 'stopped' && view.status !== 'unknown') {
      throw apiError(409, `sandbox ${sandbox.name} is ${view.status}, not stopped`)
    }
    sandbox.runtime = 'running'
    sandbox.stopped_at = null
    state.emitSandbox(sandbox)
    return state.sandboxView(sandbox)
  }
  if (rest.length === 2 && rest[1] === 'terminals') {
    if (view.status !== 'running') {
      throw apiError(409, `sandbox ${sandbox.name} is ${view.status}; start it to use terminals`)
    }
    const jobs = state.terminals.get(sandbox.name) ?? []
    if (method === 'GET') {
      return { terminals: jobs }
    }
    if (method === 'POST') {
      const body = await readJSON(req, apiError)
      const command = str(body.command).trim()
      if (!command) {
        throw apiError(400, 'command is required')
      }
      const job: Terminal = { id: 'job_' + hex(4), command: command.split(/\s+/), running: true }
      jobs.push(job)
      state.terminals.set(sandbox.name, jobs)
      return { id: job.id }
    }
  }
  throw apiError(404, 'no such endpoint')
}

async function specsAPI(state: State, method: string, rest: string[], req: IncomingMessage) {
  if (rest.length === 0 && method === 'GET') {
    const names = [...state.specs.keys()].sort()
    return { specs: names.map((name) => ({ name, yaml: state.specs.get(name) ?? '' })) }
  }
  if (rest.length !== 1) {
    throw apiError(404, 'no such endpoint')
  }
  const name = slugify(rest[0])
  if (method === 'PUT') {
    const body = await readJSON(req, apiError)
    const yaml = str(body.yaml)
    // Stand-in for the server's task.yaml validation.
    if (!/^image:\s*\S/m.test(yaml)) {
      throw apiError(400, 'spec is invalid: image is required')
    }
    if (!name) {
      throw apiError(400, `spec name "${rest[0]}" needs at least one letter or digit`)
    }
    state.specs.set(name, yaml)
    return { name, yaml }
  }
  const yaml = state.specs.get(name)
  if (yaml === undefined) {
    throw apiError(404, `spec ${rest[0]} not found`)
  }
  if (method === 'GET') {
    return { name, yaml }
  }
  if (method === 'DELETE') {
    state.specs.delete(name)
    return { name, yaml }
  }
  throw apiError(404, 'no such endpoint')
}

function checkChannel(channel: string) {
  if (!CHANNEL_RE.test(channel) && !DM_RE.test(channel)) {
    throw msgError(400, `channel "${channel}" is invalid: use # followed by 1-64 lowercase letters, digits, _ or -`)
  }
}

// Presence as the backend will send it: agc is always online; an agent is
// online while its task has a running sandbox; a human while someone holds
// the name, and franco always (seeded as online). last_seen_at is the
// participant's last message, else when it left, else when it joined.
function presenceOf(state: State, p: Participant) {
  let online: boolean
  if (p.name === 'agc') {
    online = true
  } else if (p.human) {
    online = p.name === 'franco' || [...state.participants.values()].some((q) => q.name === p.name && !q.left_at)
  } else {
    // A DM's agent may belong to a task the mock does not have (yc-pitch).
    const task = state.tasks.get(p.task) ?? [...state.tasks.values()].find((t) => t.id === p.task)
    online = task !== undefined && state.sandboxesOf(task).some((b) => b.status === 'running')
  }
  const last = state.messages.filter((m) => m.participant_id === p.id).at(-1)
  const last_seen_at: string | null = last?.created_at ?? p.left_at ?? p.joined_at ?? null
  return { online, last_seen_at }
}

async function msgAPI(state: State, method: string, verb: string, url: URL, req: IncomingMessage) {
  if (method === 'GET' && verb === 'channels') {
    const names = new Set<string>()
    for (const name of state.tasks.keys()) {
      names.add('#' + name)
    }
    for (const m of state.messages) {
      names.add(m.channel)
    }
    const channels = [...names].sort().map((channel) => {
      const list = state.messages.filter((m) => m.channel === channel)
      return { channel, read_only: false, last_id: list.at(-1)?.id ?? 0, count: list.length }
    })
    return { channels }
  }

  if (method === 'GET' && (verb === 'read' || verb === 'wait')) {
    const channel = url.searchParams.get('channel') ?? ''
    checkChannel(channel)
    const cursor = parseInteger(url.searchParams.get('after'), 'after')
    if (verb === 'read') {
      let limit = parseInteger(url.searchParams.get('limit'), 'limit')
      if (limit <= 0) {
        limit = 50
      }
      const found = state.after(channel, cursor, Math.min(limit, 500))
      return { messages: found, cursor: found.at(-1)?.id ?? cursor }
    }
    const timeoutParam = url.searchParams.get('timeout')
    const timeout = timeoutParam ? parseDuration(timeoutParam) : DEFAULT_WAIT_MS
    let found = state.after(channel, cursor, 500)
    if (found.length === 0) {
      await new Promise<void>((resolve) => {
        const waiter: Waiter = { channel, after: cursor, wake: () => done() }
        const timer = setTimeout(() => done(), timeout)
        const done = () => {
          clearTimeout(timer)
          state.removeWaiter(waiter)
          req.off('close', done)
          resolve()
        }
        state.addWaiter(waiter)
        req.on('close', done)
      })
      found = state.after(channel, cursor, 500)
    }
    return { messages: found, cursor: found.at(-1)?.id ?? cursor }
  }

  // Everyone who has posted in the channel: active first, then departed,
  // each by name, like Service.Participants.
  if (method === 'GET' && verb === 'participants') {
    if (url.searchParams.get('all') === '1') {
      return { participants: allParticipants(state) }
    }
    const channel = url.searchParams.get('channel') ?? ''
    checkChannel(channel)
    const ids = new Set(state.messages.filter((m) => m.channel === channel).map((m) => m.participant_id))
    const participants = [...ids]
      .map((id) => state.participants.get(id))
      .filter((p): p is Participant => p !== undefined)
      .sort((a, b) => Number(Boolean(a.left_at)) - Number(Boolean(b.left_at)) || a.name.localeCompare(b.name))
      .map((p) => ({ ...p, ...presenceOf(state, p) }))
    return { participants }
  }

  // Questions with who answered first: the first reply_to from someone
  // other than the asker. Mock only: for= is accepted and ignored, so every
  // question counts as the signed-in person's.
  if (method === 'GET' && verb === 'questions') {
    const channel = url.searchParams.get('channel') ?? ''
    if (channel) {
      checkChannel(channel)
    }
    const open = url.searchParams.get('open') === '1'
    const questions = state.messages
      .filter((m) => m.kind === 'question' && (!channel || m.channel === channel))
      .map((q) => {
        const answer = state.messages.find((m) => m.reply_to === q.id && m.participant_id !== q.participant_id)
        return { ...q, answered_by: answer?.from ?? '', answered_at: answer?.created_at ?? null }
      })
      .filter((q) => !open || !q.answered_by)
    return { questions }
  }

  if (method !== 'POST') {
    throw msgError(404, 'no such endpoint')
  }
  const body = await readJSON(req, msgError)

  if (verb === 'join') {
    const name = str(body.name)
    if (!PARTICIPANT_NAME_RE.test(name)) {
      throw msgError(
        400,
        `name "${name}" is invalid: use 1-32 lowercase letters, digits, _ or -, starting with a letter`,
      )
    }
    for (const p of state.participants.values()) {
      if (p.name === name && !p.left_at) {
        throw msgError(409, `name @${name} is already in use`)
      }
    }
    // Mock only: joining with a task joins as that task's agent (not
    // human), as an agent in its sandbox would, so task.done has a summary.
    const task = str(body.task)
    return state.join(name, task, task === '')
  }

  if (verb === 'leave') {
    const id = str(body.participant_id)
    const p = state.participants.get(id)
    if (!p) {
      throw msgError(404, `participant ${id} not found`)
    }
    if (!p.left_at) {
      p.left_at = iso()
    }
    return { participant_id: id, left: true }
  }

  if (verb === 'post') {
    const channel = str(body.channel)
    checkChannel(channel)
    const id = str(body.participant_id)
    const p = state.participants.get(id)
    if (!p) {
      throw msgError(404, `participant ${id} not found`)
    }
    if (p.left_at) {
      throw msgError(400, `participant @${p.name} has left, join again`)
    }
    const text = str(body.body)
    if (!text.trim()) {
      throw msgError(400, 'body is required')
    }
    let replyTo: number | undefined
    if (body.reply_to !== undefined && body.reply_to !== null) {
      replyTo = Number(body.reply_to)
      const target = state.messages.find((m) => m.id === replyTo && m.channel === channel)
      if (!target) {
        throw msgError(400, `reply_to ${replyTo} is not a message in ${channel}`)
      }
    }
    const kind = body.kind === undefined || body.kind === null ? 'message' : str(body.kind)
    if (kind !== 'message' && kind !== 'question') {
      throw msgError(400, `kind "${kind}" is invalid: use message or question`)
    }
    return state.post(channel, p, text, replyTo, kind)
  }

  throw msgError(404, 'no such endpoint')
}

async function handle(state: State, req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? '/', 'http://mock')
  const method = req.method ?? 'GET'
  const parts = url.pathname.split('/').filter(Boolean).map(decodeURIComponent)

  if (parts[0] !== 'api') {
    send(res, 404, { message: 'Not Found' })
    return
  }
  if (parts[1] === 'events' && parts.length === 2 && method === 'GET') {
    streamEvents(state, req, res)
    return
  }

  try {
    let body: unknown
    if (parts[1] === 'tasks') {
      body = await tasksAPI(state, method, parts.slice(2), req)
    } else if (parts[1] === 'sandboxes') {
      const work = url.searchParams.get('work')
      body = await sandboxesAPI(state, method, parts.slice(2), req, work === '1' || work === 'true')
    } else if (parts[1] === 'specs') {
      body = await specsAPI(state, method, parts.slice(2), req)
    } else if (parts[1] === 'secrets' && parts.length === 2 && method === 'GET') {
      body = { secrets: SECRETS }
    } else if (parts[1] === 'msg' && parts.length === 3) {
      body = await msgAPI(state, method, parts[2], url, req)
    } else if (parts[1] === 'knowledge' && method === 'GET') {
      body = knowledgeAPI(parts.slice(2), url)
    } else if (parts[1] === 'agents') {
      body = await agentsAPI(state, method, parts.slice(2), req)
    } else if (parts[1] === 'defaults' && parts.length === 2 && method === 'GET') {
      body = DEFAULTS
    } else {
      throw apiError(404, 'no such endpoint')
    }
    if (res.destroyed) {
      return
    }
    if (body instanceof Created) {
      send(res, 201, body.body)
      return
    }
    send(res, 200, body)
  } catch (err) {
    if (err instanceof HttpError) {
      send(res, err.status, err.body)
      return
    }
    console.error(err)
    send(res, 500, { error: { code: 'EINTERNAL', message: String(err), request_id: 'mock-' + hex(4) } })
  }
}

// GET /api/events: server-sent events (task.created, task.done,
// sandbox.status, message.posted), each with an id, and a comment every
// 20s as a heartbeat. Missed events are not replayed on reconnect.
function streamEvents(state: State, req: IncomingMessage, res: ServerResponse) {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive',
  })
  res.write(': connected\n\n')
  const heartbeat = setInterval(() => res.write(': heartbeat\n\n'), HEARTBEAT_MS)
  const unsubscribe = state.subscribe((frame) => res.write(frame))
  req.on('close', () => {
    clearInterval(heartbeat)
    unsubscribe()
  })
}

// /api/sandboxes/:id/terminals/:job/ws. Binary frames are terminal bytes both
// ways; a text frame {"type":"resize","rows":N,"cols":N} resizes. The mock
// echoes input and acknowledges resizes. A finished job replays a line and
// closes with "job completed", like the real socket.
function attachTerminal(ws: WebSocket, sandbox: Sandbox, job: Terminal) {
  const write = (text: string) => ws.send(Buffer.from(text), { binary: true })

  write(`\x1b[2mmock terminal · ${sandbox.name} · ${job.command.join(' ')}\x1b[0m\r\n`)
  if (!job.running) {
    write('[process exited]\r\n')
    ws.close(1000, 'job completed')
    return
  }
  write('$ ')

  ws.on('message', (data, isBinary) => {
    const text = Buffer.isBuffer(data) ? data.toString('utf8') : String(data)
    if (isBinary) {
      write(text.replace(/\r/g, '\r\n$ '))
      return
    }
    let frame: { type?: string; rows?: number; cols?: number }
    try {
      frame = JSON.parse(text)
    } catch {
      return
    }
    if (frame.type === 'resize') {
      write(`\r\n\x1b[2m[resized to ${frame.rows}x${frame.cols}]\x1b[0m\r\n$ `)
    }
  })
}

export type MockServer = { url: string; port: number; close: () => Promise<void> }

// startMock serves a fresh, seeded mock. Port 0 picks a free port.
export async function startMock(port = 8790, host = '127.0.0.1'): Promise<MockServer> {
  const state = createState()
  const server = createServer((req, res) => {
    void handle(state, req, res)
  })
  const wss = new WebSocketServer({ noServer: true })

  server.on('upgrade', (req, socket, head) => {
    const url = new URL(req.url ?? '/', 'http://mock')
    const match = /^\/api\/sandboxes\/([^/]+)\/terminals\/([^/]+)\/ws$/.exec(url.pathname)
    const refuse = (status: number, message: string) => {
      const body = JSON.stringify({ error: { code: CODES[status], message, request_id: 'mock-' + hex(4) } })
      socket.end(
        `HTTP/1.1 ${status} ${status === 404 ? 'Not Found' : 'Conflict'}\r\n` +
          `Content-Type: application/json\r\nContent-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`,
      )
    }
    if (!match) {
      refuse(404, 'no such endpoint')
      return
    }
    let sandbox: Sandbox
    try {
      sandbox = state.findSandbox(decodeURIComponent(match[1]))
    } catch (err) {
      const message =
        err instanceof HttpError ? (err.body as { error: { message: string } }).error.message : String(err)
      refuse(404, message)
      return
    }
    const status = state.sandboxView(sandbox).status
    if (status !== 'running') {
      refuse(409, `sandbox ${sandbox.name} is ${status}; start it to use terminals`)
      return
    }
    const jobID = decodeURIComponent(match[2])
    const job = (state.terminals.get(sandbox.name) ?? []).find((j) => j.id === jobID)
    if (!job) {
      refuse(404, `terminal ${jobID} not found`)
      return
    }
    wss.handleUpgrade(req, socket, head, (ws) => attachTerminal(ws, sandbox, job))
  })

  await new Promise<void>((resolve) => server.listen(port, host, resolve))
  const actual = (server.address() as AddressInfo).port

  return {
    url: `http://${host}:${actual}`,
    port: actual,
    close: () =>
      new Promise<void>((resolve) => {
        for (const client of wss.clients) {
          client.terminate()
        }
        server.closeAllConnections()
        server.close(() => resolve())
      }),
  }
}

function flag(name: string, fallback: string) {
  const i = process.argv.indexOf(name)
  if (i >= 0 && process.argv[i + 1]) {
    return process.argv[i + 1]
  }
  return fallback
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  const mock = await startMock(Number(flag('--port', process.env.PORT ?? '8790')), flag('--host', '127.0.0.1'))
  console.log(`agc mock API on ${mock.url}/api`)
}

// ---------- knowledge (ADR 0006) ----------
//
// GET /api/knowledge[?without=a,b], /api/knowledge/why/:id,
// /api/knowledge/doubt/:id and /api/knowledge/conflicts. A subset of the
// repo's evidence/, beliefs/, decisions/, rules/ and actions/ files, with
// real ids and text, plus knowledge/testdata/contradictions (two contested
// pairs) and the belief `either` from knowledge/testdata/alternatives (two
// independent reasons). because lists keep only seeded ids, and the
// microsandbox spike evidence is retracted here so one belief is in doubt.
// Labels, opinions, doubted and contested_by are computed like
// knowledge/graph.go, per set of reasons.
// Only function declarations below: they are hoisted, so a request that
// arrives while this module finishes loading still finds them.

type KRetraction = { who: string; when: string; why: string }
type KEvidence = {
  id: string
  text: string
  who: string
  when: string
  source: string
  retracted: KRetraction | null
  translated_by?: string
  contradicts?: string[]
  file: string
}
// One other set of reasons under or:.
type KOr = { because?: string[]; evidence?: string[] }
type KBelief = {
  id: string
  text: string
  because: string[]
  evidence: string[]
  or?: KOr[]
  contradicts?: string[]
  file: string
  // Who holds it (empty: the repo owner) and when; experience makes a
  // leaf a judgment.
  by?: string
  when?: string
  experience?: { of: string; in: string }
}
type KDecision = {
  id: string
  text: string
  because: string[]
  or?: KOr[]
  adr: string
  decided: { who: string; when: string }
  file: string
}
type KRule = { subject: string; id: string; text: string; check: string; because: string[]; or?: KOr[]; file: string }
type KAction = {
  id: string
  text: string
  because: string[]
  or?: KOr[]
  opened: { who: string; when: string }
  done?: { who: string; when: string }
  file: string
}

function knowledgeSeed() {
  const ev = 'evidence/repo.yaml'
  const evidence: KEvidence[] = [
    {
      id: 'gofmt-is-the-only-go-formatter',
      text: 'gofmt ships with the Go toolchain and the ecosystem has no competing formatter.',
      who: 'franco',
      when: '2026-11-01',
      source: 'https://go.dev/blog/gofmt',
      retracted: null,
      file: ev,
    },
    {
      id: 'go-spec-has-a-small-syntax',
      text: 'The Go spec defines a small syntax with few ways to write the same thing.',
      who: 'franco',
      when: '2026-11-01',
      source: 'https://go.dev/ref/spec',
      retracted: null,
      file: ev,
    },
    {
      id: 'agc-stores-everything-in-local-sqlite',
      text: 'agc stores everything in a local SQLite database by default, with no external services.',
      who: 'franco',
      when: '2026-11-01',
      source: 'README.md',
      retracted: null,
      file: ev,
    },
    {
      id: 'rig-ports-are-open-on-the-home-lan',
      text: 'The rig sits on the home LAN with no firewall in front of its ports.',
      who: 'franco',
      when: '2026-11-01',
      source: '',
      retracted: null,
      file: ev,
    },
    {
      id: 'host-harnesses-share-the-host',
      text: "Harnesses run as host subprocesses share the host's network, credentials, installed tools and process table.",
      who: 'franco',
      when: '2026-11-01',
      source: 'docs/decisions/0003-tasks-run-in-microsandbox-vms.md',
      retracted: null,
      file: ev,
    },
    {
      id: 'microsandbox-spike-on-the-rig',
      text: 'On the rig, a microsandbox 0.7.13 VM with Claude Code and Codex booted in under two seconds and kept a detached terminal across disconnects; placeholder secrets worked for Claude Code, the GitHub API and git over HTTPS, and a placeholder sent to a non-allowed host was refused.',
      who: 'franco',
      when: '2026-11-01',
      source: 'docs/decisions/0003-tasks-run-in-microsandbox-vms.md',
      retracted: {
        who: 'franco',
        when: '2026-11-02',
        why: 'The rig moved to microsandbox 0.8 and the spike was not re-run.',
      },
      file: ev,
    },
  ]
  const bf = 'beliefs/repo.yaml'
  const beliefs: KBelief[] = [
    {
      id: 'go-has-one-formatting',
      text: 'Go has one canonical format, enforced by gofmt.',
      because: [],
      evidence: ['gofmt-is-the-only-go-formatter'],
      file: bf,
    },
    {
      id: 'go-syntax-is-small',
      text: "Go's syntax is small, so there are few ways to write the same thing.",
      because: [],
      evidence: ['go-spec-has-a-small-syntax'],
      file: bf,
    },
    {
      id: 'agents-write-go-well',
      text: 'Go is better than Rust in general for writing code with agents.',
      because: ['go-has-one-formatting', 'go-syntax-is-small'],
      evidence: [],
      file: bf,
    },
    {
      id: 'strict-style-makes-agent-output-reviewable',
      text: 'A strict, uniform Go style makes code written by agents quick to review.',
      because: ['agents-write-go-well'],
      evidence: [],
      file: bf,
    },
    {
      id: 'rig-is-on-home-lan-without-firewall',
      text: 'The rig sits on the home LAN with no firewall in front of its ports.',
      because: [],
      evidence: ['rig-ports-are-open-on-the-home-lan'],
      file: bf,
    },
    {
      id: 'workflows-will-take-large-media',
      text: 'Workflows will take large media such as raw video and photo sets as input.',
      because: [],
      evidence: [],
      file: bf,
    },
    {
      id: 'host-subprocesses-share-the-host',
      text: "Harnesses run as host subprocesses share the host's network, credentials, tools and process table.",
      because: [],
      evidence: ['host-harnesses-share-the-host'],
      file: bf,
    },
    {
      id: 'microsandbox-spike-passed',
      text: 'On the rig, a microsandbox VM with Claude Code and Codex boots in under two seconds and keeps a detached terminal across disconnects.',
      because: [],
      evidence: ['microsandbox-spike-on-the-rig'],
      file: bf,
    },
  ]
  const df = 'decisions/repo.yaml'
  const decisions: KDecision[] = [
    {
      id: 'binary-artifacts-stored-by-file-class',
      text: 'Small stable binaries are committed, generated ones are gitignored and regenerated, and large source binaries stay out of git behind a text manifest.',
      because: ['workflows-will-take-large-media'],
      adr: 'docs/decisions/0002-binary-artifact-storage.md',
      decided: { who: 'franco', when: '2026-07-25' },
      file: df,
    },
    {
      id: 'tasks-run-in-microsandbox-vms',
      text: 'A Task is one microsandbox VM, and agc reads its state live from the runtime.',
      because: ['host-subprocesses-share-the-host', 'microsandbox-spike-passed'],
      adr: 'docs/decisions/0003-tasks-run-in-microsandbox-vms.md',
      decided: { who: 'franco', when: '2026-11-01' },
      file: df,
    },
    {
      id: 'messaging-over-per-task-vsock',
      text: "agc serves one message space over a per-Task vsock socket and assigns every sender's identity itself.",
      because: ['tasks-run-in-microsandbox-vms'],
      adr: 'docs/decisions/0005-messaging-transport-and-identity.md',
      decided: { who: 'franco', when: '2026-11-01' },
      file: df,
    },
    {
      id: 'secrets-are-placeholders-bound-to-hosts',
      text: 'Task specs name secrets without values, and each reaches the Task as a placeholder swapped in only for its allowed hosts.',
      because: ['tasks-run-in-microsandbox-vms'],
      adr: 'docs/decisions/0004-secrets-are-placeholders-bound-to-hosts.md',
      decided: { who: 'franco', when: '2026-11-01' },
      file: df,
    },
    {
      id: 'rules-beliefs-and-decisions-as-yaml',
      text: 'Rules, beliefs and decisions are YAML files in the repo that chain by because, and doubt only lists what to recheck.',
      because: [],
      adr: 'docs/decisions/0006-rules-and-beliefs.md',
      decided: { who: 'franco', when: '2026-11-01' },
      file: df,
    },
  ]
  // knowledge/testdata/contradictions and alternatives.
  const cev = 'evidence/contradictions.yaml'
  const aev = 'evidence/alternatives.yaml'
  evidence.push(
    {
      id: 'rig-has-kvm',
      text: 'The rig has KVM.',
      who: 'franco',
      when: '2026-11-01',
      source: '',
      retracted: null,
      file: cev,
    },
    {
      id: 'rig-lacks-kvm',
      text: 'The rig has no KVM.',
      who: 'ana',
      when: '2026-11-02',
      source: 'lsmod on the rig',
      translated_by: 'codex-cli',
      retracted: null,
      contradicts: ['rig-has-kvm'],
      file: cev,
    },
    { id: 'seen-b', text: 'B was seen.', who: 'franco', when: '2026-11-01', source: '', retracted: null, file: cev },
    { id: 'seen-x', text: 'X was seen.', who: 'franco', when: '2026-11-01', source: '', retracted: null, file: aev },
    { id: 'seen-y', text: 'Y was seen.', who: 'ana', when: '2026-11-02', source: '', retracted: null, file: aev },
  )
  const cbf = 'beliefs/contradictions.yaml'
  const abf = 'beliefs/alternatives.yaml'
  beliefs.push(
    { id: 'claim-a', text: 'The rig can run sandboxes.', because: [], evidence: ['rig-has-kvm'], file: cbf },
    { id: 'claim-b', text: 'B is true.', because: [], evidence: ['seen-b'], contradicts: ['claim-c'], file: cbf },
    { id: 'claim-c', text: 'C is true.', because: ['hunch-c'], evidence: [], file: cbf },
    { id: 'hunch-c', text: 'C seems likely.', because: [], evidence: [], file: cbf },
    { id: 'by-x', text: 'X holds.', because: [], evidence: ['seen-x'], file: abf },
    { id: 'either', text: 'W holds.', because: ['by-x'], evidence: [], or: [{ evidence: ['seen-y'] }], file: abf },
  )
  // A judgment (expert testimony) and a verdict recorded by agc rules judge.
  beliefs.push(
    {
      id: 'icon-and-shape-ui-reduces-operator-mistakes',
      text: 'An interface that speaks in icons and shapes reduces operator mistakes.',
      because: [],
      evidence: [],
      experience: { of: 'franco', in: 'years of using strategy-game and operator UIs' },
      file: bf,
    },
    {
      id: 'diff-has-no-explicit-semicolons',
      text: 'The diff writes no explicit semicolons in Go outside a 3-clause for header.',
      because: [],
      evidence: [],
      by: 'codex_cli',
      when: '2026-11-01',
      file: 'beliefs/judgments.yaml',
    },
  )
  const af = 'actions/repo.yaml'
  const actions: KAction[] = [
    {
      id: 'attach-rulesets-to-task-specs',
      text: "Let a task spec name rulesets, beliefs and decisions so agents in its sandbox receive them, for example in the sandbox's AGENTS.md.",
      because: ['rules-beliefs-and-decisions-as-yaml', 'tasks-run-in-microsandbox-vms'],
      opened: { who: 'franco', when: '2026-11-01' },
      file: af,
    },
    {
      id: 'release-builds-with-cgo',
      text: 'Build the release binaries with cgo so Tasks can run sandboxes from a released agc.',
      because: ['tasks-run-in-microsandbox-vms'],
      opened: { who: 'franco', when: '2026-11-01' },
      file: af,
    },
    {
      id: 'workflow-nodes-name-their-environment',
      text: 'Let a workflow spec embed a Task spec under `environment:` so its nodes say where they run.',
      because: ['tasks-run-in-microsandbox-vms'],
      opened: { who: 'franco', when: '2026-11-01' },
      file: af,
    },
    {
      id: 'task-delete-says-what-is-lost',
      text: "Make the panel's Task delete control say that uncommitted work is deleted and mounted volumes are kept.",
      because: ['tasks-run-in-microsandbox-vms'],
      opened: { who: 'franco', when: '2026-11-01' },
      file: af,
    },
    {
      id: 'gitignore-the-local-data-dir',
      text: 'Exclude .agent_composer/ in .gitignore so a project-local secrets.env never enters git.',
      because: ['secrets-are-placeholders-bound-to-hosts'],
      opened: { who: 'franco', when: '2026-11-01' },
      done: { who: 'franco', when: '2026-11-01' },
      file: af,
    },
  ]
  const repo = 'agent-composer code and pull requests'
  const rig = 'the Linux rig'
  const rules: KRule[] = [
    {
      subject: repo,
      id: 'go-no-explicit-semicolons',
      text: 'Never write an explicit `;` in Go outside a 3-clause `for` header.',
      check: 'scripts/check-semicolons.sh',
      because: ['strict-style-makes-agent-output-reviewable'],
      file: 'rules/repo.yaml',
    },
    {
      subject: repo,
      id: 'go-err-naming',
      text: 'Name error variables `err`, not per-call names like `parseErr`, and keep each assignment next to its check.',
      check: '',
      because: ['strict-style-makes-agent-output-reviewable'],
      file: 'rules/repo.yaml',
    },
    {
      subject: repo,
      id: 'go-gofmt-clean',
      text: 'Go files are gofmt clean, except core/resources/hooks/create.go, core/resources/hooks/update.go and core/resources/workflow/compose.go, which are never reformatted.',
      check: 'scripts/check-gofmt.sh',
      because: ['go-has-one-formatting'],
      file: 'rules/repo.yaml',
    },
    {
      subject: repo,
      id: 'rest-endpoint-openapi',
      text: 'A new REST endpoint is documented in `docs/openapi.yaml`.',
      check: '',
      because: [],
      file: 'rules/repo.yaml',
    },
    {
      subject: repo,
      id: 'never-push-from-a-worktree',
      text: 'Never push from an agent worktree; commit there and the main session merges.',
      check: '',
      because: [],
      file: 'rules/repo.yaml',
    },
    {
      subject: rig,
      id: 'rig-services-bind-localhost',
      text: 'Services on the rig bind to 127.0.0.1 and are reached over an SSH tunnel, never on 0.0.0.0.',
      check: '',
      because: ['rig-is-on-home-lan-without-firewall'],
      file: 'rules/rig.yaml',
    },
  ]
  return { dir: '/home/franco/agent-composer', evidence, beliefs, decisions, rules, actions }
}

type KSeed = ReturnType<typeof knowledgeSeed>
type KSet = { because: string[]; evidence: string[] }
type KItem = { id: string; because: string[]; evidence?: string[]; or?: KOr[] }
type KStatus = {
  label: string
  opinions: string[]
  doubted: string[]
  retracted: string[]
  without: string[]
  judgments: string[]
}

// The sets of reasons of an item: its own, then each under or:.
function knowledgeSets(item: KItem): KSet[] {
  return [
    { because: item.because, evidence: item.evidence ?? [] },
    ...(item.or ?? []).map((set) => ({ because: set.because ?? [], evidence: set.evidence ?? [] })),
  ]
}

function knowledgeAdd(list: string[], ids: string[]) {
  for (const id of ids) {
    if (!list.includes(id)) {
      list.push(id)
    }
  }
}

// The seed judged as the files say, or with the beliefs and evidence in
// without set aside (?without=, a what-if). Each set of reasons is judged
// on the leaves under it; a thing is grounded if one of its sets is.
function knowledgeJudge(k: KSeed, without: string[]) {
  const evidenceByID = new Map(k.evidence.map((e) => [e.id, e]))
  const beliefByID = new Map(k.beliefs.map((b) => [b.id, b]))
  const decisionByID = new Map(k.decisions.map((d) => [d.id, d]))
  const live = (id: string) => !evidenceByID.get(id)?.retracted && !without.includes(id)
  const empty = (): KStatus => ({
    label: 'grounded',
    opinions: [],
    doubted: [],
    retracted: [],
    without: [],
    judgments: [],
  })
  const merge = (into: KStatus, from: KStatus) => {
    knowledgeAdd(into.opinions, from.opinions)
    knowledgeAdd(into.doubted, from.doubted)
    knowledgeAdd(into.retracted, from.retracted)
    knowledgeAdd(into.without, from.without)
    knowledgeAdd(into.judgments, from.judgments)
  }
  // The weakest leaf wins: in doubt > opinion > judgment > grounded
  // (contested is applied on top, below in doubt).
  const labelFor = (s: KStatus) =>
    s.doubted.length > 0
      ? 'in_doubt'
      : s.opinions.length > 0
        ? 'opinion'
        : s.judgments.length > 0
          ? 'judgment'
          : 'grounded'

  const judgeSet = (owner: string, set: KSet, isBelief: boolean): KStatus => {
    const status = empty()
    if (isBelief && set.because.length === 0 && set.evidence.length === 0) {
      if (beliefByID.get(owner)?.experience) {
        status.judgments.push(owner)
      } else {
        status.opinions.push(owner)
      }
    }
    if (set.evidence.length > 0 && !set.evidence.some(live)) {
      status.doubted.push(owner)
      for (const id of set.evidence) {
        knowledgeAdd(without.includes(id) ? status.without : status.retracted, [id])
      }
    }
    for (const next of set.because) {
      merge(status, judge(next))
    }
    status.label = labelFor(status)
    return status
  }
  const judgeSets = (owner: string, sets: KSet[], isBelief: boolean): KStatus => {
    const merged = empty()
    for (const set of sets) {
      const status = judgeSet(owner, set, isBelief)
      if (status.label === 'grounded') {
        return empty()
      }
      merge(merged, status)
    }
    merged.label = labelFor(merged)
    return merged
  }
  const judge = (id: string): KStatus => {
    if (without.includes(id)) {
      return { ...empty(), label: 'in_doubt', doubted: [id], without: [id] }
    }
    const belief = beliefByID.get(id)
    if (belief) {
      return judgeSets(id, knowledgeSets(belief), true)
    }
    const decision = decisionByID.get(id)
    return decision ? judgeSets(id, knowledgeSets(decision), false) : empty()
  }

  // Contradictions count from either side.
  const contradicts = new Map<string, string[]>()
  for (const item of [...k.evidence, ...k.beliefs]) {
    for (const other of item.contradicts ?? []) {
      contradicts.set(item.id, [...(contradicts.get(item.id) ?? []), other])
      contradicts.set(other, [...(contradicts.get(other) ?? []), item.id])
    }
  }
  // Live evidence or a grounded belief can contest; live evidence or a
  // belief not in doubt can be contested.
  const stands = (id: string) => (evidenceByID.has(id) ? live(id) : judge(id).label === 'grounded')
  const holds = (id: string) => (evidenceByID.has(id) ? live(id) : judge(id).label !== 'in_doubt')
  const contestedBy = (belief: KBelief) => {
    const targets = [belief.id]
    knowledgeAdd(
      targets,
      knowledgeSets(belief).flatMap((set) => set.evidence.filter(live)),
    )
    const found: string[] = []
    for (const target of targets) {
      for (const other of contradicts.get(target) ?? []) {
        if (other !== belief.id && stands(other)) {
          knowledgeAdd(found, [other])
        }
      }
    }
    return found
  }
  // The live evidence and the opinions a belief comes down to, in every
  // set; a piece of evidence comes down to itself.
  const restsOn = (id: string, seen = new Set<string>()): string[] => {
    if (evidenceByID.has(id)) {
      return [id]
    }
    const belief = beliefByID.get(id)
    if (!belief || seen.has(id)) {
      return []
    }
    seen.add(id)
    const found: string[] = []
    for (const set of knowledgeSets(belief)) {
      if (set.because.length === 0 && set.evidence.length === 0) {
        knowledgeAdd(found, [id])
      }
      knowledgeAdd(found, set.evidence.filter(live))
      for (const next of set.because) {
        knowledgeAdd(found, restsOn(next, seen))
      }
    }
    return found
  }

  const orIDs = (item: KItem) => (item.or ?? []).map((set) => [...(set.because ?? []), ...(set.evidence ?? [])])
  const statusOf = (item: KItem) => {
    const { label, opinions, doubted } = judgeSets(item.id, knowledgeSets(item), false)
    return { label, opinions, doubted }
  }
  const belief = (b: KBelief) => {
    const { label, opinions, doubted, retracted } = judge(b.id)
    const contested_by = label === 'in_doubt' ? [] : contestedBy(b)
    return {
      ...b,
      or: orIDs(b),
      contradicts: b.contradicts ?? [],
      by: b.by ?? '',
      when: b.when ?? '',
      experience: b.experience ?? null,
      label: contested_by.length > 0 ? 'contested' : label,
      opinions,
      doubted,
      retracted,
      contested_by,
    }
  }
  // Decisions, rules and actions are judged on or: too, but the API does
  // not list it for them.
  const withoutOr = <T extends { or?: KOr[] }>(item: T) => {
    const copy = { ...item }
    delete copy.or
    return copy
  }
  const decision = (d: KDecision) => ({ ...withoutOr(d), ...statusOf(d) })
  const rule = (r: KRule) => ({ ...withoutOr(r), kind: r.check ? 'mechanical' : 'manual', ...statusOf(r) })
  const action = (a: KAction) => ({ ...withoutOr(a), done: a.done ?? null, ...statusOf(a) })
  return { belief, decision, rule, action, holds, stands, restsOn }
}

type KWhy = {
  id: string
  kind: string
  text: string
  evidence: {
    id: string
    text: string
    who: string
    when: string
    translated_by: string
    retracted: KRetraction | null
  }[]
  because: KWhy[]
  or: KWhy[][]
  who?: string
  when?: string
  translated_by?: string
  retracted?: KRetraction
}

// id followed down every set of its reasons. Alternatives are lists of
// nodes, beliefs and decisions then evidence; evidence in one is a node of
// kind evidence, with translated_by and retracted only when set.
function knowledgeWhy(k: KSeed, id: string): KWhy {
  const evidence = k.evidence.find((e) => e.id === id)
  if (evidence) {
    const { text, who, when, retracted, translated_by } = evidence
    return {
      id,
      kind: 'evidence',
      text,
      who,
      when,
      ...(translated_by ? { translated_by } : {}),
      ...(retracted ? { retracted } : {}),
      evidence: [],
      because: [],
      or: [],
    }
  }
  const found =
    [
      { kind: 'belief', item: k.beliefs.find((b) => b.id === id) },
      { kind: 'decision', item: k.decisions.find((d) => d.id === id) },
      { kind: 'rule', item: k.rules.find((r) => r.id === id) },
      { kind: 'action', item: k.actions.find((a) => a.id === id) },
    ].find((f) => f.item) ?? null
  if (!found?.item) {
    throw apiError(404, `belief, decision, rule or action ${id} not found`)
  }
  const [first, ...rest] = knowledgeSets(found.item)
  const cited = (ids: string[]) => ids.filter((e) => k.evidence.some((x) => x.id === e))
  return {
    id,
    kind: found.kind,
    text: found.item.text,
    evidence: k.evidence
      .filter((e) => first.evidence.includes(e.id))
      .map(({ id, text, who, when, translated_by, retracted }) => ({
        id,
        text,
        who,
        when,
        translated_by: translated_by ?? '',
        retracted,
      })),
    because: first.because.map((next) => knowledgeWhy(k, next)),
    or: rest.map((set) => [...set.because, ...cited(set.evidence)].map((next) => knowledgeWhy(k, next))),
  }
}

// What rests on id in every set of its reasons, directly or through other
// beliefs and decisions, and apart what rests on it in some sets but still
// has another. id is a belief, a piece of evidence, a decision or a rule.
function knowledgeDoubt(k: KSeed, id: string) {
  const known = [...k.beliefs, ...k.evidence, ...k.decisions, ...k.rules].some((x) => x.id === id)
  if (!known) {
    throw apiError(404, `belief, evidence, decision or rule ${id} not found`)
  }
  const falls = new Set([id])
  const reaches = (set: KSet) => set.evidence.includes(id) || set.because.some((b) => falls.has(b))
  let added = true
  while (added) {
    added = false
    for (const node of [...k.beliefs, ...k.decisions]) {
      if (!falls.has(node.id) && knowledgeSets(node).every(reaches)) {
        falls.add(node.id)
        added = true
      }
    }
  }
  const view = knowledgeJudge(k, [])
  const all = (item: KItem) => knowledgeSets(item).every(reaches)
  const some = (item: KItem) => knowledgeSets(item).some(reaches)
  const others = <T extends KItem>(items: T[]) => items.filter((x) => x.id !== id)
  // A standing item with the sets that do not reach id, each as its ids.
  const standing = (item: KItem & { text: string; file: string }) => ({
    id: item.id,
    text: item.text,
    other: knowledgeSets(item)
      .filter((set) => !reaches(set))
      .map((set) => [...set.because, ...set.evidence]),
    file: item.file,
  })
  const open = k.actions.filter((a) => !a.done)
  return {
    beliefs: others(k.beliefs)
      .filter((b) => falls.has(b.id))
      .map(view.belief),
    decisions: others(k.decisions)
      .filter((d) => falls.has(d.id))
      .map(view.decision),
    rules: k.rules.filter(all).map(view.rule),
    actions: open.filter(all).map(view.action),
    standing: {
      beliefs: others(k.beliefs)
        .filter((b) => !falls.has(b.id) && some(b))
        .map(standing),
      decisions: others(k.decisions)
        .filter((d) => !falls.has(d.id) && some(d))
        .map(standing),
      rules: k.rules.filter((r) => some(r) && !all(r)).map(standing),
      actions: open.filter((a) => some(a) && !all(a)).map(standing),
    },
  }
}

// Every contested contradiction, once, in the order declared.
function knowledgeConflicts(k: KSeed) {
  const view = knowledgeJudge(k, [])
  const side = (id: string) => {
    const e = k.evidence.find((x) => x.id === id)
    if (e) {
      return { id, kind: 'evidence', text: e.text }
    }
    return { id, kind: 'belief', text: k.beliefs.find((b) => b.id === id)?.text ?? id }
  }
  const seen = new Set<string>()
  const conflicts = []
  for (const item of [...k.evidence, ...k.beliefs]) {
    for (const other of item.contradicts ?? []) {
      if (seen.has(`${item.id} ${other}`) || seen.has(`${other} ${item.id}`)) {
        continue
      }
      seen.add(`${item.id} ${other}`)
      const contested = (view.holds(item.id) && view.stands(other)) || (view.holds(other) && view.stands(item.id))
      if (contested) {
        conflicts.push({
          a: side(item.id),
          b: side(other),
          a_rests_on: view.restsOn(item.id),
          b_rests_on: view.restsOn(other),
        })
      }
    }
  }
  return { conflicts }
}

function knowledgeAPI(rest: string[], url: URL) {
  const k = knowledgeSeed()
  if (rest.length === 0) {
    const without = (url.searchParams.get('without') ?? '').split(',').filter(Boolean)
    for (const id of without) {
      if (![...k.beliefs, ...k.evidence].some((x) => x.id === id)) {
        throw apiError(404, `without: ${id} is not a loaded belief or evidence`)
      }
    }
    const view = knowledgeJudge(k, without)
    return {
      dir: k.dir,
      evidence: k.evidence.map((e) => ({
        ...e,
        translated_by: e.translated_by ?? '',
        contradicts: e.contradicts ?? [],
        cited_by: k.beliefs.filter((b) => knowledgeSets(b).some((set) => set.evidence.includes(e.id))).map((b) => b.id),
      })),
      beliefs: k.beliefs.map(view.belief),
      decisions: k.decisions.map(view.decision),
      rules: k.rules.map(view.rule),
      actions: k.actions.map(view.action),
      without,
    }
  }
  if (rest.length === 1 && rest[0] === 'conflicts') {
    return knowledgeConflicts(k)
  }
  if (rest.length === 2 && rest[0] === 'why') {
    return knowledgeWhy(k, rest[1])
  }
  if (rest.length === 2 && rest[0] === 'doubt') {
    return knowledgeDoubt(k, rest[1])
  }
  throw apiError(404, 'no such endpoint')
}

// ---------- agents (persistent agents, proposal) ----------
//
// GET /api/agents, GET /api/agents/:name, POST /api/agents {role, harness?,
// model?} (answers 201 with the numbered agent <role>-<n>, asleep, no home),
// POST /api/agents/:name/keep {name, role?} (names or renames it, sets
// kept), /wake (asleep → waking, then awake after 2s; the first wake sets
// home to the agent's name) and /sleep (awake → asleep). Each state change
// emits agent.state {agent}. A task's status is read live from the tasks.
// Function declarations only, like the knowledge section above.

type MockAgent = {
  name: string
  role: string
  kept: boolean
  harness: string
  model: string
  owner: string
  state: 'awake' | 'asleep' | 'waking'
  home: string | null
  tasks: string[]
  conversations: { channel: string; last_at: string }[]
  last_active_at: string | null
  created_at: string
}

type MockAgents = {
  list: MockAgent[]
  // The highest number handed out per role (lowercased), so a kept
  // agent's number is never reused.
  numbers: Map<string, number>
}

const AGENT_NAME_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/

function agentsOf(state: State): MockAgents {
  const holder = state as State & { agentsMock?: MockAgents }
  if (!holder.agentsMock) {
    holder.agentsMock = agentsSeed()
  }
  return holder.agentsMock
}

function agentsSeed(): MockAgents {
  const ago = (minutes: number) => iso(Date.now() - minutes * 60_000)
  const base = { harness: 'claude_code', model: 'claude-opus-5-5', owner: 'franco' }
  const list: MockAgent[] = [
    {
      ...base,
      name: 'orchestrator-1',
      role: 'Orchestrator',
      kept: false,
      state: 'awake',
      home: 'orchestrator-1',
      tasks: ['review-42', 'flaky-ci'],
      conversations: [
        { channel: '#franco', last_at: ago(3) },
        { channel: '#review-42', last_at: ago(5) },
        { channel: '#flaky-ci', last_at: ago(40) },
      ],
      last_active_at: ago(3),
      created_at: ago(60 * 26),
    },
    {
      ...base,
      name: 'reviewer-2',
      role: 'Reviewer',
      kept: false,
      state: 'asleep',
      home: 'reviewer-2',
      tasks: [],
      conversations: [{ channel: '#franco', last_at: ago(60 * 5) }],
      last_active_at: ago(60 * 5),
      created_at: ago(60 * 30),
    },
    {
      ...base,
      name: 'ada',
      role: 'Reviewer',
      kept: true,
      state: 'asleep',
      home: 'ada',
      tasks: ['docs-pass'],
      conversations: [
        { channel: '#franco', last_at: ago(60 * 20) },
        { channel: '#docs-pass', last_at: ago(60 * 22) },
      ],
      last_active_at: ago(60 * 20),
      created_at: ago(60 * 24 * 4),
    },
    {
      // Seeded waking and left there: the mock only finishes wakes it starts.
      ...base,
      name: 'writer-1',
      role: 'Writer',
      kept: false,
      state: 'waking',
      home: 'writer-1',
      tasks: [],
      conversations: [{ channel: '#franco', last_at: ago(1) }],
      last_active_at: ago(1),
      created_at: ago(2),
    },
  ]
  // ada was reviewer-1.
  const numbers = new Map([
    ['orchestrator', 1],
    ['reviewer', 2],
    ['writer', 1],
  ])
  return { list, numbers }
}

function agentView(state: State, agent: MockAgent) {
  return {
    ...agent,
    tasks: agent.tasks.map((name) => {
      const task = state.tasks.get(name)
      return { name, status: task ? state.taskView(task).status : 'deleted' }
    }),
    conversations: agent.conversations.map((c) => ({ ...c })),
  }
}

function findAgent(state: State, name: string) {
  const agent = agentsOf(state).list.find((a) => a.name === name)
  if (!agent) {
    throw apiError(404, `agent ${name} not found`)
  }
  return agent
}

function emitAgent(state: State, agent: MockAgent) {
  state.emit('agent.state', { agent: agentView(state, agent) })
}

async function agentsAPI(state: State, method: string, rest: string[], req: IncomingMessage) {
  const mock = agentsOf(state)
  if (rest.length === 0 && method === 'GET') {
    return { agents: mock.list.map((a) => agentView(state, a)) }
  }
  if (rest.length === 0 && method === 'POST') {
    const body = await readJSON(req, apiError)
    const role = str(body.role).trim()
    if (!role) {
      throw apiError(400, 'role is required')
    }
    const harness = str(body.harness).trim() || 'claude_code'
    if (!HARNESSES.includes(harness)) {
      throw apiError(400, `unknown harness ${harness}`)
    }
    const model = str(body.model).trim() || (harness === 'claude_code' ? 'claude-opus-5-5' : '')
    const prefix = slugify(role)
    if (!prefix) {
      throw apiError(400, `role ${role} has no letters or digits to name an agent from`)
    }
    const n = (mock.numbers.get(prefix) ?? 0) + 1
    mock.numbers.set(prefix, n)
    const now = iso()
    const agent: MockAgent = {
      name: `${prefix}-${n}`,
      role,
      kept: false,
      harness,
      model,
      owner: 'franco',
      state: 'asleep',
      home: null,
      tasks: [],
      conversations: [{ channel: '#franco', last_at: now }],
      last_active_at: null,
      created_at: now,
    }
    mock.list.push(agent)
    return new Created(agentView(state, agent))
  }

  const agent = findAgent(state, rest[0])
  if (rest.length === 1 && method === 'GET') {
    return agentView(state, agent)
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'keep') {
    const body = await readJSON(req, apiError)
    const name = str(body.name).trim()
    if (!AGENT_NAME_RE.test(name)) {
      throw apiError(400, 'name must be lowercase letters, digits and dashes')
    }
    if (name !== agent.name && mock.list.some((a) => a.name === name)) {
      throw apiError(409, `agent ${name} already exists`)
    }
    const role = str(body.role).trim()
    agent.name = name
    if (role) {
      agent.role = role
    }
    agent.kept = true
    return agentView(state, agent)
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'wake') {
    if (agent.state !== 'asleep') {
      throw apiError(409, `agent ${agent.name} is ${agent.state}; only an asleep agent can be woken`)
    }
    agent.state = 'waking'
    agent.home = agent.home ?? agent.name
    emitAgent(state, agent)
    setTimeout(() => {
      if (agent.state !== 'waking') {
        return
      }
      agent.state = 'awake'
      agent.last_active_at = iso()
      emitAgent(state, agent)
    }, BOOT_MS)
    return agentView(state, agent)
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'sleep') {
    if (agent.state !== 'awake') {
      throw apiError(409, `agent ${agent.name} is ${agent.state}; only an awake agent can sleep`)
    }
    agent.state = 'asleep'
    emitAgent(state, agent)
    return agentView(state, agent)
  }
  throw apiError(404, 'no such endpoint')
}

// Direct-message channels, @<human>/<task> (another task adds them to agc).
const DM_RE = /^@[a-z0-9][a-z0-9_-]{0,63}\/[a-z0-9][a-z0-9_-]{0,63}$/

// GET /api/defaults (ADR 0009, ahead of the Go side): what fills in a task
// and who the server's owner and reserved system participant are.
const DEFAULTS = { harness: 'claude_code', model: 'claude-opus-5-5', owner: 'franco', system: 'agc' }

// GET /api/msg/participants?all=1, as Service.AllParticipants: everyone who
// ever joined, in any channel or none, active first, then departed, by name.
function allParticipants(state: State) {
  return [...state.participants.values()]
    .sort((a, b) => Number(Boolean(a.left_at)) - Number(Boolean(b.left_at)) || a.name.localeCompare(b.name))
    .map((p) => ({ ...p, ...presenceOf(state, p) }))
}

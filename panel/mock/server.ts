// Mock agc API for developing the panel without agc or a VM.
//
//   npm run mock                 # http://127.0.0.1:8790/api
//   npm run mock -- --port 9000
//
// Everything lives in memory and resets on restart. Shapes and error
// messages follow the real handlers: tasks, specs and secrets answer errors
// as {"error":{"code","message","request_id"}}, messaging (/api/msg/*) as
// {"error":"..."}. The terminal WebSocket echoes what you type and answers
// resize frames, so a terminal can be exercised end to end.
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

type Task = {
  id: string
  title: string
  name: string
  spec: TaskSpec
  status: string
  status_error?: string
  created_at: string
  updated_at: string
}

type Terminal = { id: string; command: string[]; running: boolean }

type Participant = {
  id: string
  name: string
  task: string
  human: boolean
  joined_at: string
  left_at?: string
}

type Message = {
  id: number
  channel: string
  from: string
  participant_id: string
  task: string
  reply_to?: number
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
  const readyAt = new Map<string, number>()
  const terminals = new Map<string, Terminal[]>()
  const specs = new Map<string, string>()
  const participants = new Map<string, Participant>()
  const messages: Message[] = []
  let waiters: Waiter[] = []
  let nextMessageId = 1

  function addTask(title: string, name: string, spec: TaskSpec, status: string, ageMs: number, statusError = '') {
    const at = iso(Date.now() - ageMs)
    const task: Task = { id: randomUUID(), title, name, spec, status, created_at: at, updated_at: at }
    if (statusError) {
      task.status_error = statusError
    }
    tasks.set(name, task)
    terminals.set(name, [])
    return task
  }

  // A started task reports "starting" for two seconds, like a VM booting.
  function view(task: Task) {
    const ready = readyAt.get(task.name)
    if (task.status === 'starting' && ready !== undefined && Date.now() >= ready) {
      task.status = 'running'
      task.updated_at = iso()
      readyAt.delete(task.name)
    }
    return task
  }

  // A task is found by UUID or by name, like TaskIdParam.
  function findTask(ref: string) {
    const byName = tasks.get(ref)
    if (byName) {
      return view(byName)
    }
    for (const task of tasks.values()) {
      if (task.id === ref) {
        return view(task)
      }
    }
    throw apiError(404, `task ${ref} not found`)
  }

  function join(name: string, task: string, human: boolean) {
    const p: Participant = { id: 'p_' + hex(8), name, task, human, joined_at: iso() }
    participants.set(p.id, p)
    return p
  }

  function post(channel: string, p: Participant, body: string, replyTo?: number) {
    const m: Message = {
      id: nextMessageId,
      channel,
      from: p.name,
      participant_id: p.id,
      task: p.task,
      body,
      created_at: iso(),
    }
    if (replyTo !== undefined) {
      m.reply_to = replyTo
    }
    nextMessageId += 1
    messages.push(m)
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
    addTask('Review 42', 'review-42', base, 'running', 40 * 60_000)
    addTask('Flaky CI', 'flaky-ci', { image: 'agc-agent', cpus: 2, memory: 4096 }, 'stopped', 3 * 3600_000)
    // A task from before titles existed (title equals name) whose runtime
    // could not be asked.
    addTask(
      'docs-pass',
      'docs-pass',
      { ...base, cpus: 1, memory: 2048, ref: 'docs' },
      'unknown',
      2 * 86_400_000,
      'msb: connection refused',
    )
    terminals.set('review-42', [
      { id: 'job_' + hex(4), command: ['claude'], running: true },
      { id: 'job_' + hex(4), command: ['bash'], running: false },
    ])
    specs.set(
      'backend-review',
      'image: agc-agent\ncpus: 2\nmemory: 4096\nrepo: github.com/vanclief/agent-composer\nref: master\n',
    )

    const reviewer = join('reviewer', 'review-42', false)
    const tester = join('tester', 'review-42', false)
    const franco = join('franco', '', true)
    const first = post('#review-42', reviewer, 'Cloned the repo and ran the tests. 3 failures in workflow/.')
    post('#review-42', franco, 'Look at the loop node first.', first.id)
    post('#review-42', tester, 'Reproduced 2 of 3 locally. The third looks flaky.')
    post('#review-42', reviewer, 'Fixed the loop node. Pushing to review-42.')
    post('#releases', reviewer, 'v0.4.0 tagged.')
    // The seeded human has left, so the panel can join as @franco.
    franco.left_at = iso()
  }

  seed()

  return {
    tasks,
    readyAt,
    terminals,
    specs,
    participants,
    messages,
    view,
    findTask,
    addTask,
    join,
    post,
    after,
    addWaiter(w: Waiter) {
      waiters.push(w)
    },
    removeWaiter(w: Waiter) {
      waiters = waiters.filter((x) => x !== w)
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
    const list = [...state.tasks.values()].map(state.view)
    list.sort((a, b) => b.created_at.localeCompare(a.created_at))
    return { tasks: list }
  }
  if (rest.length === 0 && method === 'POST') {
    return createTask(state, await readJSON(req, apiError))
  }

  const task = state.findTask(rest[0])
  if (rest.length === 1 && method === 'GET') {
    return task
  }
  if (rest.length === 1 && method === 'DELETE') {
    state.tasks.delete(task.name)
    state.terminals.delete(task.name)
    return { deleted: true, name: task.name }
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'stop') {
    if (task.status !== 'running') {
      throw apiError(409, `task ${task.name} is ${task.status}, not running`)
    }
    task.status = 'stopped'
    task.updated_at = iso()
    for (const job of state.terminals.get(task.name) ?? []) {
      job.running = false
    }
    return task
  }
  if (rest.length === 2 && method === 'POST' && rest[1] === 'start') {
    if (task.status === 'running') {
      throw apiError(409, `task ${task.name} is already running`)
    }
    task.status = 'starting'
    delete task.status_error
    task.updated_at = iso()
    state.readyAt.set(task.name, Date.now() + BOOT_MS)
    return task
  }
  if (rest.length === 2 && rest[1] === 'terminals') {
    if (task.status !== 'running') {
      throw apiError(409, `task ${task.name} is ${task.status}; start it to use terminals`)
    }
    const jobs = state.terminals.get(task.name) ?? []
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
      state.terminals.set(task.name, jobs)
      return { id: job.id }
    }
  }
  throw apiError(404, 'no such endpoint')
}

function createTask(state: State, body: Record<string, unknown>) {
  const title = str(body.title).trim()
  const name = str(body.name) || slugify(title)
  const spec = (body.spec ?? {}) as TaskSpec
  if (!name) {
    throw apiError(400, `title "${title}" needs at least one letter or digit`)
  }
  if (!TASK_NAME_RE.test(name)) {
    throw apiError(400, `name "${name}" is invalid: use 1-63 lowercase letters, digits or -, starting with a letter`)
  }
  if (state.tasks.has(name)) {
    throw apiError(409, `task ${name} already exists`)
  }
  if (typeof spec !== 'object' || spec === null || Array.isArray(spec)) {
    throw apiError(400, 'spec must be an object')
  }
  const known = new Set(SECRETS.map((s) => s.name))
  for (const entry of spec.secrets ?? []) {
    const secret = String(entry).split('@')[0]
    if (!known.has(secret)) {
      throw apiError(
        400,
        `secret ${secret} is not defined; add it to ~/.agent_composer/config/secrets.env and try again`,
      )
    }
  }
  for (const channel of spec.channels ?? []) {
    if (!CHANNEL_RE.test(String(channel).replace(/:read$/, ''))) {
      throw apiError(400, `channel "${channel}" is invalid: start with # and use lowercase letters, digits, _ or -`)
    }
  }
  const task = state.addTask(title || name, name, spec, 'starting', 0)
  state.readyAt.set(name, Date.now() + BOOT_MS)
  return task
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
  if (!CHANNEL_RE.test(channel)) {
    throw msgError(400, `channel "${channel}" is invalid: use # followed by 1-64 lowercase letters, digits, _ or -`)
  }
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
    return state.join(name, '', true)
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
    return state.post(channel, p, text, replyTo)
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

  try {
    let body: unknown
    if (parts[1] === 'tasks') {
      body = await tasksAPI(state, method, parts.slice(2), req)
    } else if (parts[1] === 'specs') {
      body = await specsAPI(state, method, parts.slice(2), req)
    } else if (parts[1] === 'secrets' && parts.length === 2 && method === 'GET') {
      body = { secrets: SECRETS }
    } else if (parts[1] === 'msg' && parts.length === 3) {
      body = await msgAPI(state, method, parts[2], url, req)
    } else {
      throw apiError(404, 'no such endpoint')
    }
    if (!res.destroyed) {
      send(res, 200, body)
    }
  } catch (err) {
    if (err instanceof HttpError) {
      send(res, err.status, err.body)
      return
    }
    console.error(err)
    send(res, 500, { error: { code: 'EINTERNAL', message: String(err), request_id: 'mock-' + hex(4) } })
  }
}

// /api/tasks/:id/terminals/:job/ws. Binary frames are terminal bytes both
// ways; a text frame {"type":"resize","rows":N,"cols":N} resizes. The mock
// echoes input and acknowledges resizes. A finished job replays a line and
// closes with "job completed", like the real socket.
function attachTerminal(ws: WebSocket, task: Task, job: Terminal) {
  const write = (text: string) => ws.send(Buffer.from(text), { binary: true })

  write(`\x1b[2mmock terminal · ${task.name} · ${job.command.join(' ')}\x1b[0m\r\n`)
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
    const match = /^\/api\/tasks\/([^/]+)\/terminals\/([^/]+)\/ws$/.exec(url.pathname)
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
    let task: Task
    try {
      task = state.findTask(decodeURIComponent(match[1]))
    } catch (err) {
      const message =
        err instanceof HttpError ? (err.body as { error: { message: string } }).error.message : String(err)
      refuse(404, message)
      return
    }
    if (task.status !== 'running') {
      refuse(409, `task ${task.name} is ${task.status}; start it to use terminals`)
      return
    }
    const jobID = decodeURIComponent(match[2])
    const job = (state.terminals.get(task.name) ?? []).find((j) => j.id === jobID)
    if (!job) {
      refuse(404, `terminal ${jobID} not found`)
      return
    }
    wss.handleUpgrade(req, socket, head, (ws) => attachTerminal(ws, task, job))
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

// The create dialog edits a SpecForm; the TaskSpec sent to the server and
// saved as YAML is always derived from it with formToSpec. Ported from the
// previous Alpine panel (control_panel/app.js in git history before 2026-11-02).
import { dump, load } from 'js-yaml'

import type { TaskSpec } from '@/api/client'

export const DEFAULT_IMAGE = 'agc-agent'
export const CPU_CHOICES = [1, 2, 4, 8]
// MiB, shown as GB.
export const MEMORY_CHOICES = [1, 2, 4, 8, 16].map((gb) => gb * 1024)

export type SecretPick = {
  name: string
  // The variable it arrives as in the sandbox (NAME as ENV); empty: the
  // name itself.
  env: string
  // Comma-separated, as typed.
  hosts: string
  // The loaded spec set its own hosts, so the hosts input stays visible
  // even when the server knows hosts for the secret.
  custom: boolean
}
export type EnvRow = { id: number; key: string; value: string }
export type VolumeKind = 'named' | 'host'
export type VolumeRow = { id: number; kind: VolumeKind; source: string; dest: string; rw: boolean }
export type ChannelPick = { name: string; read: boolean }

export type SpecForm = {
  image: string
  // null leaves it to the server default.
  cpus: number | null
  memory: number | null
  repo: string
  ref: string
  setup: string
  secrets: SecretPick[]
  env: EnvRow[]
  volumes: VolumeRow[]
  channels: ChannelPick[]
}

// Same rule as Slug in models/task/task.go: "Review facebook ads" becomes
// review-facebook-ads, "42 things" task-42-things. Empty means the title
// has no letters or digits.
export function slugify(title: string) {
  let slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  if (/^[0-9]/.test(slug)) {
    slug = 'task-' + slug
  }
  if (slug.length > 63) {
    slug = slug.slice(0, 63).replace(/-+$/, '')
  }
  return slug
}

// Where the repo is cloned in the VM, '' without a repo.
export function repoDir(repo: string) {
  const last =
    repo
      .trim()
      .replace(/\/+$/, '')
      .split('/')
      .pop()
      ?.replace(/\.git$/, '') ?? ''
  return last ? '/home/agent/' + last : ''
}

export function memoryLabel(mib: number) {
  return mib % 1024 === 0 ? `${mib / 1024} GB` : `${mib} MiB`
}

// Keys for env and volume rows, which have no natural identity.
let lastRowId = 0
export function rowId() {
  lastRowId += 1
  return lastRowId
}

export function blankForm(): SpecForm {
  return {
    image: DEFAULT_IMAGE,
    cpus: 2,
    memory: 4096,
    repo: '',
    ref: '',
    setup: '',
    secrets: [],
    env: [],
    volumes: [],
    channels: [],
  }
}

function asText(v: unknown) {
  return v === undefined || v === null ? '' : String(v)
}

function asTextList(v: unknown) {
  return Array.isArray(v) ? v.map(asText).filter((x) => x.trim()) : []
}

function asCount(v: unknown) {
  const n = Number(v)
  return v === undefined || v === null || v === '' || !Number.isFinite(n) || n <= 0 ? null : n
}

// NAME, NAME@host1,host2, NAME as ENV or NAME as ENV@host1,host2.
export function parseSecret(entry: string): SecretPick {
  const at = entry.indexOf('@')
  const head = at < 0 ? entry : entry.slice(0, at)
  const as = head.match(/^\s*(\S+)\s+as\s+(\S+)\s*$/)
  const name = as ? as[1] : head.trim()
  const env = as ? as[2] : ''
  if (at < 0) {
    return { name, env, hosts: '', custom: false }
  }
  const hosts = splitHosts(entry.slice(at + 1))
  return { name, env, hosts: hosts.join(', '), custom: true }
}

function splitHosts(hosts: string) {
  return hosts
    .split(',')
    .map((h) => h.trim())
    .filter(Boolean)
}

export function secretEntry(s: SecretPick) {
  const hosts = splitHosts(s.hosts)
  const env = s.env.trim()
  const head = env && env !== s.name ? s.name + ' as ' + env : s.name
  return hosts.length ? head + '@' + hosts.join(',') : head
}

// NAME:DEST[:rw] or host:PATH:DEST[:rw]; read-only unless :rw.
function parseVolume(entry: string): Omit<VolumeRow, 'id'> {
  const parts = entry.split(':')
  let rw = false
  const last = parts[parts.length - 1]
  if (parts.length > 2 && (last === 'rw' || last === 'ro')) {
    rw = parts.pop() === 'rw'
  }
  if (parts[0] === 'host' && parts.length >= 3) {
    return { kind: 'host', source: parts[1], dest: parts.slice(2).join(':'), rw }
  }
  return { kind: 'named', source: parts[0], dest: parts.slice(1).join(':'), rw }
}

function volumeEntry(v: VolumeRow) {
  const source = v.source.trim()
  const head = v.kind === 'host' ? 'host:' + source : source
  return head + ':' + v.dest.trim() + (v.rw ? ':rw' : '')
}

// #name or #name:read; the # is optional when typed.
export function parseChannel(entry: string): ChannelPick {
  let name = entry.trim()
  const read = name.endsWith(':read')
  if (read) {
    name = name.slice(0, -':read'.length)
  }
  if (!name.startsWith('#')) {
    name = '#' + name
  }
  return { name, read }
}

// Secrets and channels are keyed by name.
function firstByName<T extends { name: string }>(list: T[]) {
  return list.filter((x, i) => list.findIndex((y) => y.name === x.name) === i)
}

// Throws when the spec is not a mapping. Unknown keys are dropped.
export function specToForm(spec: unknown): SpecForm {
  let s = (spec ?? {}) as Record<string, unknown>
  if (typeof s !== 'object' || Array.isArray(s)) {
    throw new Error('The spec must be a YAML mapping (key: value lines).')
  }
  // Accept the workflow form too: a single top-level environment block.
  const keys = Object.keys(s)
  if (keys.length === 1 && typeof s.environment === 'object' && s.environment !== null) {
    s = s.environment as Record<string, unknown>
  }
  const env = s.env && typeof s.env === 'object' && !Array.isArray(s.env) ? (s.env as Record<string, unknown>) : {}
  return {
    image: asText(s.image).trim() || DEFAULT_IMAGE,
    cpus: asCount(s.cpus),
    memory: asCount(s.memory),
    repo: asText(s.repo),
    ref: asText(s.ref),
    setup: asText(s.setup),
    secrets: firstByName(asTextList(s.secrets).map(parseSecret)),
    env: Object.entries(env).map(([key, value]) => ({ id: rowId(), key, value: asText(value) })),
    volumes: asTextList(s.volumes).map((v) => ({ id: rowId(), ...parseVolume(v) })),
    channels: firstByName(asTextList(s.channels).map(parseChannel)),
  }
}

// Empty fields are omitted; an env row without a key is dropped.
export function formToSpec(f: SpecForm): TaskSpec {
  const spec: TaskSpec = { image: f.image.trim() || DEFAULT_IMAGE }
  if (f.cpus) {
    spec.cpus = f.cpus
  }
  if (f.memory) {
    spec.memory = f.memory
  }
  if (f.repo.trim()) {
    spec.repo = f.repo.trim()
  }
  if (f.ref.trim()) {
    spec.ref = f.ref.trim()
  }
  if (f.setup.trim()) {
    spec.setup = f.setup
  }
  if (f.secrets.length) {
    spec.secrets = f.secrets.map(secretEntry)
  }
  const env: Record<string, string> = {}
  for (const e of f.env) {
    if (e.key.trim()) {
      env[e.key.trim()] = e.value
    }
  }
  if (Object.keys(env).length) {
    spec.env = env
  }
  const volumes = f.volumes.filter((v) => v.source.trim() || v.dest.trim()).map(volumeEntry)
  if (volumes.length) {
    spec.volumes = volumes
  }
  if (f.channels.length) {
    spec.channels = f.channels.map((c) => c.name + (c.read ? ':read' : ''))
  }
  return spec
}

export function specYAML(spec: TaskSpec) {
  return dump(spec, { lineWidth: -1 })
}

// Parses YAML text into a form; error is the parser's message, verbatim.
export function parseSpecYAML(yaml: string): { form: SpecForm; error: null } | { form: null; error: Error } {
  try {
    return { form: specToForm(load(yaml)), error: null }
  } catch (e) {
    return { form: null, error: e instanceof Error ? e : new Error(String(e)) }
  }
}

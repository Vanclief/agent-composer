// Display helpers for tasks and their specs. Spec entries are parsed the
// way the server reads them (see TaskSpec in docs/openapi.yaml).

// Memory in the unit humans think in: 2048 MiB is "2 GB", 1536 is "1.5 GB".
export function formatMemory(mib: number) {
  return `${Number((mib / 1024).toFixed(2))} GB`
}

// A short age such as "now", "40m", "3h" or "2d".
export function shortAge(iso: string, now = Date.now()) {
  const at = Date.parse(iso)
  if (Number.isNaN(at)) {
    return iso
  }
  const seconds = Math.max(0, Math.floor((now - at) / 1000))
  if (seconds < 60) {
    return 'now'
  }
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}m`
  }
  if (seconds < 86_400) {
    return `${Math.floor(seconds / 3600)}h`
  }
  return `${Math.floor(seconds / 86_400)}d`
}

// Where the repo is cloned: github.com/vanclief/agent-composer and
// https://github.com/vanclief/agent-composer.git both land in
// /home/agent/agent-composer.
export function repoPath(repo: string) {
  const base = repo
    .trim()
    .replace(/\/+$/, '')
    .replace(/\.git$/, '')
    .split('/')
    .pop()
  return `/home/agent/${base ?? ''}`
}

export function lineCount(text: string) {
  return text.replace(/\n+$/, '').split('\n').length
}

// NAME or NAME@host1,host2. hosts is empty when the entry names none.
export function parseSecret(entry: string) {
  const at = entry.indexOf('@')
  if (at < 0) {
    return { name: entry.trim(), hosts: [] as string[] }
  }
  const hosts = entry
    .slice(at + 1)
    .split(',')
    .map((h) => h.trim())
    .filter(Boolean)
  return { name: entry.slice(0, at).trim(), hosts }
}

// NAME:DEST[:rw] or host:PATH:DEST[:rw]; read-only unless :rw.
export function parseVolume(entry: string) {
  const parts = entry.split(':')
  let readOnly = true
  const last = parts[parts.length - 1]
  if (parts.length > 2 && (last === 'rw' || last === 'ro')) {
    readOnly = parts.pop() !== 'rw'
  }
  if (parts[0] === 'host' && parts.length >= 3) {
    return { host: true, source: parts[1], destination: parts.slice(2).join(':'), readOnly }
  }
  return { host: false, source: parts[0], destination: parts.slice(1).join(':'), readOnly }
}

// #name or #name:read.
export function parseChannel(entry: string) {
  let name = entry.trim()
  const readOnly = name.endsWith(':read')
  if (readOnly) {
    name = name.slice(0, -':read'.length)
  }
  return { name: name.replace(/^#/, ''), readOnly }
}

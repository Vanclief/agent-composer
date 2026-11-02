// Terminals in a sandbox (ADR 0008). Not in docs/openapi.yaml, so typed by
// hand from interfaces/rest/handler/terminals.go. sandbox is an id or name.
import { apiFetch } from './client'

// A TTY job in the sandbox, as the runtime reports it. The command a
// sandbox is allocated with runs as the job "agent".
export type Terminal = {
  id: string
  command: string[]
  running: boolean
}

export type StartTerminalRequest = {
  // Split on whitespace, e.g. "claude --resume".
  command: string
  // Working directory; empty means the VM user's default.
  dir?: string
}

const enc = encodeURIComponent

export const terminals = {
  list(sandbox: string, signal?: AbortSignal) {
    return apiFetch<{ terminals: Terminal[] }>('GET', `/sandboxes/${enc(sandbox)}/terminals`, undefined, signal)
  },
  start(sandbox: string, request: StartTerminalRequest) {
    return apiFetch<{ id: string }>('POST', `/sandboxes/${enc(sandbox)}/terminals`, request)
  },
}

// terminalSocketURL is the WebSocket for one job. Binary frames carry
// terminal bytes both ways; send the text frame
// {"type":"resize","rows":N,"cols":N} to resize. The server closes with
// reason "job completed" when the program exits; closing the socket only
// detaches and the job keeps running.
export function terminalSocketURL(sandbox: string, job: string) {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${window.location.host}/api/sandboxes/${enc(sandbox)}/terminals/${enc(job)}/ws`
}

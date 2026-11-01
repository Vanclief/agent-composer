// Terminals in a Task's VM. Not in docs/openapi.yaml, so typed by hand from
// interfaces/rest/handler/terminals.go and core/resources/tasks/terminals.go.
import { apiFetch } from './client'

// A TTY job in the VM, as the runtime reports it.
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
  list(task: string) {
    return apiFetch<{ terminals: Terminal[] }>('GET', `/tasks/${enc(task)}/terminals`)
  },
  start(task: string, request: StartTerminalRequest) {
    return apiFetch<{ id: string }>('POST', `/tasks/${enc(task)}/terminals`, request)
  },
}

// terminalSocketURL is the WebSocket for one job. Binary frames carry
// terminal bytes both ways; send the text frame
// {"type":"resize","rows":N,"cols":N} to resize. The server closes with
// reason "job completed" when the program exits; closing the socket only
// detaches and the job keeps running.
export function terminalSocketURL(task: string, job: string) {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${window.location.host}/api/tasks/${enc(task)}/terminals/${enc(job)}/ws`
}

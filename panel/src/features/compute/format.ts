// Display helpers for sandboxes.
import type { SandboxView } from '@/api/tasks'

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

// The moment that matters for the sandbox now: when it stopped, when it
// booted, or (still starting, or failed) when it was allocated.
export function sandboxMoment(sandbox: SandboxView) {
  if (sandbox.stopped_at) {
    return { at: sandbox.stopped_at, label: 'Stopped' }
  }
  if (sandbox.booted_at) {
    return { at: sandbox.booted_at, label: 'Booted' }
  }
  return { at: sandbox.created_at, label: 'Allocated' }
}

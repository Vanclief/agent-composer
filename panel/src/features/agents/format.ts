// Display helpers for agents.
import type { AgentView } from '@/api/agents'
import { shortAge } from '@/features/tasks/format'

// Awake first, then waking, then asleep, then any other word the backend
// sends; most recently active first inside each.
const ORDER = ['awake', 'waking', 'asleep']

export function sortAgents(list: AgentView[]) {
  const rank = (state: string) => {
    const i = ORDER.indexOf(state)
    return i < 0 ? ORDER.length : i
  }
  return [...list].sort(
    (a, b) =>
      rank(a.state) - rank(b.state) ||
      (b.last_active_at ?? '').localeCompare(a.last_active_at ?? '') ||
      a.name.localeCompare(b.name),
  )
}

// "last active 3m ago", "last active now" or "never active".
export function lastActive(at: string | null) {
  if (!at) {
    return 'never active'
  }
  const age = shortAge(at)
  return age === 'now' ? 'last active now' : `last active ${age} ago`
}

// An age with "ago", or "now".
export function ago(at: string) {
  const age = shortAge(at)
  return age === 'now' ? 'now' : `${age} ago`
}

// The DM with the owner first, then the rest as the server sent them.
export function dmFirst(agent: AgentView) {
  const dm = `#${agent.owner}`
  return [...agent.conversations].sort((a, b) => Number(b.channel === dm) - Number(a.channel === dm))
}

// What Keep… accepts as a name.
export const AGENT_NAME_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/

export const ROLE_SUGGESTIONS = ['Reviewer', 'Orchestrator', 'Writer']

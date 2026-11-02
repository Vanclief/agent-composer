// /agents: every persistent agent, one row each: state (verbatim), name
// (bold when kept, muted and "numbered" while not), role, harness and
// model (with its effort), how many tasks it manages and when it was last active. Awake
// first, then waking, then asleep. "New agent" makes a numbered one.
import { useMemo, useState } from 'react'
import { Clock, Plus } from 'lucide-react'
import { Link } from 'react-router'

import type { AgentView } from '@/api/agents'
import { useAgents } from '@/api/hooks'
import { Avatar } from '@/components/avatar'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { KeptBadge, RoleChip } from './AgentChips'
import { NewAgentDialog } from './AgentDialogs'
import { lastActive, sortAgents } from './format'

const color = conceptColors.agent

export function AgentsPage() {
  usePageTitle('Agents')
  const agents = useAgents()
  const [creating, setCreating] = useState(false)
  const sorted = useMemo(() => sortAgents(agents.data ?? []), [agents.data])

  return (
    <div className="flex flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <span
          title="Agents"
          data-testid="agent-count"
          className="inline-flex items-center gap-1.5 text-sm text-zinc-500 tabular-nums dark:text-zinc-400"
        >
          <conceptIcons.agent className={cn('size-4', color.text)} />
          {agents.data?.length ?? '–'}
        </span>
        <Button onClick={() => setCreating(true)} className={color.solid}>
          <Plus /> New agent
        </Button>
      </div>
      <ErrorText error={agents.error} />
      {agents.isPending && (
        <div className="flex flex-col gap-2" aria-label="Loading agents">
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
        </div>
      )}
      {agents.data?.length === 0 && (
        <EmptySlot
          reason="No agents yet. An agent is born from a role and gets a number; keep the ones worth keeping."
          className="self-start"
        />
      )}
      {sorted.length > 0 && (
        <ul aria-label="Agents" className="flex flex-col gap-2">
          {sorted.map((agent) => (
            <AgentRow key={agent.name} agent={agent} />
          ))}
        </ul>
      )}
      <NewAgentDialog open={creating} onOpenChange={setCreating} />
    </div>
  )
}

// The whole row opens the agent. Two lines: the title (avatar, name,
// numbered, role, tasks managed, last active at the far right), then owner,
// harness and model as readouts. The state sits first; the Eva look draws it
// as a tab fused into the plate's left edge (styles/eva-agents.css).
function AgentRow({ agent }: { agent: AgentView }) {
  const count = agent.tasks.length
  return (
    <li
      data-agent={agent.name}
      data-kept={agent.kept ? 'true' : 'false'}
      className={cn(
        'relative flex flex-col gap-1 rounded-xl border bg-white px-4 py-2.5 text-sm shadow-sm transition-colors hover:bg-pink-500/5 dark:bg-zinc-900/60 dark:hover:bg-pink-500/10',
        color.border,
      )}
    >
      <div data-slot="title" className="flex items-center gap-3">
        <div data-testid="agent-state" className="flex w-20 shrink-0">
          <StatusPill status={agent.state} />
        </div>
        <Avatar name={agent.name} size="sm" presence={agent.state === 'awake'} />
        <Link
          to={`/agents/${encodeURIComponent(agent.name)}`}
          className={cn(
            'truncate font-mono after:absolute after:inset-0 after:content-[""]',
            agent.kept ? 'font-semibold' : 'text-muted-foreground',
          )}
        >
          {agent.name}
        </Link>
        {!agent.kept && <KeptBadge kept={false} />}
        <RoleChip role={agent.role} />
        <div data-slot="right" className="ml-auto flex shrink-0 items-center gap-3">
          <span
            data-testid="agent-tasks"
            title={`Manages ${count} ${count === 1 ? 'task' : 'tasks'}`}
            className="inline-flex shrink-0 items-center gap-1 text-xs text-zinc-600 tabular-nums dark:text-zinc-400"
          >
            <conceptIcons.task aria-hidden className={cn('size-3.5', conceptColors.task.text)} />
            {count}
          </span>
          <span className="relative z-10 shrink-0">
            <IconChip
              icon={Clock}
              title={
                agent.last_active_at ? `Last active ${new Date(agent.last_active_at).toLocaleString()}` : undefined
              }
              className="font-sans"
            >
              {lastActive(agent.last_active_at)}
            </IconChip>
          </span>
        </div>
      </div>
      <div
        data-slot="meta"
        className="flex flex-wrap items-center gap-x-4 gap-y-1 pl-[5.75rem] text-xs text-muted-foreground"
      >
        <Readout label="owner" title={`Owner ${agent.owner}`} value={agent.owner ? `@${agent.owner}` : '–'} />
        <Readout label="harness" title="Harness" value={agent.harness} />
        <Readout
          label="model"
          title={agent.effort ? `Model, effort ${agent.effort}` : 'Model'}
          value={agent.effort ? `${agent.model || '–'} · ${agent.effort}` : agent.model || '–'}
        />
      </div>
    </li>
  )
}

// A labelled monospace value: "harness claude_code".
function Readout({ label, title, value }: { label: string; title: string; value: string }) {
  return (
    <span data-readout={label} title={title} className="inline-flex items-baseline gap-1.5 font-mono">
      <span className="text-[0.65rem] uppercase opacity-70">{label}</span>
      <span>{value}</span>
    </span>
  )
}

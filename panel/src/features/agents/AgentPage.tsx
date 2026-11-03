// /agents/:name: one agent: a header with its state (verbatim), name, role
// and whether it is kept, and its actions (Wake when asleep, Sleep when
// awake, disabled while waking; Keep… for a numbered agent, Rename… for a
// kept one). Cards: its home sandbox, the tasks it manages, the channels
// it reads (the DM with its owner first), who it is (model with its
// effort), the skills it has (a tile each, opening the skill's text) and the
// rules it follows (a row each).
import { useState, type ReactNode } from 'react'
import { Clock, Loader2, Moon, Pencil, Sun, UserCheck } from 'lucide-react'
import { Link, useParams } from 'react-router'

import type { AgentView } from '@/api/agents'
import { useAgent, useSkillText, useSleepAgent, useWakeAgent } from '@/api/hooks'
import { Avatar } from '@/components/avatar'
import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/shell/page-title'

import { KeptBadge, RoleChip } from './AgentChips'
import { KeepAgentDialog } from './AgentDialogs'
import { ago, dmFirst, lastActive } from './format'

const color = conceptColors.agent

export function AgentPage() {
  const { name = '' } = useParams()
  usePageTitle('', name)
  const agent = useAgent(name)

  if (agent.isPending) {
    return (
      <div className="flex flex-col gap-4 p-6" aria-label="Loading agent">
        <Skeleton className="h-20" />
        <Skeleton className="h-48" />
      </div>
    )
  }
  if (!agent.data) {
    return (
      <div className="p-6">
        <ErrorText error={agent.error} />
      </div>
    )
  }

  const a = agent.data
  return (
    <div data-agent-page={a.name} className="flex flex-col gap-4 p-6">
      <header
        data-testid="agent-header"
        className={cn(
          'flex flex-wrap items-center gap-x-4 gap-y-2.5 rounded-xl border border-border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
          color.edge,
        )}
      >
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span data-testid="agent-state">
            <StatusPill status={a.state} />
          </span>
          <Avatar name={a.name} size="md" presence={a.state === 'awake'} />
          <h2
            data-testid="agent-name"
            className={cn('truncate font-mono text-base', a.kept ? 'font-semibold' : 'text-muted-foreground')}
          >
            {a.name}
          </h2>
          <RoleChip role={a.role} />
          <KeptBadge kept={a.kept} />
          <IconChip
            icon={Clock}
            title={a.last_active_at ? `Last active ${new Date(a.last_active_at).toLocaleString()}` : undefined}
            className="font-sans"
          >
            {lastActive(a.last_active_at)}
          </IconChip>
        </div>
        <AgentActions agent={a} />
      </header>
      <ErrorText error={agent.error} />
      <div className="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-3">
        <HomeCard agent={a} />
        <TasksCard agent={a} />
        <ConversationsCard agent={a} />
        <IdentityCard agent={a} />
        <SkillsCard agent={a} />
        <RulesCard agent={a} />
      </div>
    </div>
  )
}

function AgentActions({ agent }: { agent: AgentView }) {
  const wake = useWakeAgent(agent.name)
  const sleep = useSleepAgent(agent.name)
  const busy = wake.isPending || sleep.isPending
  const waking = agent.state === 'waking'

  return (
    <div data-slot="acts" className="ml-auto flex flex-col items-end gap-2">
      <div className="flex items-center gap-2">
        {agent.state === 'asleep' && (
          <Button data-act="wake" variant="outline" size="sm" disabled={busy} onClick={() => wake.mutate()}>
            {wake.isPending ? <Loader2 className="animate-spin" /> : <Sun />}
            Wake
          </Button>
        )}
        {(agent.state === 'awake' || waking) && (
          <Button data-act="sleep" variant="outline" size="sm" disabled={busy || waking} onClick={() => sleep.mutate()}>
            {sleep.isPending || waking ? <Loader2 className="animate-spin" /> : <Moon />}
            {waking ? 'Waking…' : 'Sleep'}
          </Button>
        )}
        <KeepAgentDialog
          agent={agent}
          trigger={
            <Button data-act="keep" variant="outline" size="sm" disabled={busy}>
              {agent.kept ? <Pencil /> : <UserCheck />}
              {agent.kept ? 'Rename…' : 'Keep…'}
            </Button>
          }
        />
      </div>
      <ErrorText error={wake.error ?? sleep.error} className="text-xs" />
    </div>
  )
}

function AgentCard({
  label,
  concept,
  children,
  name,
}: {
  label: string
  concept: Concept
  children: ReactNode
  name: string
}) {
  const Icon = conceptIcons[concept]
  return (
    <section
      aria-label={label}
      data-card={name}
      className={cn(
        'flex flex-col gap-2.5 rounded-xl border border-border bg-white p-3.5 shadow-sm dark:bg-zinc-900/60',
        conceptColors[concept].edge,
      )}
    >
      <h3 className="flex items-center gap-1.5 text-sm font-medium">
        <Icon aria-hidden className={cn('size-4', conceptColors[concept].text)} />
        {label}
      </h3>
      {children}
    </section>
  )
}

const rowLink =
  'flex items-center gap-2 rounded-md px-2 py-1 text-sm transition-colors hover:bg-zinc-100 dark:hover:bg-zinc-800/60'

function HomeCard({ agent }: { agent: AgentView }) {
  return (
    <AgentCard label="Home" concept="sandbox" name="home">
      {agent.home ? (
        <Link to={`/compute/${encodeURIComponent(agent.home)}`} className={cn(rowLink, 'self-start')}>
          <IconChip icon={conceptIcons.sandbox} concept="sandbox" title="Home sandbox">
            {agent.home}
          </IconChip>
        </Link>
      ) : (
        <EmptySlot reason="Not woken yet" className="self-start" />
      )}
    </AgentCard>
  )
}

function TasksCard({ agent }: { agent: AgentView }) {
  return (
    <AgentCard label="Tasks" concept="task" name="tasks">
      {agent.tasks.length === 0 ? (
        <EmptySlot reason="Manages no tasks" className="self-start" />
      ) : (
        <ul className="flex flex-col">
          {agent.tasks.map((task) => (
            <li key={task.name}>
              <Link to={`/tasks/${encodeURIComponent(task.name)}`} data-task={task.name} className={rowLink}>
                <span className="flex w-20 shrink-0">
                  <StatusPill status={task.status} />
                </span>
                <conceptIcons.task aria-hidden className={cn('size-4 shrink-0', conceptColors.task.text)} />
                <span className="truncate font-mono">{task.name}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </AgentCard>
  )
}

function ConversationsCard({ agent }: { agent: AgentView }) {
  const conversations = dmFirst(agent)
  return (
    <AgentCard label="Conversations" concept="channel" name="conversations">
      {conversations.length === 0 ? (
        <EmptySlot reason="Reads no channels" className="self-start" />
      ) : (
        <ul className="flex flex-col">
          {conversations.map((c) => (
            <li key={c.channel}>
              <Link
                to={`/messages/${encodeURIComponent(c.channel.replace(/^#/, ''))}`}
                data-channel={c.channel}
                className={rowLink}
              >
                <conceptIcons.channel aria-hidden className={cn('size-4 shrink-0', conceptColors.channel.text)} />
                <span className="truncate font-mono">{c.channel.replace(/^#/, '')}</span>
                {c.channel === `#${agent.owner}` && (
                  <span title="Its DM with the owner" className="text-xs text-muted-foreground">
                    DM
                  </span>
                )}
                <span
                  title={`Last activity ${new Date(c.last_at).toLocaleString()}`}
                  className="ml-auto shrink-0 text-xs text-muted-foreground tabular-nums"
                >
                  {ago(c.last_at)}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </AgentCard>
  )
}

function IdentityCard({ agent }: { agent: AgentView }) {
  return (
    <AgentCard label="Identity" concept="agent" name="identity">
      <dl className="grid grid-cols-[5rem_1fr] items-center gap-x-3 gap-y-1.5 text-sm">
        <dt className="text-xs text-muted-foreground">Harness</dt>
        <dd>
          <IconChip icon={conceptIcons.agent} concept="agent">
            {agent.harness}
          </IconChip>
        </dd>
        <dt className="text-xs text-muted-foreground">Model</dt>
        <dd data-testid="agent-model" className="flex min-w-0 items-center gap-1.5 font-mono text-xs">
          <span className="truncate">{agent.model || '–'}</span>
          <EffortChip effort={agent.effort} />
        </dd>
        <dt className="text-xs text-muted-foreground">Owner</dt>
        <dd>
          <IconChip icon={conceptIcons.human} concept="human">
            {agent.owner}
          </IconChip>
        </dd>
        <dt className="text-xs text-muted-foreground">Created</dt>
        <dd className="text-xs" title={new Date(agent.created_at).toLocaleString()}>
          {ago(agent.created_at)}
        </dd>
      </dl>
    </AgentCard>
  )
}

// The model's reasoning effort beside it, dim: "· high". Nothing when empty
// (the harness default).
function EffortChip({ effort }: { effort?: string }) {
  if (!effort) {
    return null
  }
  return (
    <span
      data-testid="agent-effort"
      title="Effort"
      className="inline-flex shrink-0 items-center rounded-md border border-zinc-200 px-1.5 py-0.5 font-mono text-xs text-muted-foreground dark:border-zinc-800"
    >
      · {effort}
    </span>
  )
}

function SkillsCard({ agent }: { agent: AgentView }) {
  const skills = agent.skills ?? []
  const [open, setOpen] = useState<string | null>(null)
  return (
    <AgentCard label="Skills" concept="skill" name="skills">
      {skills.length === 0 ? (
        <EmptySlot reason="no skills" className="self-start" />
      ) : (
        <ul aria-label="Skills" className="flex flex-wrap gap-2">
          {skills.map((skill) => (
            <li key={skill}>
              <button
                type="button"
                data-skill={skill}
                title={`Read the skill ${skill}`}
                onClick={() => setOpen(skill)}
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 font-mono text-xs transition-colors hover:bg-blue-500/20 focus-visible:ring-2 focus-visible:ring-violet-500 focus-visible:outline-none',
                  conceptColors.skill.bg,
                  conceptColors.skill.border,
                  conceptColors.skill.text,
                )}
              >
                <conceptIcons.skill aria-hidden className="size-3.5" />
                {skill}
              </button>
            </li>
          ))}
        </ul>
      )}
      <SkillDialog name={open} onClose={() => setOpen(null)} />
    </AgentCard>
  )
}

// The skill's markdown, as plain text.
function SkillDialog({ name, onClose }: { name: string | null; onClose: () => void }) {
  const text = useSkillText(name ?? '', name !== null)
  return (
    <Dialog open={name !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5 font-mono">
            <conceptIcons.skill aria-hidden className={cn('size-4', conceptColors.skill.text)} />
            {name}
          </DialogTitle>
          <DialogDescription>Skill</DialogDescription>
        </DialogHeader>
        {text.isPending ? (
          <Skeleton className="h-40" />
        ) : text.error ? (
          <ErrorText error={text.error} />
        ) : (
          <pre
            data-testid="skill-text"
            className="max-h-[60vh] overflow-auto rounded-md border border-border bg-zinc-50 p-3 font-mono text-xs whitespace-pre-wrap dark:bg-zinc-950/60"
          >
            {text.data}
          </pre>
        )}
      </DialogContent>
    </Dialog>
  )
}

// One row per rule: the sentence, then its id dim and mono. Not clickable.
function RulesCard({ agent }: { agent: AgentView }) {
  const rules = agent.rules ?? []
  return (
    <AgentCard label="Rules" concept="rule" name="rules">
      {rules.length === 0 ? (
        <EmptySlot reason="no rules" className="self-start" />
      ) : (
        <ul aria-label="Rules" className="flex flex-col divide-y divide-border">
          {rules.map((r) => (
            <li
              key={r.id}
              data-rule={r.id}
              className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5 px-2 py-1.5 text-sm"
            >
              <span data-slot="rule">{r.rule}</span>
              <span data-slot="id" className="ml-auto font-mono text-xs break-all text-muted-foreground">
                {r.id}
              </span>
            </li>
          ))}
        </ul>
      )}
    </AgentCard>
  )
}

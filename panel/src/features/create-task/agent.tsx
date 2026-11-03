// The Agent card: who works the task (a harness or a person), with which
// model, who manages it and who owns it (ADR 0009). The defaults are shown
// as values, so the user sees who will work and who will manage before
// pressing Create.
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import { Card, chipBase, IconInput, Tip } from './bits'

// ADR 0009 item 1, as the server applies them when a field is empty: the
// harness claude_code, its model claude-opus-5-5, the owner franco. The
// manager is the task's creator: in the panel, the signed-in person, franco.
export const HARNESSES = ['claude_code', 'codex_cli', 'pi'] as const
export const DEFAULT_HARNESS = 'claude_code'
export const DEFAULT_MANAGER = 'franco'
export const DEFAULT_OWNER = 'franco'
const DEFAULT_MODELS: Record<string, string> = { claude_code: 'claude-opus-5-5' }

// The assignee toggle's value for "a person"; the name is typed apart.
export const PERSON = 'person'

export function defaultModel(worker: string) {
  return DEFAULT_MODELS[worker] ?? ''
}

export type AgentForm = {
  // A harness id or PERSON.
  worker: string
  person: string
  model: string
  // Who created the task and runs it day to day: the signed-in person.
  manager: string
  // The human the task answers to.
  owner: string
}

export function blankAgent(): AgentForm {
  return {
    worker: DEFAULT_HARNESS,
    person: '',
    model: defaultModel(DEFAULT_HARNESS),
    manager: DEFAULT_MANAGER,
    owner: DEFAULT_OWNER,
  }
}

// The assignee sent to the server: the harness id or the person's name.
export function assigneeOf(a: AgentForm) {
  return a.worker === PERSON ? a.person.trim() : a.worker
}

const HARNESS_TIPS: Record<string, string> = {
  claude_code: 'Claude Code, run by agc in the sandbox',
  codex_cli: 'Codex CLI, run by agc in the sandbox',
  pi: 'pi, run by agc in the sandbox',
}

export function AgentCard({
  agent,
  name,
  onWorker,
  update,
}: {
  agent: AgentForm
  // The task's name, for its channel in the hint.
  name: string
  onWorker: (worker: string) => void
  update: (patch: Partial<AgentForm>) => void
}) {
  const color = conceptColors.agent
  const AgentIcon = conceptIcons.agent
  const HumanIcon = conceptIcons.human
  const person = agent.worker === PERSON

  return (
    <Card name="agent" concept="agent" label="Agent" tip="Who works the task, and who manages it">
      <div className="flex flex-wrap items-center gap-2">
        <ToggleGroup
          type="single"
          spacing={1.5}
          aria-label="Assignee"
          value={agent.worker}
          onValueChange={(v) => v && onWorker(v)}
          className="flex-wrap"
        >
          {HARNESSES.map((h) => (
            <Tip key={h} wrap tip={HARNESS_TIPS[h]}>
              <ToggleGroupItem
                value={h}
                data-assignee={h}
                size="sm"
                className={cn(chipBase, color.on, 'gap-1 font-mono')}
              >
                <AgentIcon aria-hidden />
                {h}
              </ToggleGroupItem>
            </Tip>
          ))}
          <Tip wrap tip="A person does this task, without an agent">
            <ToggleGroupItem
              value={PERSON}
              data-assignee={PERSON}
              size="sm"
              className={cn(chipBase, color.on, 'gap-1')}
            >
              <HumanIcon aria-hidden />a person
            </ToggleGroupItem>
          </Tip>
        </ToggleGroup>
        {person ? (
          <IconInput
            data-person
            icon={HumanIcon}
            concept="human"
            tip="Person"
            wrapperClassName="w-44"
            autoFocus
            value={agent.person}
            onChange={(e) => update({ person: e.target.value })}
            placeholder="alice"
          />
        ) : null}
      </div>
      {person ? null : (
        <div className="flex flex-wrap items-center gap-2">
          <FieldLabel text="Model" />
          <IconInput
            data-model
            icon={AgentIcon}
            concept="agent"
            tip="Model; empty takes the server's default"
            wrapperClassName="w-56"
            value={agent.model}
            onChange={(e) => update({ model: e.target.value })}
            placeholder="model"
          />
        </div>
      )}
      <div className="flex flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <FieldLabel text="Manager" />
          <IconInput
            data-manager
            icon={HumanIcon}
            concept="human"
            tip="Manager: created the task and runs it day to day"
            wrapperClassName="w-56"
            value={agent.manager}
            onChange={(e) => update({ manager: e.target.value })}
            placeholder="manager"
          />
          <FieldLabel text="Owner" />
          <IconInput
            data-owner
            icon={HumanIcon}
            concept="human"
            tip="Owner: the human the task answers to"
            wrapperClassName="w-56"
            value={agent.owner}
            onChange={(e) => update({ owner: e.target.value })}
            placeholder="owner"
          />
        </div>
        <p data-agent-hint className="text-xs text-muted-foreground">
          The manager answers the agent in #{name || '<name>'} and raises to {agent.owner.trim() || '<owner>'} what only
          the owner can decide.
        </p>
      </div>
    </Card>
  )
}

function FieldLabel({ text }: { text: string }) {
  return (
    <span data-field-label className="w-14 shrink-0 text-xs text-muted-foreground">
      {text}
    </span>
  )
}

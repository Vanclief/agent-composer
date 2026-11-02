// New agent (role, harness, model) and Keep…/Rename… (name, role).
import { useState, type FormEvent, type ReactNode } from 'react'
import { Loader2, Plus } from 'lucide-react'
import { useNavigate } from 'react-router'

import type { AgentView } from '@/api/agents'
import { useCreateAgent, useKeepAgent } from '@/api/hooks'
import { conceptColors } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DEFAULT_HARNESS, defaultModel, HARNESSES } from '@/features/create-task/agent'

import { AGENT_NAME_RE, ROLE_SUGGESTIONS } from './format'

type DialogProps = { open: boolean; onOpenChange: (open: boolean) => void }

// Creates a numbered agent from a role and opens its page.
export function NewAgentDialog({ open, onOpenChange }: DialogProps) {
  const navigate = useNavigate()
  const create = useCreateAgent()
  const [role, setRole] = useState('')
  const [harness, setHarness] = useState<string>(DEFAULT_HARNESS)
  const [model, setModel] = useState(defaultModel(DEFAULT_HARNESS))
  const ready = role.trim() !== ''

  function openChange(next: boolean) {
    onOpenChange(next)
    if (!next) {
      setRole('')
      setHarness(DEFAULT_HARNESS)
      setModel(defaultModel(DEFAULT_HARNESS))
      create.reset()
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!ready) {
      return
    }
    create.mutate(
      { role: role.trim(), harness, model: model.trim() || undefined },
      {
        onSuccess: (agent) => {
          openChange(false)
          void navigate(`/agents/${encodeURIComponent(agent.name)}`)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={openChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>New agent</DialogTitle>
            <DialogDescription>Born from a role with a number; keep it later to give it a name.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="agent-role">Role</Label>
            <Input
              id="agent-role"
              list="agent-role-suggestions"
              autoFocus
              value={role}
              onChange={(e) => setRole(e.target.value)}
              placeholder="Reviewer"
            />
            <datalist id="agent-role-suggestions">
              {ROLE_SUGGESTIONS.map((r) => (
                <option key={r} value={r} />
              ))}
            </datalist>
          </div>
          <div className="flex gap-3">
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="agent-harness">Harness</Label>
              <Select
                value={harness}
                onValueChange={(h) => {
                  setHarness(h)
                  setModel(defaultModel(h))
                }}
              >
                <SelectTrigger id="agent-harness" className="w-full font-mono">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {HARNESSES.map((h) => (
                    <SelectItem key={h} value={h} className="font-mono">
                      {h}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="agent-model">Model</Label>
              <Input
                id="agent-model"
                className="font-mono"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="server default"
                title="Model; empty takes the server's default"
              />
            </div>
          </div>
          <ErrorText error={create.error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => openChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!ready || create.isPending} className={conceptColors.agent.solid}>
              {create.isPending ? <Loader2 className="animate-spin" /> : <Plus />}
              {ready ? 'Create' : 'Create: needs a role'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// Keep… names a numbered agent; Rename… (same dialog) renames a kept one.
// Both may change the role. The page follows the new name.
export function KeepAgentDialog({ agent, trigger }: { agent: AgentView; trigger: ReactNode }) {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const keep = useKeepAgent(agent.name)
  const [name, setName] = useState(agent.kept ? agent.name : '')
  const [role, setRole] = useState(agent.role)
  const verb = agent.kept ? 'Rename' : 'Keep'
  const trimmed = name.trim()
  const why = !trimmed ? 'needs a name' : AGENT_NAME_RE.test(trimmed) ? '' : 'lowercase letters, digits and dashes'

  function openChange(next: boolean) {
    setOpen(next)
    if (next) {
      setName(agent.kept ? agent.name : '')
      setRole(agent.role)
    } else {
      keep.reset()
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (why) {
      return
    }
    keep.mutate(
      { name: trimmed, role: role.trim() || undefined },
      {
        onSuccess: (kept) => {
          setOpen(false)
          void navigate(`/agents/${encodeURIComponent(kept.name)}`, { replace: true })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={openChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>
              {verb} {agent.name}
            </DialogTitle>
            <DialogDescription>
              {agent.kept ? 'A new name for this agent.' : 'Give it a name: it stops being a numbered agent.'}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="keep-name">Name</Label>
            <Input
              id="keep-name"
              autoFocus
              className="font-mono"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="ada"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="keep-role">Role</Label>
            <Input id="keep-role" list="keep-role-suggestions" value={role} onChange={(e) => setRole(e.target.value)} />
            <datalist id="keep-role-suggestions">
              {ROLE_SUGGESTIONS.map((r) => (
                <option key={r} value={r} />
              ))}
            </datalist>
          </div>
          <ErrorText error={keep.error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => openChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={Boolean(why) || keep.isPending} className={conceptColors.agent.solid}>
              {keep.isPending && <Loader2 className="animate-spin" />}
              {why ? `${verb}: ${why}` : verb}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

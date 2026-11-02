// The six cards of the create dialog. Each edits its slice of the SpecForm
// through update(); no prose, the icons' tooltips say what a sentence would.
import { useState } from 'react'
import {
  ArrowRight,
  ChevronDown,
  ChevronUp,
  Database,
  Eye,
  Globe,
  Laptop,
  Lock,
  LockOpen,
  Pencil,
  Plus,
  TriangleAlert,
  X,
} from 'lucide-react'

import type { Secret } from '@/api/client'
import type { SecretHarness } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Toggle } from '@/components/ui/toggle'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import { Card, chipBase, CountBadge, IconInput, Tip } from './bits'
import {
  CPU_CHOICES,
  MEMORY_CHOICES,
  memoryLabel,
  parseChannel,
  repoDir,
  rowId,
  type SecretPick,
  type SpecForm,
  type VolumeKind,
  type VolumeRow,
} from './spec-form'

type CardProps = {
  form: SpecForm
  update: (patch: Partial<SpecForm>) => void
}

const removeClass =
  'shrink-0 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground [&_svg]:size-3.5'

function RemoveButton({ onClick, label = 'Remove' }: { onClick: () => void; label?: string }) {
  return (
    <button type="button" onClick={onClick} aria-label={label} title={label} className={removeClass}>
      <X />
    </button>
  )
}

// ---------- machine ----------

// Preset chips plus, when the value is not a preset, one extra lit chip.
// Clicking the lit chip again unsets it, so the server default applies.
function PresetChips({
  name,
  concept,
  tip,
  choices,
  value,
  label,
  onChange,
}: {
  name: string
  concept: 'cpu' | 'memory'
  tip: string
  choices: number[]
  value: number | null
  label: (n: number) => string
  onChange: (n: number | null) => void
}) {
  const all = value && !choices.includes(value) ? [...choices, value] : choices
  const Icon = conceptIcons[concept]
  const color = conceptColors[concept]
  return (
    <div className="flex items-center gap-2" {...{ [`data-${name}`]: '' }}>
      <Tip tip={tip}>
        <Icon aria-label={tip} className={cn('size-4 shrink-0', color.text)} />
      </Tip>
      <ToggleGroup
        type="single"
        spacing={1.5}
        aria-label={tip}
        value={value ? String(value) : ''}
        onValueChange={(v) => onChange(v ? Number(v) : null)}
        className="flex-wrap"
      >
        {all.map((n) => (
          <ToggleGroupItem key={n} value={String(n)} size="sm" className={cn(chipBase, color.on)}>
            {label(n)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}

export function MachineCard({ form, update }: CardProps) {
  return (
    <Card name="machine" concept="sandbox" label="Size" tip="Image, vCPUs and memory of the sandbox">
      <IconInput
        data-image
        icon={conceptIcons.image}
        concept="image"
        tip="Image: any OCI image"
        value={form.image}
        onChange={(e) => update({ image: e.target.value })}
        placeholder="agc-agent:latest"
      />
      <PresetChips
        name="cpus"
        concept="cpu"
        tip="vCPUs. Click the lit chip again for the server default."
        choices={CPU_CHOICES}
        value={form.cpus}
        label={String}
        onChange={(cpus) => update({ cpus })}
      />
      <PresetChips
        name="memory"
        concept="memory"
        tip="Memory. Click the lit chip again for the server default."
        choices={MEMORY_CHOICES}
        value={form.memory}
        label={memoryLabel}
        onChange={(memory) => update({ memory })}
      />
    </Card>
  )
}

// ---------- code ----------

export function CodeCard({ form, update }: CardProps) {
  const [setupOpen, setSetupOpen] = useState(false)
  const dir = repoDir(form.repo)
  const setup = form.setup.replace(/\n+$/, '')
  const setupLines = setup.trim() ? setup.split('\n').length : 0
  const SetupIcon = conceptIcons.setup

  return (
    <Card name="code" concept="repo" label="Code" tip="Repository cloned into the sandbox">
      <IconInput
        data-repo
        icon={conceptIcons.repo}
        concept="repo"
        tip="Repository: github.com/owner/name or an https URL"
        value={form.repo}
        onChange={(e) => update({ repo: e.target.value })}
        placeholder="https://github.com/org/repo"
      />
      <div className="flex items-center gap-2">
        <IconInput
          data-ref
          icon={conceptIcons.ref}
          concept="ref"
          tip="Branch, tag or commit"
          wrapperClassName="w-36 shrink-0"
          value={form.ref}
          onChange={(e) => update({ ref: e.target.value })}
          placeholder="main"
        />
        {dir ? (
          <span data-repo-dir className="min-w-0">
            <IconChip icon={conceptIcons.repo} concept="repo" title="Where the repo is cloned in the sandbox">
              {dir}
            </IconChip>
          </span>
        ) : null}
      </div>
      <div className="flex flex-col rounded-md border border-border">
        <button
          type="button"
          data-setup-toggle
          aria-expanded={setupOpen}
          onClick={() => setSetupOpen(!setupOpen)}
          className="flex h-7 items-center gap-1.5 rounded-md px-2 text-xs hover:bg-accent"
        >
          <Tip tip="Bash script, runs once after the clone">
            <SetupIcon aria-hidden className={cn('size-3.5', conceptColors.setup.text)} />
          </Tip>
          Setup script
          {setupLines ? (
            <span data-setup-lines>
              <CountBadge count={setupLines} className={cn(conceptColors.setup.bg, conceptColors.setup.text)} />
            </span>
          ) : null}
          {setupOpen ? (
            <ChevronUp className="ml-auto size-3.5 text-muted-foreground" />
          ) : (
            <ChevronDown className="ml-auto size-3.5 text-muted-foreground" />
          )}
        </button>
        {setupOpen ? (
          <Textarea
            data-setup
            autoFocus
            rows={4}
            spellCheck={false}
            value={form.setup}
            onChange={(e) => update({ setup: e.target.value })}
            placeholder="go mod download"
            className="field-sizing-fixed rounded-t-none border-x-0 border-b-0 bg-muted/40 font-mono text-xs shadow-none md:text-xs dark:bg-muted/40"
          />
        ) : null}
      </div>
    </Card>
  )
}

// ---------- secrets ----------

type SecretChip = { name: string; hosts: string[]; known: boolean }

// The providers' API hosts agc binds by default (secrets.KnownHosts): a
// secret sent only there is an account, so it may stand in for a variable.
const PROVIDER_HOSTS = ['api.anthropic.com', 'api.openai.com']

export function SecretsCard({
  form,
  update,
  secrets,
  error,
  harnesses,
  harness,
}: CardProps & {
  secrets: Secret[] | undefined
  error: unknown
  // GET /api/secrets' harnesses, and the task's harness ('' for a person).
  harnesses?: SecretHarness[]
  harness?: string
}) {
  // Every secret secrets.env has, plus any the form selects that it does
  // not (from a saved spec), so nothing selected is hidden.
  const chips: SecretChip[] = (secrets ?? []).map((s) => ({ name: s.name, hosts: s.hosts ?? [], known: true }))
  for (const s of form.secrets) {
    if (!chips.some((c) => c.name === s.name)) {
      chips.push({ name: s.name, hosts: [], known: false })
    }
  }
  const picked = (name: string) => form.secrets.find((s) => s.name === name)

  // A variable some harness reads; a secret named so needs no alias.
  const read = new Set((harnesses ?? []).flatMap((h) => h.reads))
  // The hosts of a provider's API: the defaults, plus the hosts of every
  // secret a harness reads.
  const providerHosts = new Set(PROVIDER_HOSTS)
  for (const s of secrets ?? []) {
    if (read.has(s.name)) {
      for (const h of s.hosts ?? []) {
        providerHosts.add(h)
      }
    }
  }
  const reads = (harnesses ?? []).find((h) => h.id === harness)?.reads ?? []

  // The variables a picked chip may stand in for, empty when it offers none.
  function asChoices(chip: SecretChip, pick: SecretPick) {
    const typed = pick.hosts
      .split(',')
      .map((h) => h.trim())
      .filter(Boolean)
    const hosts = typed.length ? typed : chip.hosts
    if (read.has(chip.name) || !hosts.every((h) => providerHosts.has(h))) {
      return []
    }
    const env = pick.env.trim()
    return env && !reads.includes(env) ? [...reads, env] : reads
  }

  function toggle(chip: SecretChip, on: boolean) {
    if (on) {
      update({ secrets: [...form.secrets, { name: chip.name, env: '', hosts: '', custom: false }] })
      return
    }
    update({ secrets: form.secrets.filter((s) => s.name !== chip.name) })
  }

  function setHosts(name: string, hosts: string) {
    update({ secrets: form.secrets.map((s) => (s.name === name ? { ...s, hosts } : s)) })
  }

  function setEnv(name: string, env: string) {
    update({ secrets: form.secrets.map((s) => (s.name === name ? { ...s, env } : s)) })
  }

  return (
    <Card
      name="secrets"
      concept="secret"
      label="Secrets"
      tip="Secrets from secrets.env. Each is sent only to its hosts. A secret can stand in for the variable a harness reads"
      count={form.secrets.length}
    >
      <div className="flex flex-wrap items-center gap-1.5">
        {chips.map((chip) => {
          const pick = picked(chip.name)
          const choices = pick ? asChoices(chip, pick) : []
          const env = pick?.env.trim() ?? ''
          const needsHosts = Boolean(pick) && (!chip.hosts.length || Boolean(pick?.custom))
          let tip = 'No known hosts: say where it may be sent'
          if (!chip.known) {
            tip = 'Not in secrets.env'
          } else if (chip.hosts.length) {
            tip = 'Sent only to ' + chip.hosts.join(', ')
          }
          return (
            <div key={chip.name} className="flex max-w-full flex-wrap items-center gap-1">
              <Tip wrap tip={tip}>
                <Toggle
                  data-secret={chip.name}
                  pressed={Boolean(pick)}
                  onPressedChange={(on) => toggle(chip, on)}
                  size="sm"
                  className={cn(
                    chipBase,
                    conceptColors.secret.on,
                    'gap-1 font-mono',
                    !chip.known && 'border-dashed border-red-500/70',
                  )}
                >
                  {chip.known ? <conceptIcons.secret aria-hidden /> : <TriangleAlert className="text-red-500" />}
                  {chip.name}
                  {env && env !== chip.name ? (
                    <>
                      {' '}
                      <span data-secret-arrow className="text-muted-foreground/70">
                        →
                      </span>{' '}
                      <span data-secret-env>{env}</span>
                    </>
                  ) : null}
                  {chip.hosts.length ? <Globe aria-hidden className="opacity-60" /> : null}
                </Toggle>
              </Tip>
              {needsHosts && pick ? (
                <IconInput
                  data-secret-hosts={chip.name}
                  icon={Globe}
                  tip="Hosts this secret may be sent to, comma-separated"
                  wrapperClassName="w-52"
                  autoFocus={!pick.custom}
                  value={pick.hosts}
                  onChange={(e) => setHosts(chip.name, e.target.value)}
                  placeholder="api.example.com, example.com"
                />
              ) : null}
              {choices.length && pick ? (
                <ToggleGroup
                  type="single"
                  data-secret-as={chip.name}
                  aria-label={`${chip.name} as`}
                  value={env}
                  onValueChange={(v) => setEnv(chip.name, v)}
                  className="gap-1"
                >
                  {choices.map((v) => (
                    <Tip key={v} wrap tip={`Sent as ${v}`}>
                      <ToggleGroupItem
                        value={v}
                        data-env={v}
                        size="sm"
                        className={cn(chipBase, conceptColors.env.on, 'font-mono')}
                      >
                        {v}
                      </ToggleGroupItem>
                    </Tip>
                  ))}
                </ToggleGroup>
              ) : null}
            </div>
          )
        })}
        {secrets && chips.length === 0 ? (
          <EmptySlot reason="No secrets in secrets.env" className="min-h-7 py-1" />
        ) : null}
      </div>
      <ErrorText error={error} className="text-xs" />
    </Card>
  )
}

// ---------- env ----------

export function EnvCard({ form, update }: CardProps) {
  const [added, setAdded] = useState(0)

  function setRow(id: number, patch: { key?: string; value?: string }) {
    update({ env: form.env.map((e) => (e.id === id ? { ...e, ...patch } : e)) })
  }

  function add() {
    const id = rowId()
    setAdded(id)
    update({ env: [...form.env, { id, key: '', value: '' }] })
  }

  return (
    <Card name="env" concept="env" label="Env" tip="Environment variables set in the sandbox" count={form.env.length}>
      <div className="flex flex-col gap-1.5">
        {form.env.map((row) => (
          <div key={row.id} data-env className="flex items-center gap-1.5">
            <Input
              data-env-key
              aria-label="Name"
              autoFocus={row.id === added}
              value={row.key}
              onChange={(e) => setRow(row.id, { key: e.target.value })}
              placeholder="KEY"
              className="h-7 w-2/5 px-2 font-mono text-xs shadow-none md:text-xs"
            />
            <span className="text-xs text-muted-foreground">=</span>
            <Input
              data-env-value
              aria-label="Value"
              value={row.value}
              onChange={(e) => setRow(row.id, { value: e.target.value })}
              placeholder="value"
              className="h-7 flex-1 px-2 font-mono text-xs shadow-none md:text-xs"
            />
            <RemoveButton onClick={() => update({ env: form.env.filter((e) => e.id !== row.id) })} />
          </div>
        ))}
        <EmptySlot onAdd={add} label="Add variable" className="min-h-7" />
      </div>
    </Card>
  )
}

// ---------- volumes ----------

export function VolumesCard({ form, update }: CardProps) {
  const [added, setAdded] = useState(0)

  function setRow(id: number, patch: Partial<VolumeRow>) {
    update({ volumes: form.volumes.map((v) => (v.id === id ? { ...v, ...patch } : v)) })
  }

  function add() {
    const id = rowId()
    setAdded(id)
    update({ volumes: [...form.volumes, { id, kind: 'named', source: '', dest: '', rw: false }] })
  }

  return (
    <Card
      name="volumes"
      concept="volume"
      label="Volumes"
      tip="Mounted into the sandbox. Named volumes are kept by agc across tasks; host paths come from this machine."
      count={form.volumes.length}
    >
      <div className="flex flex-col gap-1.5">
        {form.volumes.map((v) => (
          <div key={v.id} data-volume className="flex items-center gap-1.5">
            <Tip wrap tip="Named volume or host path">
              <ToggleGroup
                type="single"
                variant="outline"
                size="sm"
                aria-label="Kind"
                value={v.kind}
                onValueChange={(kind) => kind && setRow(v.id, { kind: kind as VolumeKind })}
                className="shrink-0 shadow-none"
              >
                <ToggleGroupItem
                  value="named"
                  data-kind="named"
                  aria-label="Named volume"
                  className={cn(chipBase, conceptColors.volume.on, 'min-w-7 px-1.5')}
                >
                  <Database className="size-3.5" />
                </ToggleGroupItem>
                <ToggleGroupItem
                  value="host"
                  data-kind="host"
                  aria-label="Host path"
                  className={cn(chipBase, conceptColors.volume.on, 'min-w-7 px-1.5')}
                >
                  <Laptop className="size-3.5" />
                </ToggleGroupItem>
              </ToggleGroup>
            </Tip>
            <Input
              data-volume-source
              aria-label="Source"
              autoFocus={v.id === added}
              value={v.source}
              onChange={(e) => setRow(v.id, { source: e.target.value })}
              placeholder={v.kind === 'host' ? '/Users/me/data' : 'go-mod-cache'}
              className="h-7 flex-1 px-2 font-mono text-xs shadow-none md:text-xs"
            />
            <ArrowRight aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
            <Input
              data-volume-dest
              aria-label="Destination"
              value={v.dest}
              onChange={(e) => setRow(v.id, { dest: e.target.value })}
              placeholder="/home/agent/data"
              className="h-7 flex-1 px-2 font-mono text-xs shadow-none md:text-xs"
            />
            <Tip wrap tip={v.rw ? 'Read-write. Click to make read-only.' : 'Read-only. Click to allow writes.'}>
              <Toggle
                data-volume-rw
                aria-label="Read-write"
                pressed={v.rw}
                onPressedChange={(rw) => setRow(v.id, { rw })}
                size="sm"
                className={cn(chipBase, conceptColors.volume.on, 'min-w-0 gap-0.5 px-1.5 font-mono')}
              >
                {v.rw ? <LockOpen className="size-3.5" /> : <Lock className="size-3.5" />}
                {v.rw ? 'rw' : 'ro'}
              </Toggle>
            </Tip>
            <RemoveButton onClick={() => update({ volumes: form.volumes.filter((x) => x.id !== v.id) })} />
          </div>
        ))}
        <EmptySlot onAdd={add} label="Add volume" className="min-h-7" />
      </div>
    </Card>
  )
}

// ---------- channels ----------

export function ChannelsCard({ form, update, slug }: CardProps & { slug: string }) {
  const [draft, setDraft] = useState('')
  const ChannelIcon = conceptIcons.channel
  const color = conceptColors.channel

  function add() {
    if (!draft.trim()) {
      return
    }
    const c = parseChannel(draft)
    const taken = c.name === '#' || c.name === '#' + slug || form.channels.some((x) => x.name === c.name)
    if (!taken) {
      update({ channels: [...form.channels, c] })
    }
    setDraft('')
  }

  function setRead(name: string, read: boolean) {
    update({ channels: form.channels.map((c) => (c.name === name ? { ...c, read } : c)) })
  }

  return (
    <Card
      name="channels"
      concept="channel"
      label="Channels"
      tip="Message channels the task can use"
      count={form.channels.length + 1}
    >
      <div className="flex flex-wrap items-center gap-1.5 text-xs">
        <Tip tip="The task's own channel, always read-write">
          <span
            data-own-channel
            className={cn(
              'flex h-7 items-center gap-1 rounded-md border px-2 font-mono',
              slug ? color.selected : 'border-dashed border-border text-muted-foreground',
            )}
          >
            <ChannelIcon aria-hidden className="size-3.5" />
            {slug || 'name'}
            <Pencil aria-hidden className="ml-0.5 size-3 opacity-60" />
          </span>
        </Tip>
        {form.channels.map((c) => (
          <span
            key={c.name}
            data-channel={c.name}
            className={cn(
              'flex h-7 items-center gap-1 rounded-md border pr-1 pl-2 font-mono',
              // Read-write is lit; read-only is the dimmer tint.
              c.read ? [color.bg, color.border, color.text] : color.selected,
            )}
          >
            <ChannelIcon aria-hidden className="size-3.5" />
            {c.name.replace(/^#/, '')}
            <Tip
              tip={
                c.read
                  ? 'Read-only: watches, cannot post. Click to allow posting.'
                  : 'Can read and post. Click to make read-only.'
              }
            >
              <button
                type="button"
                data-channel-read
                aria-label={c.read ? 'Read-only' : 'Read-write'}
                aria-pressed={c.read}
                onClick={() => setRead(c.name, !c.read)}
                className="ml-0.5 rounded p-0.5 hover:bg-current/20"
              >
                {c.read ? <Eye className="size-3.5" /> : <Pencil className="size-3.5" />}
              </button>
            </Tip>
            <button
              type="button"
              aria-label={'Remove ' + c.name}
              title="Remove"
              onClick={() => update({ channels: form.channels.filter((x) => x.name !== c.name) })}
              className="rounded p-0.5 opacity-70 hover:bg-current/20 hover:opacity-100"
            >
              <X className="size-3.5" />
            </button>
          </span>
        ))}
        <div className="relative">
          <Plus
            aria-hidden
            className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground"
          />
          <input
            data-channel-draft
            aria-label="Add channel"
            title="Type a channel, Enter adds it. End with :read for read-only."
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                add()
              }
            }}
            placeholder="#channel"
            className="h-7 w-36 rounded-md border border-dashed border-border bg-transparent pr-2 pl-7 font-mono outline-none placeholder:text-muted-foreground focus:border-solid focus:border-ring"
          />
        </div>
      </div>
    </Card>
  )
}

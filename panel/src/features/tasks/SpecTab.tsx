// /tasks/:name/spec: the task's spec, read only, as cards in the create
// dialog's vocabulary. The raw YAML sits behind the </> toggle.
import { useState, type ReactNode } from 'react'
import { dump } from 'js-yaml'
import { ArrowRight, ChevronRight, Eye, FolderOpen, Lock, Pencil } from 'lucide-react'
import { useParams } from 'react-router'

import type { TaskSpec } from '@/api/client'
import { useSecrets, useTask } from '@/api/hooks'
import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { IconChip } from '@/components/icon-chip'
import { Skeleton } from '@/components/ui/skeleton'
import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/utils'

import { formatMemory, lineCount, parseChannel, parseSecret, parseVolume, repoPath } from './format'

export function SpecTab() {
  const { name = '' } = useParams()
  // TaskPage shows the task's error; this tab only renders its data.
  const task = useTask(name)
  const [raw, setRaw] = useState(false)
  const RawIcon = conceptIcons.raw

  if (!task.data) {
    return (
      <div className="grid gap-3 p-6 md:grid-cols-2 xl:grid-cols-3">
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
      </div>
    )
  }

  const spec = task.data.spec
  return (
    <div className="flex flex-col gap-3 px-6 pt-4 pb-6">
      <div className="flex justify-end">
        <Toggle
          variant="outline"
          size="sm"
          pressed={raw}
          onPressedChange={setRaw}
          aria-label="Raw YAML"
          title="Raw YAML"
        >
          <RawIcon />
        </Toggle>
      </div>
      {raw ? (
        <pre className="overflow-auto rounded-xl border border-border bg-white p-4 font-mono text-xs shadow-sm dark:bg-zinc-900/60">
          {dump(spec)}
        </pre>
      ) : (
        <SpecCards spec={spec} />
      )}
    </div>
  )
}

function SpecCards({ spec }: { spec: TaskSpec }) {
  const secrets = useSecrets()
  const knownHosts = new Map(secrets.data?.map((s) => [s.name, s.hosts]))
  const env = Object.entries(spec.env ?? {})
  const volumes = (spec.volumes ?? []).map(parseVolume)
  const channels = (spec.channels ?? []).map(parseChannel)
  const hasMachine = Boolean(spec.image || spec.cpus || spec.memory)

  return (
    <div className="grid items-start gap-3 md:grid-cols-2 xl:grid-cols-3">
      <SpecCard title="Machine" concept="task">
        {hasMachine ? (
          <div className="flex flex-wrap gap-1.5">
            {spec.image && (
              <IconChip icon={conceptIcons.image} concept="image" title="Image">
                {spec.image}
              </IconChip>
            )}
            {spec.cpus ? (
              <IconChip icon={conceptIcons.cpu} concept="cpu" title="CPUs">
                {spec.cpus} CPU
              </IconChip>
            ) : null}
            {spec.memory ? (
              <IconChip icon={conceptIcons.memory} concept="memory" title={`Memory, ${spec.memory} MiB`}>
                {formatMemory(spec.memory)}
              </IconChip>
            ) : null}
          </div>
        ) : (
          <EmptySlot reason="Defaults" />
        )}
      </SpecCard>

      <SpecCard title="Code" concept="repo">
        {spec.repo || spec.setup ? (
          <div className="flex flex-col gap-2">
            {spec.repo && (
              <div className="flex flex-wrap gap-1.5">
                <IconChip icon={conceptIcons.repo} concept="repo" title="Repo">
                  {spec.repo}
                </IconChip>
                {spec.ref && (
                  <IconChip icon={conceptIcons.ref} concept="ref" title="Ref">
                    {spec.ref}
                  </IconChip>
                )}
                <IconChip icon={FolderOpen} concept="repo" title="Cloned to">
                  {repoPath(spec.repo)}
                </IconChip>
              </div>
            )}
            {spec.setup && <SetupScript script={spec.setup} />}
          </div>
        ) : (
          <EmptySlot reason="No repo" />
        )}
      </SpecCard>

      <SpecCard title="Secrets" concept="secret" count={spec.secrets?.length}>
        {spec.secrets?.length ? (
          <div className="flex flex-wrap gap-1.5">
            {spec.secrets.map(parseSecret).map((secret) => {
              const hosts = secret.hosts.length ? secret.hosts : (knownHosts.get(secret.name) ?? [])
              return (
                <IconChip
                  key={secret.name}
                  icon={conceptIcons.secret}
                  concept="secret"
                  title={hosts.length ? hosts.join(', ') : 'No hosts'}
                >
                  {secret.name}
                </IconChip>
              )
            })}
          </div>
        ) : (
          <EmptySlot reason="No secrets" />
        )}
      </SpecCard>

      <SpecCard title="Env" concept="env" count={env.length}>
        {env.length ? (
          <ul className="flex flex-col gap-1 font-mono text-xs">
            {env.map(([key, value]) => (
              <li key={key} className="truncate" title={`${key}=${value}`}>
                <span className={conceptColors.env.text}>{key}=</span>
                {value}
              </li>
            ))}
          </ul>
        ) : (
          <EmptySlot reason="No env" />
        )}
      </SpecCard>

      <SpecCard title="Volumes" concept="volume" count={volumes.length}>
        {volumes.length ? (
          <ul className="flex flex-col gap-1.5">
            {volumes.map((volume) => (
              <li
                key={`${volume.source}:${volume.destination}`}
                className="flex min-w-0 items-center gap-1.5 font-mono text-xs"
              >
                <IconChip
                  icon={conceptIcons.volume}
                  concept="volume"
                  title={volume.host ? 'Host path' : 'Named volume'}
                >
                  {volume.source}
                </IconChip>
                <ArrowRight aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate">{volume.destination}</span>
                {volume.readOnly ? (
                  <span role="img" aria-label="Read-only" title="Read-only" className="shrink-0">
                    <Lock aria-hidden className={cn('size-3.5', conceptColors.volume.text)} />
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
        ) : (
          <EmptySlot reason="No volumes" />
        )}
      </SpecCard>

      <SpecCard title="Channels" concept="channel" count={channels.length}>
        {channels.length ? (
          <div className="flex flex-wrap gap-1.5">
            {channels.map((channel) => {
              const Mode = channel.readOnly ? Eye : Pencil
              return (
                <IconChip
                  key={channel.name}
                  icon={conceptIcons.channel}
                  concept="channel"
                  title={channel.readOnly ? 'Read-only' : 'Read-write'}
                  // Read-write is lit, read-only the softer tint, as in the create dialog.
                  className={channel.readOnly ? undefined : conceptColors.channel.selected}
                >
                  <span className="inline-flex items-center gap-1" data-mode={channel.readOnly ? 'read' : 'write'}>
                    {channel.name}
                    <Mode aria-hidden className="size-3" />
                  </span>
                </IconChip>
              )
            })}
          </div>
        ) : (
          <EmptySlot reason="No channels" />
        )}
      </SpecCard>
    </div>
  )
}

type SpecCardProps = {
  title: string
  concept: Concept
  // Items in the card, shown as a badge when there are any.
  count?: number
  children: ReactNode
}

// A card per concept: its icon and title in the concept's color and a top
// edge in that color, so cards are told apart at a glance.
function SpecCard({ title, concept, count, children }: SpecCardProps) {
  const Icon = conceptIcons[concept]
  const color = conceptColors[concept]
  return (
    <section
      aria-label={title}
      className={cn(
        'flex flex-col gap-3 rounded-xl border border-border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
        color.edge,
      )}
    >
      <h3 className={cn('flex items-center gap-2 text-sm font-medium', color.text)}>
        <Icon className="size-4" />
        {title}
        {count ? (
          <span className={cn('ml-auto rounded px-1.5 font-mono text-xs tabular-nums', color.bg, color.text)}>
            {count}
          </span>
        ) : null}
      </h3>
      {children}
    </section>
  )
}

// The setup script, collapsed to its line count until clicked.
function SetupScript({ script }: { script: string }) {
  const [open, setOpen] = useState(false)
  const SetupIcon = conceptIcons.setup
  const lines = lineCount(script)

  return (
    <div className="rounded-md border border-border">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-2.5 py-1.5 text-xs hover:bg-zinc-50 dark:hover:bg-zinc-800/40"
      >
        <ChevronRight aria-hidden className={cn('size-3.5 text-zinc-500 transition-transform', open && 'rotate-90')} />
        <SetupIcon aria-hidden className={cn('size-3.5', conceptColors.setup.text)} />
        Setup
        <span className="ml-auto rounded-full border border-border px-1.5 font-mono text-[11px] text-muted-foreground">
          {lines} {lines === 1 ? 'line' : 'lines'}
        </span>
      </button>
      {open && <pre className="overflow-auto border-t border-border p-2.5 font-mono text-xs">{script}</pre>}
    </div>
  )
}

// The inline "Allocate compute" form: a saved spec as the starting point,
// then CPUs and memory as preset chips. Submitting allocates a sandbox that
// comes back starting (ADR 0008 item 4).
import { useMemo, useState } from 'react'
import { load } from 'js-yaml'
import { Loader2, Plus, X } from 'lucide-react'

import { useAllocate, useSpecs } from '@/api/hooks'
import type { SandboxRequest, TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons, type Concept } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import { formatMemory } from './format'

const CPU_CHOICES = [1, 2, 4, 8]
// MiB, shown as GB.
const MEMORY_CHOICES = [1, 2, 4, 8, 16].map((gb) => gb * 1024)

const chip =
  'h-7 min-w-9 rounded-md border border-border bg-transparent px-2 text-xs font-normal tabular-nums text-muted-foreground shadow-none hover:bg-accent hover:text-accent-foreground'

// A saved spec is a sandbox spec (ADR 0008 item 7); a task.yaml-shaped one
// nests it under sandbox:.
function parseSpec(yaml: string): SandboxRequest {
  const value = load(yaml)
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('saved spec is not a YAML mapping')
  }
  const record = value as Record<string, unknown>
  const nested = record.sandbox
  if (typeof nested === 'object' && nested !== null && !Array.isArray(nested)) {
    return nested as SandboxRequest
  }
  return record as SandboxRequest
}

type AllocateFormProps = {
  task: TaskView
  onClose: () => void
}

export function AllocateForm({ task, onClose }: AllocateFormProps) {
  const specs = useSpecs()
  const allocate = useAllocate(task.name)
  const [preset, setPreset] = useState('')
  const [cpus, setCPUs] = useState<number | null>(null)
  const [memory, setMemory] = useState<number | null>(null)

  const parsed = useMemo(() => {
    const saved = specs.data?.find((s) => s.name === preset)
    if (!saved) {
      return { request: {} as SandboxRequest, error: null }
    }
    try {
      return { request: parseSpec(saved.yaml), error: null }
    } catch (error) {
      return { request: {} as SandboxRequest, error }
    }
  }, [specs.data, preset])

  function pickPreset(name: string) {
    setPreset(name)
    const saved = specs.data?.find((s) => s.name === name)
    if (!saved) {
      return
    }
    try {
      const request = parseSpec(saved.yaml)
      setCPUs(request.cpus ?? null)
      setMemory(request.memory ?? null)
    } catch {
      // parsed.error shows it.
    }
  }

  function submit() {
    const request: SandboxRequest = { ...parsed.request }
    if (cpus !== null) {
      request.cpus = cpus
    }
    if (memory !== null) {
      request.memory = memory
    }
    allocate.mutate(request, { onSuccess: onClose })
  }

  const SavedSpecIcon = conceptIcons.savedSpec

  return (
    <section
      aria-label="Allocate compute"
      className={cn(
        'flex flex-col gap-3 rounded-xl border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
        conceptColors.sandbox.border,
        conceptColors.sandbox.edge,
      )}
    >
      <div className="flex flex-wrap items-center gap-2" data-presets>
        <SavedSpecIcon aria-label="Saved spec" className={cn('size-4 shrink-0', conceptColors.savedSpec.text)} />
        {specs.data && specs.data.length === 0 && <EmptySlot reason="No saved specs" />}
        {specs.data && specs.data.length > 0 && (
          <ToggleGroup
            type="single"
            spacing={1.5}
            aria-label="Saved spec"
            value={preset}
            onValueChange={pickPreset}
            className="flex-wrap"
          >
            {specs.data.map((s) => (
              <ToggleGroupItem
                key={s.name}
                value={s.name}
                size="sm"
                title={s.yaml}
                className={cn(chip, 'font-mono', conceptColors.savedSpec.on)}
              >
                {s.name}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        )}
        <ErrorText error={specs.error} />
      </div>
      <ErrorText error={parsed.error} />
      <PresetChips concept="cpu" tip="CPUs" choices={CPU_CHOICES} value={cpus} label={String} onChange={setCPUs} />
      <PresetChips
        concept="memory"
        tip="Memory"
        choices={MEMORY_CHOICES}
        value={memory}
        label={formatMemory}
        onChange={setMemory}
      />
      <ErrorText error={allocate.error} />
      <div className="flex items-center justify-end gap-2">
        <Button variant="outline" size="sm" onClick={onClose}>
          <X /> Cancel
        </Button>
        <Button size="sm" disabled={allocate.isPending} onClick={submit} className={conceptColors.sandbox.solid}>
          {allocate.isPending ? <Loader2 className="animate-spin" /> : <Plus />}
          Allocate
        </Button>
      </div>
    </section>
  )
}

// Preset chips plus, when the value is not a preset, one extra lit chip.
// Clicking the lit chip again unsets it, so the server default applies.
function PresetChips({
  concept,
  tip,
  choices,
  value,
  label,
  onChange,
}: {
  concept: Concept
  tip: string
  choices: number[]
  value: number | null
  label: (n: number) => string
  onChange: (n: number | null) => void
}) {
  const all = value !== null && !choices.includes(value) ? [...choices, value] : choices
  const Icon = conceptIcons[concept]
  const color = conceptColors[concept]
  return (
    <div className="flex items-center gap-2">
      <Icon aria-label={tip} className={cn('size-4 shrink-0', color.text)} />
      <ToggleGroup
        type="single"
        spacing={1.5}
        aria-label={tip}
        value={value !== null ? String(value) : ''}
        onValueChange={(v) => onChange(v ? Number(v) : null)}
        className="flex-wrap"
      >
        {all.map((n) => (
          <ToggleGroupItem key={n} value={String(n)} size="sm" className={cn(chip, color.on)}>
            {label(n)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}

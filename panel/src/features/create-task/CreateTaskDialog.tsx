// "New task": title with its live #slug, saved-spec tiles to start from,
// and the spec as six cards (or raw YAML behind the </> toggle). Ported
// from the previous Alpine modal (control_panel/ in git history before 2026-11-02).
import { useMemo, useRef, useState, type FormEvent } from 'react'
import { File, Loader2, Play, Trash2, X } from 'lucide-react'
import { useNavigate } from 'react-router'

import { useCreateTask, useDeleteSpec, useSaveSpec, useSecrets, useSpecs } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { Dialog, DialogClose, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/utils'

import { chipClass, IconInput, Tip } from './bits'
import { ChannelsCard, CodeCard, EnvCard, MachineCard, SecretsCard, VolumesCard } from './cards'
import { blankForm, formToSpec, parseSpecYAML, slugify, specYAML, type SpecForm } from './spec-form'

type CreateTaskDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const tileClass = 'flex h-7 items-center gap-1.5 rounded-md border px-2.5 text-xs transition-colors'
const tileOn = 'border-primary bg-primary text-primary-foreground'
const tileOff = 'border-border text-muted-foreground hover:bg-accent hover:text-accent-foreground'

export function CreateTaskDialog({ open, onOpenChange }: CreateTaskDialogProps) {
  const secrets = useSecrets()
  const specs = useSpecs()
  const create = useCreateTask()
  const saveSpec = useSaveSpec()
  const deleteSpec = useDeleteSpec()
  const navigate = useNavigate()
  const titleRef = useRef<HTMLInputElement>(null)

  // The form survives Cancel, like a draft; a successful create resets it.
  const [form, setForm] = useState<SpecForm>(blankForm)
  const [title, setTitle] = useState('')
  // '' is the Blank tile.
  const [selected, setSelected] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [pickError, setPickError] = useState<Error | null>(null)
  // null while the save-spec input is closed.
  const [specName, setSpecName] = useState<string | null>(null)
  const [raw, setRaw] = useState(false)
  const [yaml, setYAML] = useState('')

  const slug = slugify(title)
  const parsed = useMemo(() => (raw ? parseSpecYAML(yaml) : null), [raw, yaml])
  // The form the spec comes from: in raw view, the parsed YAML (null when
  // it does not parse).
  const current = raw ? (parsed?.form ?? null) : form
  const ready = Boolean(slug) && current !== null

  function update(patch: Partial<SpecForm>) {
    setForm((f) => ({ ...f, ...patch }))
  }

  function reset() {
    setTitle('')
    setRaw(false)
    setYAML('')
    setSpecName(null)
    setConfirmDelete(false)
    setPickError(null)
    create.reset()
    saveSpec.reset()
    deleteSpec.reset()
  }

  function handleOpenChange(next: boolean) {
    if (!next) {
      reset()
    }
    onOpenChange(next)
  }

  function toggleRaw() {
    if (raw) {
      // Stay in the YAML while it does not parse; its error is on screen.
      if (parsed?.form) {
        setForm(parsed.form)
        setRaw(false)
      }
      return
    }
    setYAML(specYAML(formToSpec(form)))
    setRaw(true)
  }

  // A saved spec is only a starting point: editing the form afterwards
  // keeps the selection.
  function pickSpec(name: string) {
    setConfirmDelete(false)
    setPickError(null)
    deleteSpec.reset()
    let next = blankForm()
    if (name) {
      const saved = specs.data?.find((s) => s.name === name)
      if (!saved) {
        return
      }
      const result = parseSpecYAML(saved.yaml)
      if (result.error) {
        setPickError(new Error(`${name}: ${result.error.message}`))
        return
      }
      next = result.form
    }
    setSelected(name)
    setForm(next)
    if (raw) {
      setYAML(specYAML(formToSpec(next)))
    }
  }

  function removeSpec() {
    deleteSpec.mutate(selected, {
      onSuccess: () => {
        setSelected('')
        setConfirmDelete(false)
      },
    })
  }

  function cancelSpecName() {
    saveSpec.reset()
    setSpecName(null)
  }

  function submitSpec() {
    const name = specName?.trim() ?? ''
    if (!name || !slugify(name) || !current || saveSpec.isPending) {
      return
    }
    saveSpec.mutate(
      { name, yaml: specYAML(formToSpec(current)) },
      {
        onSuccess: (saved) => {
          setSpecName(null)
          setSelected(saved.name)
          setConfirmDelete(false)
        },
      },
    )
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!ready || !current || create.isPending) {
      return
    }
    create.mutate(
      { title: title.trim(), spec: formToSpec(current) },
      {
        onSuccess: (task) => {
          setForm(blankForm())
          setSelected('')
          handleOpenChange(false)
          navigate(`/tasks/${encodeURIComponent(task.name)}`)
        },
      },
    )
  }

  let createLabel = 'Create task'
  if (create.isPending) {
    createLabel = 'Creating…'
  } else if (!title.trim()) {
    createLabel = 'Create: needs a title'
  } else if (!slug) {
    createLabel = 'Create: needs a letter or digit'
  } else if (!current) {
    createLabel = 'Create: fix YAML'
  }

  const TaskIcon = conceptIcons.task
  const RawIcon = conceptIcons.raw
  const SpecIcon = conceptIcons.savedSpec
  const ChannelIcon = conceptIcons.channel

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        showCloseButton={false}
        aria-describedby={undefined}
        onEscapeKeyDown={(e) => {
          // Escape in the save-spec prompt closes the prompt, not the dialog.
          if (specName !== null) {
            e.preventDefault()
            cancelSpecName()
          }
        }}
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          titleRef.current?.focus()
        }}
        className="flex max-h-[calc(100dvh-2rem)] flex-col gap-0 overflow-hidden p-0 text-sm sm:max-w-5xl"
      >
        <form data-create-form onSubmit={submit} className="flex min-h-0 flex-1 flex-col">
          <div className="flex items-center gap-2 border-b border-border px-5 py-3">
            <TaskIcon aria-hidden className="size-4 text-muted-foreground" />
            <DialogTitle className="text-sm">New task</DialogTitle>
            <div className="ml-auto flex items-center gap-1">
              <Tip wrap tip={raw ? 'Back to cards' : 'Edit as YAML'}>
                <Toggle
                  data-raw-toggle
                  aria-label="YAML"
                  pressed={raw}
                  onPressedChange={toggleRaw}
                  size="sm"
                  className={cn(chipClass, 'gap-1')}
                >
                  <RawIcon className="size-3.5" />
                  YAML
                </Toggle>
              </Tip>
              <DialogClose asChild>
                <Button type="button" variant="ghost" size="icon-sm" aria-label="Close">
                  <X />
                </Button>
              </DialogClose>
            </div>
          </div>

          <div className="flex min-h-0 flex-col gap-4 overflow-y-auto px-5 py-4">
            {/* Title + live slug */}
            <div className="flex items-center gap-2">
              <div className="relative min-w-0 flex-1">
                <Tip tip="Title">
                  <TaskIcon
                    aria-hidden
                    className="absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
                  />
                </Tip>
                <Input
                  ref={titleRef}
                  data-title
                  aria-label="Title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Review facebook ads"
                  className="pl-9"
                />
              </div>
              <Tip tip="VM and channel name, derived from the title">
                <span className="shrink-0">
                  {slug ? (
                    <IconChip icon={ChannelIcon} className="h-9 bg-muted/50 px-2.5 text-sm">
                      <span data-slug>{slug}</span>
                    </IconChip>
                  ) : (
                    <span
                      data-slug-empty
                      className={cn(
                        'inline-flex h-9 items-center gap-1 rounded-md border border-dashed px-2.5 font-mono',
                        title.trim()
                          ? 'border-red-500/60 text-red-600 dark:text-red-400'
                          : 'border-border text-muted-foreground',
                      )}
                    >
                      <ChannelIcon aria-hidden className="size-3.5" />
                      name
                    </span>
                  )}
                </span>
              </Tip>
            </div>

            {/* Start from: saved spec tiles */}
            <div className="flex flex-col gap-2">
              <div data-start-from className="flex flex-wrap items-center gap-1.5">
                <Tip tip="Start from a saved spec. Editing afterwards does not change the saved one.">
                  <span className="mr-1 flex items-center gap-1 text-xs text-muted-foreground">
                    <SpecIcon aria-hidden className="size-3.5" />
                    Start from
                  </span>
                </Tip>
                <button
                  type="button"
                  data-tile=""
                  aria-pressed={!selected}
                  onClick={() => pickSpec('')}
                  className={cn(tileClass, !selected ? tileOn : tileOff)}
                >
                  <File className="size-3.5" />
                  Blank
                </button>
                {specs.data?.map((s) => {
                  const on = s.name === selected
                  return (
                    <div
                      key={s.name}
                      className={cn(
                        'flex items-center rounded-md border transition-colors',
                        on ? tileOn : tileOff,
                        on && confirmDelete && 'ring-2 ring-red-500/60',
                      )}
                    >
                      <button
                        type="button"
                        data-tile={s.name}
                        aria-pressed={on}
                        onClick={() => pickSpec(s.name)}
                        className={cn('flex h-7 items-center gap-1.5 pl-2.5 font-mono text-xs', on ? 'pr-1' : 'pr-2.5')}
                      >
                        <SpecIcon className="size-3.5" />
                        {s.name}
                      </button>
                      {on && !confirmDelete ? (
                        <button
                          type="button"
                          data-tile-delete
                          aria-label={'Delete saved spec ' + s.name}
                          title="Delete this saved spec"
                          onClick={() => setConfirmDelete(true)}
                          className="mr-1 rounded p-0.5 opacity-60 hover:opacity-100"
                        >
                          <X className="size-3.5" />
                        </button>
                      ) : null}
                      {on && confirmDelete ? (
                        <button
                          type="button"
                          data-tile-confirm
                          title="Click again to delete; pick any tile to keep it"
                          disabled={deleteSpec.isPending}
                          onClick={removeSpec}
                          className="mr-1 flex items-center gap-1 rounded bg-red-600 px-1.5 py-0.5 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-60"
                        >
                          <Trash2 className="size-3" />
                          {deleteSpec.isPending ? 'Deleting…' : 'Delete'}
                        </button>
                      ) : null}
                    </div>
                  )
                })}

                <span className="mx-1 h-5 w-px bg-border" />
                {specName === null ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="xs"
                    data-save-spec
                    className="h-7 px-2.5"
                    onClick={() => {
                      saveSpec.reset()
                      setSpecName('')
                    }}
                    title="Save this form as a reusable spec"
                  >
                    <SpecIcon />
                    Save spec
                  </Button>
                ) : (
                  <SaveSpecInput
                    name={specName}
                    onName={setSpecName}
                    pending={saveSpec.isPending}
                    canSave={current !== null}
                    onSave={submitSpec}
                    onCancel={cancelSpecName}
                  />
                )}
              </div>
              <ErrorText error={pickError ?? saveSpec.error ?? deleteSpec.error ?? specs.error} />
            </div>

            {raw ? (
              <div className="flex flex-col gap-2">
                <Textarea
                  data-raw
                  aria-label="Spec YAML"
                  rows={22}
                  spellCheck={false}
                  value={yaml}
                  onChange={(e) => setYAML(e.target.value)}
                  aria-invalid={Boolean(parsed?.error)}
                  className="field-sizing-fixed bg-muted/40 font-mono text-xs leading-relaxed md:text-xs dark:bg-muted/40"
                />
                <ErrorText error={parsed?.error} className="font-mono text-xs" />
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-3">
                <MachineCard form={form} update={update} />
                <CodeCard form={form} update={update} />
                <SecretsCard form={form} update={update} secrets={secrets.data} error={secrets.error} />
                <EnvCard form={form} update={update} />
                <VolumesCard form={form} update={update} />
                <ChannelsCard form={form} update={update} slug={slug} />
              </div>
            )}
          </div>

          {create.error ? (
            <div className="border-t border-border px-5 pt-3">
              <ErrorText error={create.error} />
            </div>
          ) : null}
          <div className="flex justify-end gap-2 border-t border-border px-5 py-3">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" data-create disabled={!ready || create.isPending}>
              {create.isPending ? <Loader2 className="animate-spin" /> : <Play />}
              {createLabel}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// The inline name prompt of "Save spec", with the slug it is saved as.
function SaveSpecInput({
  name,
  onName,
  pending,
  canSave,
  onSave,
  onCancel,
}: {
  name: string
  onName: (name: string) => void
  pending: boolean
  canSave: boolean
  onSave: () => void
  onCancel: () => void
}) {
  const slug = slugify(name)
  let label = 'Save'
  if (pending) {
    label = 'Saving…'
  } else if (!name.trim()) {
    label = 'Save: needs a name'
  } else if (!slug) {
    label = 'Save: needs a letter or digit'
  } else if (!canSave) {
    label = 'Save: fix YAML'
  }

  return (
    <div className="flex items-center gap-1.5">
      <IconInput
        data-spec-name
        icon={conceptIcons.savedSpec}
        tip="Saved spec name"
        wrapperClassName="w-44"
        autoFocus
        value={name}
        onChange={(e) => onName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            onSave()
          }
        }}
        placeholder="backend-review"
      />
      {slug && slug !== name.trim() ? (
        <IconChip icon={conceptIcons.savedSpec} title="Saved as" className="h-7 bg-muted/50">
          {slug}
        </IconChip>
      ) : null}
      <Button type="button" size="xs" className="h-7 px-2.5" disabled={pending || !slug || !canSave} onClick={onSave}>
        {label}
      </Button>
      <Button type="button" variant="ghost" size="icon-xs" aria-label="Cancel" onClick={onCancel}>
        <X />
      </Button>
    </div>
  )
}

// "New task" (ADR 0008, 0009): the order (title, name, brief, for), the
// agent and its manager with the server's defaults filled in, and the
// compute to allocate now: saved-spec tiles to start from, the command and
// the spec as six cards (or raw YAML behind the </> toggle). Creating
// returns at once; a requested sandbox comes back starting.
import { useMemo, useRef, useState, type FormEvent } from 'react'
import { File, Loader2, Play, Trash2, X } from 'lucide-react'
import { useNavigate } from 'react-router'

import { useCreateTask, useDeleteSpec, useSaveSpec, useSecrets, useSpecs } from '@/api/hooks'
import type { CreateTaskRequest } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { Dialog, DialogClose, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/utils'

import { AgentCard, assigneeOf, blankAgent, defaultModel, PERSON, type AgentForm } from './agent'
import { Card, chipBase, IconInput, Tip } from './bits'
import { ChannelsCard, CodeCard, EnvCard, MachineCard, SecretsCard, VolumesCard } from './cards'
import { blankOrder, OrderCard, type OrderForm } from './order'
import { blankForm, formToSpec, memoryLabel, parseSpecYAML, slugify, specYAML, type SpecForm } from './spec-form'

type CreateTaskDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const tileClass = 'flex h-7 items-center gap-1.5 rounded-md border px-2.5 text-xs transition-colors'
// A lit Start-from tile takes the saved-spec color.
const tileOn = conceptColors.savedSpec.selected
const tileOff = 'border-border text-muted-foreground hover:bg-accent hover:text-accent-foreground'

export function CreateTaskDialog({ open, onOpenChange }: CreateTaskDialogProps) {
  const secrets = useSecrets()
  const specs = useSpecs()
  const create = useCreateTask()
  const saveSpec = useSaveSpec()
  const deleteSpec = useDeleteSpec()
  const navigate = useNavigate()
  const titleRef = useRef<HTMLInputElement>(null)

  // The agent and the spec survive Cancel, like a draft; closing clears the
  // order, and a successful create resets everything.
  const [order, setOrder] = useState<OrderForm>(blankOrder)
  const [agent, setAgent] = useState<AgentForm>(blankAgent)
  // Allocate compute now: on for a harness, off for a person.
  const [compute, setCompute] = useState(true)
  const [command, setCommand] = useState('')
  const [form, setForm] = useState<SpecForm>(blankForm)
  // '' is the Blank tile.
  const [selected, setSelected] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [pickError, setPickError] = useState<Error | null>(null)
  // null while the save-spec input is closed.
  const [specName, setSpecName] = useState<string | null>(null)
  const [raw, setRaw] = useState(false)
  const [yaml, setYAML] = useState('')
  const [pickingFor, setPickingFor] = useState(false)

  const title = order.title.trim()
  const slug = slugify(order.title)
  const name = (order.name ?? slug).trim()
  const person = agent.worker === PERSON
  const assignee = assigneeOf(agent)
  const parsed = useMemo(() => (raw ? parseSpecYAML(yaml) : null), [raw, yaml])
  // The form the spec comes from: in raw view, the parsed YAML (null when
  // it does not parse).
  const current = raw ? (parsed?.form ?? null) : form

  let blocker = ''
  if (!title) {
    blocker = 'needs a title'
  } else if (!name && order.name === null) {
    blocker = 'needs a letter or digit'
  } else if (!name) {
    blocker = 'needs a name'
  } else if (!assignee) {
    blocker = "needs the person's name"
  } else if (compute && !current) {
    blocker = 'fix YAML'
  } else if (!compute && !order.brief.trim()) {
    blocker = 'needs a brief'
  }
  const ready = !blocker

  function update(patch: Partial<SpecForm>) {
    setForm((f) => ({ ...f, ...patch }))
  }

  function updateOrder(patch: Partial<OrderForm>) {
    setOrder((o) => ({ ...o, ...patch }))
  }

  function updateAgent(patch: Partial<AgentForm>) {
    setAgent((a) => ({ ...a, ...patch }))
  }

  // Picking who works resets the model to that harness's default and the
  // compute switch to its default: on for a harness, off for a person.
  function pickWorker(worker: string) {
    updateAgent({ worker, model: defaultModel(worker) })
    setCompute(worker !== PERSON)
  }

  function reset() {
    setOrder(blankOrder())
    setPickingFor(false)
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
    if (!ready || create.isPending) {
      return
    }
    // The defaults on screen are sent as they are, so the task gets what
    // the user saw.
    const request: CreateTaskRequest = {
      title,
      name: order.name === null ? undefined : name,
      brief: order.brief,
      assignee,
      model: person ? '' : agent.model.trim(),
      manager: agent.manager.trim(),
      for: order.for,
    }
    if (compute && current) {
      request.sandbox = formToSpec(current)
      if (command.trim()) {
        request.sandbox.command = command.trim()
      }
    }
    create.mutate(request, {
      onSuccess: (task) => {
        setForm(blankForm())
        setSelected('')
        setAgent(blankAgent())
        setCompute(true)
        setCommand('')
        handleOpenChange(false)
        navigate(`/tasks/${encodeURIComponent(task.name)}`)
      },
    })
  }

  let createLabel = 'Create task'
  if (create.isPending) {
    createLabel = 'Creating…'
  } else if (blocker) {
    createLabel = 'Create: ' + blocker
  }

  // One line saying what Create will make.
  const summary: string[] = []
  if (person) {
    summary.push(assignee || 'a person')
  } else {
    summary.push(agent.model.trim() ? `${agent.worker} (${agent.model.trim()})` : agent.worker)
  }
  if (agent.manager.trim()) {
    summary.push('manager ' + agent.manager.trim())
  }
  if (!compute) {
    summary.push('no sandbox')
  } else if (current) {
    const size = [current.cpus ? `${current.cpus} cpus` : '', current.memory ? memoryLabel(current.memory) : '']
      .filter(Boolean)
      .join(' ')
    if (size) {
      summary.push(size)
    }
    if (current.repo.trim()) {
      summary.push('repo ' + current.repo.trim())
    }
  }

  const TaskIcon = conceptIcons.task
  const RawIcon = conceptIcons.raw
  const SpecIcon = conceptIcons.savedSpec
  const TerminalIcon = conceptIcons.terminal

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        showCloseButton={false}
        aria-describedby={undefined}
        onEscapeKeyDown={(e) => {
          // Escape in the save-spec prompt or the for picker closes it, not
          // the dialog.
          if (specName !== null) {
            e.preventDefault()
            cancelSpecName()
          } else if (pickingFor) {
            e.preventDefault()
            setPickingFor(false)
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
            <TaskIcon aria-hidden className={cn('size-4', conceptColors.task.text)} />
            <DialogTitle className="text-sm">New task</DialogTitle>
            <DialogClose asChild>
              <Button type="button" variant="ghost" size="icon-sm" aria-label="Close" className="ml-auto">
                <X />
              </Button>
            </DialogClose>
          </div>

          <div className="flex min-h-0 flex-col gap-3 overflow-y-auto px-5 py-4">
            <OrderCard
              order={order}
              update={updateOrder}
              slug={slug}
              titleRef={titleRef}
              pickingFor={pickingFor}
              onPickingFor={setPickingFor}
            />
            <AgentCard agent={agent} onWorker={pickWorker} update={updateAgent} />
            <Card
              name="compute"
              concept="sandbox"
              label="Compute"
              tip="The sandbox this task is allocated: one microsandbox VM"
              aside={
                <>
                  {compute ? (
                    <Tip wrap tip={raw ? 'Back to cards' : 'Edit as YAML'}>
                      <Toggle
                        data-raw-toggle
                        aria-label="YAML"
                        pressed={raw}
                        onPressedChange={toggleRaw}
                        size="sm"
                        className={cn(chipBase, conceptColors.raw.on, 'gap-1')}
                      >
                        <RawIcon className="size-3.5" />
                        YAML
                      </Toggle>
                    </Tip>
                  ) : null}
                  <label className="flex items-center gap-2 text-xs">
                    <Switch
                      data-compute
                      checked={compute}
                      onCheckedChange={setCompute}
                      className="data-[state=checked]:bg-violet-600"
                    />
                    Allocate compute now
                  </label>
                </>
              }
            >
              {compute ? (
                <>
                  {/* Start from: saved spec tiles */}
                  <div className="flex flex-col gap-2">
                    <div data-start-from className="flex flex-wrap items-center gap-1.5">
                      <Tip tip="Start from a saved spec. Editing afterwards does not change the saved one.">
                        <span className="mr-1 flex items-center gap-1 text-xs text-muted-foreground">
                          <SpecIcon aria-hidden className={cn('size-3.5', conceptColors.savedSpec.text)} />
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
                              className={cn(
                                'flex h-7 items-center gap-1.5 pl-2.5 font-mono text-xs',
                                on ? 'pr-1' : 'pr-2.5',
                              )}
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
                          <SpecIcon className={conceptColors.savedSpec.text} />
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

                  <div className="flex items-center gap-2">
                    <IconInput
                      data-command
                      icon={TerminalIcon}
                      concept="terminal"
                      tip="Command: the program the sandbox runs as its first terminal, the job agent"
                      wrapperClassName="flex-1"
                      value={command}
                      onChange={(e) => setCommand(e.target.value)}
                      placeholder={person ? 'empty: no program' : 'empty: agc runs the agent for you'}
                    />
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
                      <ChannelsCard form={form} update={update} slug={name} />
                    </div>
                  )}
                </>
              ) : (
                <EmptySlot
                  reason={
                    person ? 'No sandbox: a person does this task' : 'No sandbox: allocate compute later from the task'
                  }
                  className="justify-start"
                />
              )}
            </Card>
          </div>

          {create.error ? (
            <div className="border-t border-border px-5 pt-3">
              <ErrorText error={create.error} />
            </div>
          ) : null}
          <div className="flex items-center gap-2 border-t border-border px-5 py-3">
            <span data-summary className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">
              {summary.join(' · ')}
            </span>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button
              type="submit"
              data-create
              disabled={!ready || create.isPending}
              className={conceptColors.task.solid}
            >
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
        concept="savedSpec"
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
        <IconChip icon={conceptIcons.savedSpec} concept="savedSpec" title="Saved as" className="h-7">
          {slug}
        </IconChip>
      ) : null}
      <Button
        type="button"
        variant="outline"
        size="xs"
        className={cn('h-7 px-2.5', conceptColors.savedSpec.text)}
        disabled={pending || !slug || !canSave}
        onClick={onSave}
      >
        {label}
      </Button>
      <Button type="button" variant="ghost" size="icon-xs" aria-label="Cancel" onClick={onCancel}>
        <X />
      </Button>
    </div>
  )
}

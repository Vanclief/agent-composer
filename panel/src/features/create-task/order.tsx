// The Order card: what the task is, in words (ADR 0008 item 2). Title with
// its name (the slug, editable), the brief, and the knowledge action or
// decision it is for.
import { useMemo, type Ref } from 'react'
import { Pencil, Plus, RotateCcw, X } from 'lucide-react'

import { useKnowledge } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import { Card, Tip } from './bits'

export type OrderForm = {
  title: string
  // null follows the title's slug; a string is the user's own name.
  name: string | null
  brief: string
  for: string
  // Toast and sound when the task starts and when it is done.
  notify: boolean
}

export function blankOrder(): OrderForm {
  return { title: '', name: null, brief: '', for: '', notify: true }
}

type ForOption = { id: string; kind: 'action' | 'decision'; text: string; done: boolean }

// The first words of an item's text, for a one-line option.
function firstWords(text: string, count = 9) {
  const words = text.split(/\s+/).filter(Boolean)
  return words.length > count ? words.slice(0, count).join(' ') + '…' : words.join(' ')
}

// Plain substring match on id and text: predictable, unlike fuzzy scoring.
function containsFilter(value: string, search: string) {
  return value.toLowerCase().includes(search.trim().toLowerCase()) ? 1 : 0
}

export function OrderCard({
  order,
  update,
  slug,
  titleRef,
  pickingFor,
  onPickingFor,
}: {
  order: OrderForm
  update: (patch: Partial<OrderForm>) => void
  // The title's slug; the name when the user has not edited it.
  slug: string
  titleRef: Ref<HTMLInputElement>
  pickingFor: boolean
  onPickingFor: (open: boolean) => void
}) {
  const TaskIcon = conceptIcons.task
  const ChannelIcon = conceptIcons.channel
  const BriefIcon = conceptIcons.brief
  const name = order.name ?? slug
  const edited = order.name !== null

  return (
    <Card name="order" concept="task" label="Order" tip="What needs doing, in words">
      <div className="flex flex-col gap-1.5">
        <div className="relative min-w-0">
          <Tip tip="Title">
            <TaskIcon
              aria-hidden
              className={cn('absolute top-1/2 left-2.5 size-4 -translate-y-1/2', conceptColors.task.text)}
            />
          </Tip>
          <Input
            ref={titleRef}
            data-title
            aria-label="Title"
            value={order.title}
            onChange={(e) => update({ title: e.target.value })}
            placeholder="Review facebook ads"
            className="pl-9"
          />
        </div>
        <div className="flex items-center gap-1.5">
          <Tip tip="Task and channel name: the title's slug unless you edit it">
            <label
              className={cn(
                'flex h-7 items-center gap-1 rounded-md border px-2 font-mono text-xs',
                name ? [conceptColors.channel.bg, conceptColors.channel.border, conceptColors.channel.text] : '',
                !name && 'border-dashed border-border text-muted-foreground',
                !name && order.title.trim() && 'border-red-500/60 text-red-600 dark:text-red-400',
              )}
            >
              <ChannelIcon aria-hidden className="size-3.5 shrink-0" />
              <input
                data-name
                aria-label="Name"
                value={name}
                onChange={(e) => update({ name: e.target.value })}
                placeholder="name"
                size={Math.max(name.length, 4)}
                spellCheck={false}
                className="bg-transparent outline-none placeholder:text-muted-foreground"
              />
              <Pencil aria-hidden className="size-3 shrink-0 opacity-60" />
            </label>
          </Tip>
          {edited ? (
            <Tip tip="Back to the title's slug">
              <button
                type="button"
                data-name-reset
                aria-label="Name from title"
                onClick={() => update({ name: null })}
                className="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
              >
                <RotateCcw className="size-3.5" />
              </button>
            </Tip>
          ) : null}
        </div>
      </div>
      <div className="relative">
        <Tip tip="Brief: what to do, and what done looks like (markdown)">
          <BriefIcon aria-hidden className={cn('absolute top-2.5 left-2.5 size-4', conceptColors.brief.text)} />
        </Tip>
        <Textarea
          data-brief
          aria-label="Brief"
          rows={4}
          value={order.brief}
          onChange={(e) => update({ brief: e.target.value })}
          placeholder="What to do, and what done looks like"
          className="field-sizing-fixed min-h-24 pl-9"
        />
      </div>
      <ForPicker value={order.for} onChange={(id) => update({ for: id })} open={pickingFor} onOpen={onPickingFor} />
      <NotifySwitch checked={order.notify} onChange={(notify) => update({ notify })} />
    </Card>
  )
}

// Sent as the task's notify: a toast and a chime when it starts and when
// it is done.
function NotifySwitch({ checked, onChange }: { checked: boolean; onChange: (checked: boolean) => void }) {
  const BellIcon = conceptIcons.notify
  return (
    <Tip tip="Toast and sound when the task starts and when it is done">
      <label className="flex items-center gap-2 self-start text-xs">
        <Switch data-notify checked={checked} onCheckedChange={onChange} />
        <BellIcon aria-hidden className={cn('size-3.5', conceptColors.notify.text)} />
        Notify me
      </label>
    </Tip>
  )
}

// A combobox over the knowledge actions (open first) and decisions; the
// task's `for` is optional.
function ForPicker({
  value,
  onChange,
  open,
  onOpen,
}: {
  value: string
  onChange: (id: string) => void
  open: boolean
  onOpen: (open: boolean) => void
}) {
  const knowledge = useKnowledge()
  const options = useMemo(() => {
    const k = knowledge.data
    if (!k) {
      return []
    }
    const actions: ForOption[] = k.actions.map((a) => ({
      id: a.id,
      kind: 'action',
      text: a.text,
      done: a.done !== null,
    }))
    // Open actions first, each group in file order.
    actions.sort((a, b) => Number(a.done) - Number(b.done))
    const decisions: ForOption[] = k.decisions.map((d) => ({ id: d.id, kind: 'decision', text: d.text, done: false }))
    return [...actions, ...decisions]
  }, [knowledge.data])
  const picked = options.find((o) => o.id === value)

  function pick(id: string) {
    onChange(id)
    onOpen(false)
  }

  let chip = (
    <button
      type="button"
      data-for=""
      aria-label="Pick an action or decision"
      title="Pick an action or decision"
      onClick={() => onOpen(!open)}
      className="flex h-7 w-9 items-center justify-center rounded-md border border-dashed border-border text-muted-foreground transition-colors hover:border-zinc-400 hover:text-foreground"
    >
      <Plus className="size-3.5" />
    </button>
  )
  if (value) {
    const kind = picked?.kind ?? 'action'
    const Icon = conceptIcons[kind]
    const color = conceptColors[kind]
    chip = (
      <span
        className={cn(
          'flex h-7 min-w-0 items-center gap-1 rounded-md border pr-1 pl-2 text-xs',
          color.bg,
          color.border,
          color.text,
        )}
      >
        <button
          type="button"
          data-for={value}
          onClick={() => onOpen(!open)}
          className="flex min-w-0 items-center gap-1.5"
        >
          <Icon aria-hidden className="size-3.5 shrink-0" />
          <span className="font-mono">{value}</span>
          {picked ? <span className="truncate text-foreground/70">{firstWords(picked.text)}</span> : null}
        </button>
        <button
          type="button"
          data-for-clear
          aria-label="Clear for"
          title="Clear"
          onClick={() => onChange('')}
          className="rounded p-0.5 opacity-70 hover:bg-current/20 hover:opacity-100"
        >
          <X className="size-3.5" />
        </button>
      </span>
    )
  }

  const groups: { kind: ForOption['kind']; heading: string }[] = [
    { kind: 'action', heading: 'Actions' },
    { kind: 'decision', heading: 'Decisions' },
  ]

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex min-w-0 items-center gap-2">
        <Tip tip="The knowledge action or decision this task serves (optional)">
          <span className="w-8 shrink-0 text-xs text-muted-foreground">for</span>
        </Tip>
        {chip}
      </div>
      {open ? (
        <Command data-for-picker filter={containsFilter} className="h-auto rounded-md border border-border">
          <CommandInput autoFocus placeholder="action or decision id" className="h-8 text-xs" />
          <CommandList className="max-h-52">
            <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
            {groups.map(({ kind, heading }) => {
              const Icon = conceptIcons[kind]
              const color = conceptColors[kind]
              return (
                <CommandGroup key={kind} heading={heading}>
                  {options
                    .filter((o) => o.kind === kind)
                    .map((o) => (
                      <CommandItem
                        key={o.id}
                        value={o.id + ' ' + o.text}
                        data-for-option={o.id}
                        onSelect={() => pick(o.id)}
                        className={cn('gap-2 text-xs', o.done && 'opacity-60')}
                      >
                        <Icon aria-hidden className={cn('size-3.5 shrink-0', color.text)} />
                        <span className="shrink-0 font-mono">{o.id}</span>
                        <span className="truncate text-muted-foreground">{firstWords(o.text)}</span>
                        {o.done ? <span className="ml-auto shrink-0 text-muted-foreground">done</span> : null}
                      </CommandItem>
                    ))}
                </CommandGroup>
              )
            })}
          </CommandList>
          <ErrorText error={knowledge.error} className="m-2 text-xs" />
        </Command>
      ) : null}
    </div>
  )
}

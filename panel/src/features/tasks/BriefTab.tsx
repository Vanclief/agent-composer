// /tasks/:name/brief: what to do and what done looks like (ADR 0008), the
// criteria it is accepted on and the verdict (ADR 0010 item 6), then the
// exact TASK.md the newest sandbox's agent received (ADR 0009 item 6), all
// verbatim.
import { useState } from 'react'
import { Check, CircleCheck, Copy } from 'lucide-react'
import { useParams } from 'react-router'

import { useTask } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import type { TaskView } from '@/api/tasks'
import { cn } from '@/lib/utils'

import { ago } from './format'

export function BriefTab() {
  const { name = '' } = useParams()
  const task = useTask(name)
  if (!task.data) {
    // TaskPage shows the loading state and the error.
    return null
  }
  const newest = task.data.sandboxes[0]
  const BriefIcon = conceptIcons.brief
  const SandboxIcon = conceptIcons.sandbox

  return (
    <div className="h-full overflow-y-auto">
      <div className="flex max-w-4xl flex-col gap-4 p-6">
        <section aria-label="Brief" className="rounded-xl border border-border bg-white shadow-sm dark:bg-zinc-900/60">
          <header className="flex items-center gap-2 border-b border-border px-4 py-2 text-sm font-medium">
            <BriefIcon aria-hidden className={cn('size-4', conceptColors.brief.text)} />
            Brief
          </header>
          {task.data.brief ? (
            <pre data-testid="brief" className="px-4 py-3 font-sans text-sm leading-relaxed whitespace-pre-wrap">
              {task.data.brief}
            </pre>
          ) : (
            <EmptySlot reason="No brief" className="m-4" />
          )}
        </section>

        <Criteria task={task.data} />
        <Delivery task={task.data} />

        <section
          aria-label="What the agent received"
          className="rounded-xl border border-border bg-white shadow-sm dark:bg-zinc-900/60"
        >
          <header className="flex items-center gap-2 border-b border-border px-4 py-2 text-sm font-medium">
            <SandboxIcon aria-hidden className={cn('size-4', conceptColors.sandbox.text)} />
            What the agent received
            {newest && (
              <>
                <IconChip icon={conceptIcons.sandbox} concept="sandbox" title="The newest sandbox" className="ml-1">
                  {newest.name}
                </IconChip>
                <CopyButton text={newest.instructions} className="ml-auto" />
              </>
            )}
          </header>
          {newest ? (
            <pre
              data-testid="instructions"
              className="overflow-x-auto bg-zinc-50 px-4 py-3 font-mono text-xs leading-relaxed whitespace-pre-wrap dark:bg-zinc-950/60"
            >
              {newest.instructions}
            </pre>
          ) : (
            <EmptySlot reason="No sandbox allocated yet" className="m-4" />
          )}
        </section>
      </div>
    </div>
  )
}

// The criteria as unchecked boxes: agc does not evaluate them; the manager
// or the owner does when accepting. Hidden when there are none.
function Criteria({ task }: { task: TaskView }) {
  if (task.accept.length === 0) {
    return null
  }
  const Icon = conceptIcons.criteria
  return (
    <section
      aria-label="Accepted when"
      className="rounded-xl border border-border bg-white shadow-sm dark:bg-zinc-900/60"
    >
      <header className="flex items-baseline gap-2 border-b border-border px-4 py-2 text-sm font-medium">
        <Icon aria-hidden className={cn('size-4 self-center', conceptColors.criteria.text)} />
        <h3>Accepted when</h3>
        <span className="text-xs font-normal text-muted-foreground">
          agc does not check these; the manager or owner does
        </span>
      </header>
      <ul data-testid="criteria" className="flex flex-col gap-1.5 px-4 py-3 text-sm">
        {task.accept.map((criterion, i) => (
          <li key={i} data-testid="criterion" className="flex items-start gap-2">
            <input
              type="checkbox"
              checked={false}
              disabled
              readOnly
              aria-label={criterion}
              className="mt-0.5 size-4 shrink-0 accent-zinc-500"
            />
            <span>{criterion}</span>
          </li>
        ))}
      </ul>
    </section>
  )
}

// The agent's last delivery summary and, once done, who accepted it and
// when. Read-only; hidden when the task was never delivered nor accepted.
function Delivery({ task }: { task: TaskView }) {
  const done = task.status === 'done'
  if (!task.summary && !done) {
    return null
  }
  const Icon = conceptIcons.delivery
  return (
    <section aria-label="Delivery" className="rounded-xl border border-border bg-white shadow-sm dark:bg-zinc-900/60">
      <header className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-2 text-sm font-medium">
        <Icon aria-hidden className={cn('size-4', conceptColors.delivery.text)} />
        Summary
        {task.delivered_at && !done ? (
          <span
            className="text-xs font-normal text-muted-foreground"
            title={new Date(task.delivered_at).toLocaleString()}
          >
            delivered {ago(task.delivered_at)}
          </span>
        ) : null}
        {done ? (
          <span
            data-testid="accepted"
            title={task.done_at ? new Date(task.done_at).toLocaleString() : undefined}
            className="ml-auto inline-flex items-center gap-1.5 text-xs font-normal text-emerald-700 dark:text-emerald-400"
          >
            <CircleCheck aria-hidden className="size-3.5" />
            {task.accepted_by ? `Accepted by ${task.accepted_by}` : 'Accepted'}
            {task.done_at ? ` ${ago(task.done_at)}` : ''}
          </span>
        ) : null}
      </header>
      {task.summary ? (
        <pre data-testid="summary" className="px-4 py-3 font-sans text-sm leading-relaxed whitespace-pre-wrap">
          {task.summary}
        </pre>
      ) : (
        <EmptySlot reason="Accepted without a delivery" className="m-4" />
      )}
    </section>
  )
}

// Copies text; the icon turns into a check for a moment, or the label says
// the browser refused.
function CopyButton({ text, className }: { text: string; className?: string }) {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle')

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      setState('copied')
    } catch {
      setState('failed')
    }
    window.setTimeout(() => setState('idle'), 1500)
  }

  return (
    <Button variant="outline" size="xs" onClick={copy} className={className} title="Copy the text">
      {state === 'copied' ? <Check className="text-emerald-600" /> : <Copy />}
      {state === 'failed' ? 'Copy: blocked by the browser' : 'Copy'}
    </Button>
  )
}

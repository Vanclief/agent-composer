// /tasks/:name/brief: what to do and what done looks like (ADR 0008), then
// the exact TASK.md the newest sandbox's agent received (ADR 0009 item 6),
// both verbatim.
import { useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { useParams } from 'react-router'

import { useTask } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

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

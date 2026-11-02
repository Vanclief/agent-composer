// A thread: the message, its replies oldest first, and a composer that
// posts with reply_to. A pane on the right from lg up; below that a sheet
// over the channel.
import { X } from 'lucide-react'

import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { cn } from '@/lib/utils'

import { Composer } from './Composer'
import { useChannel } from './context'
import { GroupRows } from './MessageList'
import { isSystem, MessageRow, QuestionCard, SystemLine } from './parts'
import { isQuestion } from './questions'
import type { Message } from '@/api/msg'

function Parent({ message }: { message: Message }) {
  if (isSystem(message)) {
    return <SystemLine message={message} />
  }
  if (isQuestion(message)) {
    return <QuestionCard message={message} />
  }
  return <MessageRow message={message} first inThread />
}

// Consecutive replies from one participant share a header.
function runs(messages: Message[]) {
  const out: Message[][] = []
  for (const m of messages) {
    const last = out.at(-1)
    if (last && last[0].participant_id === m.participant_id) {
      last.push(m)
    } else {
      out.push([m])
    }
  }
  return out
}

export function ThreadPane({ id, onClose }: { id: number; onClose: () => void }) {
  const { channel, identity, threads } = useChannel()
  const parent = threads.byID.get(id)
  const replies = threads.replies(id)
  const Icon = conceptIcons.thread
  return (
    <aside
      aria-label="Thread"
      data-testid="thread-pane"
      className="fixed inset-0 z-40 flex flex-col bg-background lg:static lg:inset-auto lg:z-auto lg:w-[26rem] lg:shrink-0 lg:border-l lg:border-border"
    >
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-4">
        <Icon aria-hidden className={cn('size-4', conceptColors.thread.text)} />
        <span className="text-sm font-semibold">Thread</span>
        <span className="font-mono text-xs text-muted-foreground">{channel}</span>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close thread"
          title="Close thread"
          className="ml-auto grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-zinc-100 hover:text-foreground dark:hover:bg-zinc-800"
        >
          <X className="size-4" />
        </button>
      </div>
      {parent ? (
        <>
          <div data-testid="thread-messages" className="min-h-0 flex-1 overflow-y-auto px-3 py-3 text-sm">
            <Parent message={parent} />
            <div className="flex items-center gap-2 py-2 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              <span>{replies.length === 1 ? '1 reply' : `${replies.length} replies`}</span>
              <span className="h-px flex-1 bg-border" />
            </div>
            <ol className="flex flex-col gap-2">
              {runs(replies).map((run) => (
                <GroupRows key={run[0].id} messages={run} inThread />
              ))}
            </ol>
          </div>
          <Composer channel={channel} identity={identity} replyTo={id} />
        </>
      ) : (
        <div className="flex flex-1 items-center justify-center p-6">
          <EmptySlot reason={`Message #${id} is not loaded`} />
        </div>
      )}
    </aside>
  )
}

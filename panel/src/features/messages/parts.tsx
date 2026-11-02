// The pieces of a feed: a message row (gutter, header, body, thread
// footer, the hover "Reply in thread"), agc's system lines, and question
// cards with their Answer box.
import { useMemo, useState, type KeyboardEvent } from 'react'
import { Check, ChevronRight, Hexagon } from 'lucide-react'
import { Link } from 'react-router'

import type { Message } from '@/api/msg'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { Avatar } from '@/components/avatar'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import { useChannel } from './context'
import { MarkdownText, MessageBody } from './MessageBody'
import { answerOf } from './questions'
import { AcceptActions, QuestionBody } from './QuestionParts'
import { parseQuestion } from './questionShape'
import { useSend } from './send'
import { clock, exact, relative } from './time'

// The reserved participant agc posts as (ADR 0009).
export const AGC = 'agc'

export function isSystem(m: Message) {
  return m.from === AGC && !m.task
}

function RelativeTime({ iso, className }: { iso: string; className?: string }) {
  const { now } = useChannel()
  return (
    <time dateTime={iso} title={exact(iso)} className={cn('font-mono text-xs text-muted-foreground', className)}>
      {relative(iso, now)}
    </time>
  )
}

// A human or an agent (its task's icon), after the sender's name.
function SenderIcon({ message }: { message: Message }) {
  const human = !message.task
  const Icon = human ? conceptIcons.human : conceptIcons.task
  return (
    <Icon
      aria-hidden
      data-concept={human ? 'human' : 'task'}
      data-slot="sender-icon"
      className={cn('size-3.5 shrink-0', human ? conceptColors.human.text : conceptColors.task.text)}
    />
  )
}

// The sender's avatar, @name, its kind (a human or a task's icon), the
// agent's task as a link, and when unless the row's time column has it.
function Header({ message, time = true }: { message: Message; time?: boolean }) {
  const human = !message.task
  return (
    <div data-slot="header" className="flex items-center gap-2">
      <Avatar name={message.from} size="xs" human={human} />
      <span
        data-testid="author"
        data-kind={human ? 'human' : 'agent'}
        title={human ? 'Human' : `Agent in task ${message.task}`}
        className="font-mono text-sm font-semibold text-foreground"
      >
        @{message.from}
      </span>
      <SenderIcon message={message} />
      {!human && (
        <Link to={`/tasks/${encodeURIComponent(message.task)}`} title={`Task ${message.task}`}>
          <IconChip icon={conceptIcons.task} concept="task" className="hover:brightness-125">
            {message.task}
          </IconChip>
        </Link>
      )}
      {time && <RelativeTime iso={message.created_at} />}
    </div>
  )
}

// "N replies" under a message with a thread; opens the thread pane.
function ThreadFooter({ message, replies }: { message: Message; replies: Message[] }) {
  const { openThread } = useChannel()
  const Icon = conceptIcons.thread
  const last = replies.at(-1)
  if (!last) {
    return null
  }
  return (
    <button
      type="button"
      data-testid="thread-footer"
      onClick={() => openThread(message.id)}
      className="mt-0.5 flex w-fit items-center gap-1.5 rounded-md px-1 py-0.5 text-xs hover:bg-zinc-200/60 dark:hover:bg-zinc-800"
    >
      <Icon aria-hidden className={cn('size-3.5', conceptColors.thread.text)} />
      <span className={cn('font-medium', conceptColors.thread.text)}>
        {replies.length === 1 ? '1 reply' : `${replies.length} replies`}
      </span>
      <span className="text-muted-foreground">
        last <RelativeTime iso={last.created_at} />
      </span>
    </button>
  )
}

function ReplyAction({ id }: { id: number }) {
  const { openThread } = useChannel()
  const Icon = conceptIcons.thread
  return (
    <button
      type="button"
      onClick={() => openThread(id)}
      aria-label="Reply in thread"
      title="Reply in thread"
      className="absolute -top-2.5 right-2 hidden items-center gap-1 rounded-md border border-border bg-background px-1.5 py-0.5 text-xs text-muted-foreground shadow-sm group-hover:flex group-focus-within:flex hover:text-foreground"
    >
      <Icon aria-hidden className="size-3.5" /> Reply in thread
    </button>
  )
}

// One message: the gutter (sender icon by the header, the clock on hover
// after), the header when first, the body, and the thread footer. inThread
// leaves out the footer and the reply action.
export function MessageRow({ message, first, inThread }: { message: Message; first: boolean; inThread?: boolean }) {
  const { threads } = useChannel()
  const replies = inThread ? [] : threads.replies(message.id)
  return (
    <div
      data-testid="message"
      data-message-id={message.id}
      className="group relative flex gap-2 rounded-md px-1 py-0.5 hover:bg-zinc-100 dark:hover:bg-zinc-800/40"
    >
      <div data-slot="stamp" className="flex w-11 shrink-0 justify-end pt-0.5">
        {first ? (
          <RelativeTime iso={message.created_at} className="text-[10px]" />
        ) : (
          <time
            dateTime={message.created_at}
            title={exact(message.created_at)}
            className="font-mono text-[10px] text-muted-foreground opacity-0 group-hover:opacity-100"
          >
            {clock(message.created_at)}
          </time>
        )}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5" title={`#${message.id}`}>
        {first && <Header message={message} time={false} />}
        <MessageBody body={message.body} />
        {replies.length > 0 && <ThreadFooter message={message} replies={replies} />}
      </div>
      {!inThread && <ReplyAction id={message.id} />}
    </div>
  )
}

// ---------- agc ----------

// agc's mark: the logo in the top left, small and muted.
export function AgcMark({ className }: { className?: string }) {
  return (
    <span
      title="agc"
      className={cn(
        'grid size-4 shrink-0 place-items-center rounded-sm bg-zinc-400 text-white dark:bg-zinc-600 dark:text-zinc-100',
        className,
      )}
    >
      <Hexagon aria-hidden className="size-2.5" />
    </span>
  )
}

// What an agc post is: the task's instructions (TASK.md, a heading first),
// a status line (ADR 0010), or a plain notice.
function systemKind(body: string) {
  if (body.startsWith('# ')) {
    return 'instructions'
  }
  if (body.startsWith('Delivered:')) {
    return 'delivered'
  }
  if (body.startsWith('Accepted by ')) {
    return 'accepted'
  }
  if (body.startsWith('Rejected by ')) {
    return 'rejected'
  }
  return 'notice'
}

const statusIcons = {
  delivered: conceptIcons.delivery,
  accepted: conceptIcons.criteria,
  rejected: conceptIcons.criteria,
}

// A compact muted line, not a chat bubble. The instructions collapse to
// "Instructions" until expanded.
export function SystemLine({ message }: { message: Message }) {
  const kind = systemKind(message.body)
  const [open, setOpen] = useState(false)
  const StatusIcon = kind in statusIcons ? statusIcons[kind as keyof typeof statusIcons] : undefined
  return (
    <div
      data-testid="message"
      data-message-id={message.id}
      className="group relative flex gap-2 rounded-md px-1 py-0.5 text-xs text-muted-foreground"
    >
      <div data-slot="stamp" className="flex w-11 shrink-0 justify-end pt-px">
        <RelativeTime iso={message.created_at} className="text-[10px]" />
      </div>
      <div
        data-testid="system-line"
        data-system={kind}
        title={`#${message.id}`}
        className="flex min-w-0 flex-1 items-start gap-1.5"
      >
        <span data-testid="author" className="sr-only">
          @{message.from}
        </span>
        <AgcMark />
        {StatusIcon && <StatusIcon aria-hidden data-status-icon={kind} className="mt-0.5 size-3.5 shrink-0" />}
        {kind === 'instructions' ? (
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <button
              type="button"
              aria-expanded={open}
              onClick={() => setOpen(!open)}
              className="flex w-fit items-center gap-1 font-medium hover:text-foreground"
            >
              <ChevronRight aria-hidden className={cn('size-3.5 transition-transform', open && 'rotate-90')} />
              <conceptIcons.brief aria-hidden className="size-3.5" />
              Instructions
            </button>
            {open && (
              <MarkdownText
                body={message.body}
                className="rounded-md border border-border bg-zinc-50 p-3 text-foreground dark:bg-zinc-900/60"
              />
            )}
          </div>
        ) : (
          <MarkdownText body={message.body} className="flex-1 text-xs leading-4" />
        )}
      </div>
    </div>
  )
}

// ---------- questions ----------

function AnswerBox({ question }: { question: Message }) {
  const { channel, identity } = useChannel()
  const post = useSend(identity)
  const [draft, setDraft] = useState('')
  const joined = Boolean(identity.participantID)
  const empty = draft.trim() === ''
  let label = 'Answer'
  if (!joined) {
    label = identity.joining ? 'Answer: joining' : 'Answer: join first'
  } else if (empty) {
    label = 'Answer: empty'
  } else if (post.isPending) {
    label = 'Answering'
  }
  const disabled = !joined || empty || post.isPending

  async function answer() {
    if (disabled) {
      return
    }
    const posted = await post.send({ channel, body: draft, reply_to: question.id })
    if (posted) {
      setDraft('')
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      answer()
    }
  }

  return (
    <div className="flex flex-col gap-1.5">
      <ErrorText error={post.error} />
      <div className="flex items-end gap-2">
        <Textarea
          aria-label={`Answer @${question.from}`}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={`Answer @${question.from}`}
          rows={1}
          className="max-h-32 min-h-9 flex-1 resize-none bg-background py-1.5 text-sm dark:bg-background"
        />
        <Button
          size="sm"
          disabled={disabled}
          onClick={answer}
          variant="outline"
          data-act="answer"
          className={conceptColors.question.selected}
        >
          <conceptIcons.message /> {label}
        </Button>
      </div>
    </div>
  )
}

// A question: who asks, the body (the title, labelled rows and View diff,
// QuestionParts.tsx), Accept and Reject… for "Accept <task>?", then the
// Answer box until someone other than the asker replies; then "Answered by
// @name" and that first answer. At most 72ch wide, from the message
// column's left edge.
export function QuestionCard({ message }: { message: Message }) {
  const { threads, questions } = useChannel()
  const replies = threads.replies(message.id)
  const answer = answerOf(message, replies)
  const info = questions.get(message.id)
  const answeredBy = answer?.from ?? info?.answered_by ?? ''
  const answeredAt = answer?.created_at ?? info?.answered_at ?? null
  const shape = useMemo(() => parseQuestion(message.body), [message.body])
  const Icon = conceptIcons.question
  const color = conceptColors.question
  return (
    <div data-testid="message" data-message-id={message.id} className="group relative flex gap-2 px-1 py-1">
      <div data-slot="stamp" className="flex w-11 shrink-0 justify-end pt-2">
        <RelativeTime iso={message.created_at} className="text-[10px]" />
      </div>
      <div
        data-testid="question-card"
        data-state={answeredBy ? 'answered' : 'open'}
        title={`#${message.id}`}
        className={cn(
          'flex max-w-[72ch] min-w-0 flex-1 flex-col gap-2 rounded-md border px-4 py-3',
          answeredBy ? 'border-border' : [color.border, 'bg-pink-500/5'],
        )}
      >
        <div className="flex items-center gap-2">
          <span data-slot="kind" className="inline-flex shrink-0 items-center gap-1 text-xs">
            <Icon aria-hidden className={cn('size-4 shrink-0', answeredBy ? 'text-muted-foreground' : color.text)} />
            {message.kind}
          </span>
          <Header message={message} time={false} />
        </div>
        <QuestionBody shape={shape} />
        {shape.accept && <AcceptActions question={message} task={shape.accept} />}
        {answeredBy ? (
          <div data-testid="answer" className="flex flex-col gap-0.5 border-l-2 border-border pl-2">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Check aria-hidden className="size-3.5" />
              <span data-testid="answered-by">Answered by @{answeredBy}</span>
              {answeredAt && <RelativeTime iso={answeredAt} className="text-[10px]" />}
            </div>
            {answer && <MessageBody body={answer.body} />}
          </div>
        ) : (
          <AnswerBox question={message} />
        )}
        {replies.length > 0 && <ThreadFooter message={message} replies={replies} />}
      </div>
      <ReplyAction id={message.id} />
    </div>
  )
}

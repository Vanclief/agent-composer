// A question's body in the reviewer's shape (questionShape.ts): the title,
// then "Changes", "Asks" and "Diff" as labelled rows, a diff link as a
// "View diff" button, the rest as markdown. Then, for "Accept <task>?" on
// a task in review that I own, Accept and Reject… posting the channel
// commands of ADR 0010 (core/resources/tasks/commands.go); once agc has
// answered, the task's status from the tasks query, verbatim.
import { useEffect, useState, type KeyboardEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { ExternalLink } from 'lucide-react'

import { queryKeys, useTasks } from '@/api/hooks'
import type { Message } from '@/api/msg'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import { useChannel } from './context'
import { MarkdownText, MessageBody } from './MessageBody'
import type { QuestionShape } from './questionShape'
import { useSend } from './send'

// One step larger than a message (text-sm), with room between lines.
const BODY = 'text-base leading-7'

function ViewDiff({ url }: { url: string }) {
  return (
    <Button asChild size="sm" variant="outline" className="w-fit">
      <a href={url} target="_blank" rel="noreferrer" data-testid="view-diff" title={url}>
        <ExternalLink aria-hidden /> View diff
      </a>
    </Button>
  )
}

export function QuestionBody({ shape }: { shape: QuestionShape }) {
  return (
    <div data-testid="question-body" className="flex min-w-0 flex-col gap-1.5">
      {shape.title && (
        <MarkdownText body={shape.title} className="text-lg leading-7 font-semibold text-foreground [&_p]:my-0" />
      )}
      {shape.parts.map((part, i) => {
        if (part.type === 'text') {
          return <MessageBody key={i} body={part.text} className={BODY} />
        }
        if (part.type === 'link') {
          return <ViewDiff key={i} url={part.url} />
        }
        return (
          <div key={i} data-testid="question-row" data-row={part.label} className="flex items-baseline gap-3">
            <span className="w-16 shrink-0 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              {part.label}
            </span>
            {part.url ? (
              <ViewDiff url={part.url} />
            ) : (
              <MarkdownText body={part.value} className={cn(BODY, 'flex-1 [&_p]:my-0')} />
            )}
          </div>
        )
      })}
    </div>
  )
}

// How long the card keeps asking the tasks query after a command was
// posted, while agc runs it.
const SETTLE_MS = 30_000

export function AcceptActions({ question, task: name }: { question: Message; task: string }) {
  const { channel, identity, me } = useChannel()
  const tasks = useTasks()
  const queryClient = useQueryClient()
  const post = useSend(identity)
  const [rejecting, setRejecting] = useState(false)
  const [why, setWhy] = useState('')
  const [sent, setSent] = useState<'accept' | 'reject' | ''>('')
  const task = tasks.data?.find((t) => t.name === name)

  const status = task?.status ?? ''
  // While agc runs the command, read the task again every second.
  useEffect(() => {
    if (!sent || status !== 'review') {
      return
    }
    const started = Date.now()
    const timer = window.setInterval(() => {
      if (Date.now() - started > SETTLE_MS) {
        window.clearInterval(timer)
        return
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.tasks })
    }, 1000)
    return () => window.clearInterval(timer)
  }, [sent, status, queryClient])

  if (!task) {
    return null
  }

  const mine = task.owner === me
  if (status !== 'review' || !mine) {
    return (
      <div data-testid="accept-state" className="flex items-center gap-2 text-sm text-muted-foreground">
        <conceptIcons.task aria-hidden className="size-4" />
        <span className="font-mono">{task.name}</span>
        <StatusPill status={task.status} />
        {task.accepted_by && <span data-testid="accepted-by">Accepted by @{task.accepted_by}</span>}
      </div>
    )
  }

  const joined = Boolean(identity.participantID)
  const busy = post.isPending || sent !== ''
  const inbox = channel === `#${me}`
  const inTask = channel === `#${task.name}`

  // ADR 0010: "/accept <task>" counts only in the owner's inbox; elsewhere
  // "/accept" goes in #<task>. There is no "/reject <task> <why>" in the
  // inbox, so a reject always goes in #<task>.
  async function accept() {
    const ok = await post.send(
      inbox
        ? { channel, body: `/accept ${name}`, reply_to: question.id }
        : { channel: `#${name}`, body: '/accept', reply_to: inTask ? question.id : undefined },
    )
    if (ok) {
      setSent('accept')
    }
  }

  async function reject() {
    const reason = why.trim()
    if (!reason) {
      return
    }
    const ok = await post.send({
      channel: `#${name}`,
      body: `/reject ${reason}`,
      reply_to: inTask ? question.id : undefined,
    })
    if (ok) {
      setSent('reject')
      setWhy('')
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
      e.preventDefault()
      reject()
    }
  }

  let acceptLabel = 'Accept'
  if (!joined) {
    acceptLabel = identity.joining ? 'Accept: joining' : 'Accept: join first'
  } else if (post.isPending && !rejecting) {
    acceptLabel = 'Accepting'
  } else if (sent === 'accept') {
    acceptLabel = 'Accept: sent, waiting for agc'
  }
  let sendLabel = 'Send'
  if (!joined) {
    sendLabel = identity.joining ? 'Send: joining' : 'Send: join first'
  } else if (why.trim() === '') {
    sendLabel = 'Send: needs a reason'
  } else if (post.isPending) {
    sendLabel = 'Rejecting'
  } else if (sent === 'reject') {
    sendLabel = 'Send: sent, waiting for agc'
  }

  return (
    <div data-testid="accept-actions" className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" disabled={!joined || busy} onClick={accept} data-act="accept">
          <conceptIcons.criteria aria-hidden /> {acceptLabel}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          aria-expanded={rejecting}
          onClick={() => setRejecting(!rejecting)}
          data-act="reject"
        >
          Reject…
        </Button>
        <StatusPill status={task.status} className="ml-auto" />
      </div>
      {rejecting && (
        <div className="flex items-center gap-2">
          <Input
            aria-label={`Why reject ${name}`}
            placeholder="Why"
            value={why}
            onChange={(e) => setWhy(e.target.value)}
            onKeyDown={onKeyDown}
            autoFocus
            className="h-8 flex-1"
          />
          <Button
            size="sm"
            variant="outline"
            disabled={!joined || why.trim() === '' || busy}
            onClick={reject}
            data-act="reject-send"
          >
            {sendLabel}
          </Button>
        </div>
      )}
      <ErrorText error={post.error} />
    </div>
  )
}

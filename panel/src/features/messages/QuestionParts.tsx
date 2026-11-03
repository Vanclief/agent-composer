// A question's body in the reviewer's shape (questionShape.ts): the title,
// then "Changes" and "Asks" as labelled rows, "Before" and "After" images
// side by side, the diff as a small muted "Diff" link after the rows (the
// owner does not read code, Franco 2026-11-02), the rest as markdown.
// Then, for "Accept <task>?" on a task in review that I own, Accept and
// Send back… posting the channel commands of ADR 0010
// (core/resources/tasks/commands.go); once agc has answered, the task's
// status from the tasks query, verbatim.
import { useEffect, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { queryKeys, useTasks } from '@/api/hooks'
import type { Message } from '@/api/msg'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { ShotImages } from '@/components/shot-images'
import { StatusPill } from '@/components/status-pill'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import { useChannel } from './context'
import { MarkdownText, MessageBody } from './MessageBody'
import type { QuestionPart, QuestionShape } from './questionShape'
import { useSend } from './send'

// The same size as a message body, with room between lines.
const BODY = 'text-base leading-7'

const LABEL = 'w-16 shrink-0 text-[11px] font-medium tracking-wide text-muted-foreground uppercase'

function DiffLink({ url }: { url: string }) {
  return (
    <a
      href={url}
      target="_blank"
      rel="noreferrer"
      data-testid="diff-link"
      title={url}
      className="w-fit text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
    >
      Diff
    </a>
  )
}

type ImageRow = Extract<QuestionPart, { type: 'row' }>

export function QuestionBody({ shape }: { shape: QuestionShape }) {
  const images = shape.parts.filter((p): p is ImageRow => p.type === 'row' && Boolean(p.image))
  let diff = ''
  let lastRow = -1
  const items: ReactNode[] = []
  shape.parts.forEach((part, i) => {
    if (part.type === 'text') {
      items.push(<MessageBody key={i} body={part.text} className={BODY} />)
      return
    }
    if (part.type === 'link' || (part.label === 'Diff' && part.url)) {
      if (!diff) {
        diff = part.url ?? ''
        return
      }
    }
    if (part.type === 'link') {
      items.push(<MessageBody key={i} body={part.url} className={BODY} />)
      return
    }
    if (part.image) {
      if (part === images[0]) {
        items.push(
          <ShotImages
            key={i}
            testId="question-images"
            shots={images.map((row) => ({ label: row.label, url: row.url ?? '' }))}
          />,
        )
        lastRow = items.length - 1
      }
      return
    }
    items.push(
      <div key={i} data-testid="question-row" data-row={part.label} className="flex items-baseline gap-3">
        <span className={LABEL}>{part.label}</span>
        <MarkdownText body={part.value} className={cn(BODY, 'flex-1 [&_p]:my-0')} />
      </div>,
    )
    lastRow = items.length - 1
  })
  if (diff) {
    items.splice(lastRow + 1, 0, <DiffLink key="diff" url={diff} />)
  }
  return (
    <div data-testid="question-body" className="flex min-w-0 flex-col gap-1.5">
      {shape.title && (
        <MarkdownText body={shape.title} className="text-lg leading-7 font-semibold text-foreground [&_p]:my-0" />
      )}
      {items}
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
  const [sendingBack, setSendingBack] = useState(false)
  const [feedback, setFeedback] = useState('')
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
  // inbox, so a send back is "/reject <feedback>" in #<task>, and the same
  // feedback answers the question where it was asked.
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

  async function sendBack() {
    const text = feedback.trim()
    if (!text || busy) {
      return
    }
    const ok = await post.send({
      channel: `#${name}`,
      body: `/reject ${text}`,
      reply_to: inTask ? question.id : undefined,
    })
    if (!ok) {
      return
    }
    setSent('reject')
    if (!inTask) {
      await post.send({ channel, body: text, reply_to: question.id })
    }
    setFeedback('')
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
      e.preventDefault()
      sendBack()
    }
  }

  let acceptLabel = 'Accept'
  if (!joined) {
    acceptLabel = identity.joining ? 'Accept: joining' : 'Accept: join first'
  } else if (post.isPending && !sendingBack) {
    acceptLabel = 'Accepting'
  } else if (sent === 'accept') {
    acceptLabel = 'Accept: sent, waiting for agc'
  }
  let sendLabel = 'Send back'
  if (!joined) {
    sendLabel = identity.joining ? 'Send back: joining' : 'Send back: join first'
  } else if (post.isPending) {
    sendLabel = 'Sending back'
  } else if (sent === 'reject') {
    sendLabel = 'Send back: sent, waiting for agc'
  } else if (feedback.trim() === '') {
    sendLabel = 'Send back: write feedback'
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
          aria-expanded={sendingBack}
          onClick={() => setSendingBack(!sendingBack)}
          data-act="send-back"
        >
          Send back…
        </Button>
        <StatusPill status={task.status} className="ml-auto" />
      </div>
      {sendingBack && (
        <div className="flex items-center gap-2">
          <Input
            aria-label={`Feedback for ${name}`}
            placeholder="Feedback"
            value={feedback}
            onChange={(e) => setFeedback(e.target.value)}
            onKeyDown={onKeyDown}
            autoFocus
            className="h-8 flex-1"
          />
          <Button
            size="sm"
            variant="outline"
            disabled={!joined || feedback.trim() === '' || busy}
            onClick={sendBack}
            data-act="send-back-send"
          >
            {sendLabel}
          </Button>
        </div>
      )}
      <ErrorText error={post.error} />
    </div>
  )
}

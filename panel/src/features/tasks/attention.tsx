// Attention is personal (panel/CLAUDE.md, rtsdesign rule 14): who a task
// waits on, and what they have to do.
// - review waits on the manager (ADR 0010: the manager or the owner
//   accepts), or on the owner when the owner manages it or the manager never
//   joined messaging (no participant of that name in
//   GET /api/msg/participants?all=1; the signed-in person counts as joined);
// - an open question in #<task> waits on whom it is for (the questions
//   query with for=<me>, so only mine are known);
// - a crashed or failed sandbox of a task not done waits on the owner.
// What waits on me is an act (accept, answer, restart) with the place to do
// it; what waits on someone else is only their name.
import { useCallback } from 'react'
import { Link } from 'react-router'

import { useAllParticipants, useTasks } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { useMyName } from '@/features/messages/identity'
import { useOpenQuestions } from '@/features/messages/questions'
import { cn } from '@/lib/utils'

export type ActName = 'accept' | 'answer' | 'restart'

export type Act = {
  act: ActName
  // Where the act is done.
  to: string
  title: string
}

export type Attention = {
  // What waits on me, in a fixed order: accept, answer, restart.
  mine: Act[]
  // Who else something waits on, each once.
  others: string[]
}

const NONE: Attention = { mine: [], others: [] }

const BROKEN = ['crashed', 'failed']

// A function from a task to what in it waits on whom.
export function useAttention() {
  const me = useMyName()
  const participants = useAllParticipants()
  const questions = useOpenQuestions()
  const joined = participants.data
  const all = questions.all

  return useCallback(
    (task: TaskView): Attention => {
      const mine: Act[] = []
      const others: string[] = []
      const waitOn = (who: string, act: Act) => {
        if (!who) {
          return
        }
        if (who === me) {
          mine.push(act)
        } else if (!others.includes(who)) {
          others.push(who)
        }
      }
      const page = `/tasks/${encodeURIComponent(task.name)}`

      if (task.status === 'review') {
        // Until the participants load (or if the list fails) the manager is
        // taken as joined.
        const managerJoined = task.manager === me || !joined || joined.some((p) => p.name === task.manager)
        const who = task.manager && task.manager !== task.owner && managerJoined ? task.manager : task.owner
        waitOn(who, { act: 'accept', to: page, title: `Delivered: accept or reject ${task.name}` })
      }

      const asked = all.filter((q) => q.channel === `#${task.name}`)
      if (asked.length > 0) {
        mine.push({
          act: 'answer',
          to: `/messages/${encodeURIComponent(task.name)}?thread=${asked[0].id}`,
          title: asked.length === 1 ? `@${asked[0].from} asks: ${asked[0].body}` : `${asked.length} questions for you`,
        })
      }

      const broken = task.status === 'done' ? undefined : task.sandboxes.find((b) => BROKEN.includes(b.status))
      if (broken) {
        waitOn(task.owner, {
          act: 'restart',
          to: `${page}/compute?sandbox=${encodeURIComponent(broken.name)}`,
          title: `Sandbox ${broken.name} ${broken.status}${broken.status_error ? `: ${broken.status_error}` : ''}`,
        })
      }

      if (mine.length === 0 && others.length === 0) {
        return NONE
      }
      return { mine, others }
    },
    [me, joined, all],
  )
}

// How many tasks wait on me: the Tasks nav badge.
export function useMyTaskCount() {
  const tasks = useTasks()
  const attention = useAttention()
  return (tasks.data ?? []).filter((t) => attention(t).mine.length > 0).length
}

// A lit amber chip naming the act; it opens the place to do it.
export function ActChip({ act, className }: { act: Act; className?: string }) {
  return (
    <Link
      to={act.to}
      data-testid="act"
      data-act={act.act}
      title={act.title}
      className={cn(
        'relative z-10 inline-flex shrink-0 items-center rounded-full bg-amber-500 px-1.5 text-[11px] leading-[18px] font-medium text-amber-950 shadow-[0_0_6px_0] shadow-amber-500/50 hover:bg-amber-400 focus-visible:ring-2 focus-visible:ring-amber-300 focus-visible:outline-none',
        className,
      )}
    >
      {act.act}
    </Link>
  )
}

// Dim "· @name" after the pill: it waits on someone else.
export function WaitsOn({ names, className }: { names: string[]; className?: string }) {
  if (names.length === 0) {
    return null
  }
  return (
    <span
      data-testid="waits-on"
      title={`Waiting on ${names.map((n) => '@' + n).join(', ')}`}
      className={cn('shrink-0 text-xs whitespace-nowrap text-muted-foreground', className)}
    >
      · {names.map((n) => '@' + n).join(' ')}
    </span>
  )
}

// The Tasks nav badge: a lit amber count, nothing at zero.
export function TasksBadge({ count, testid = 'tasks-badge' }: { count: number; testid?: string }) {
  if (count === 0) {
    return null
  }
  return (
    <span
      data-testid={testid}
      title={count === 1 ? '1 task waits on you' : `${count} tasks wait on you`}
      className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-amber-500 px-1.5 font-mono text-xs leading-5 font-semibold text-amber-950 tabular-nums"
    >
      {count}
    </span>
  )
}

// The lit amber mark at a row's left edge: this waits on me. The row needs
// relative positioning.
export function AttentionMark({ className }: { className?: string }) {
  return (
    <span
      aria-hidden
      data-testid="attention-mark"
      className={cn(
        'absolute inset-y-1 left-0 w-0.5 rounded-full bg-amber-500 shadow-[0_0_6px_1px] shadow-amber-500/60',
        className,
      )}
    />
  )
}

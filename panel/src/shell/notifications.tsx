// Toasts and the chime for the live feed: "Task started" on task.created
// and "Task done" on task.done, only for tasks with notify on. Each has an
// Open action to /tasks/<name>. Toasts go top-right; one id per task and
// event, so a repeated event updates its toast instead of stacking.
import { useCallback } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import type { LiveEvent } from '@/api/events'
import type { TaskView } from '@/api/tasks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
import { cn } from '@/lib/utils'

import { playChime } from './sound'

// How much of the summary shows before "Show more".
export const SUMMARY_PREVIEW = 240
const STARTED_MS = 8000
const DONE_MS = 12_000

function agentLabel(task: TaskView) {
  return task.model ? `${task.assignee} (${task.model})` : task.assignee
}

function TaskIcon() {
  const Icon = conceptIcons.task
  return <Icon aria-hidden className={cn('size-4', conceptColors.task.text)} />
}

function StartedBody({ task }: { task: TaskView }) {
  const sandbox = task.sandboxes[0]?.name
  return (
    <div data-testid="toast-task" data-task={task.name} className="flex min-w-0 flex-col gap-1.5">
      <span className="truncate font-medium text-foreground">{task.title}</span>
      <span className="flex flex-wrap items-center gap-1">
        {task.assignee ? (
          <IconChip
            icon={task.model ? conceptIcons.agent : conceptIcons.human}
            concept={task.model ? 'agent' : 'human'}
            title="Assignee (model)"
          >
            {agentLabel(task)}
          </IconChip>
        ) : null}
        {sandbox ? (
          <IconChip icon={conceptIcons.sandbox} concept="sandbox" title="Sandbox">
            {sandbox}
          </IconChip>
        ) : null}
      </span>
    </div>
  )
}

// Expanding re-issues the toast (onToggle), so sonner measures its new
// height; state inside the description would overflow the toast.
function DoneBody({
  task,
  summary,
  expanded,
  onToggle,
}: {
  task: TaskView
  summary: string
  expanded: boolean
  onToggle: () => void
}) {
  const long = summary.length > SUMMARY_PREVIEW
  const shown = long && !expanded ? summary.slice(0, SUMMARY_PREVIEW).trimEnd() + '…' : summary
  return (
    <div data-testid="toast-task" data-task={task.name} className="flex min-w-0 flex-col gap-1">
      <span className="truncate font-medium text-foreground">{task.title}</span>
      {summary ? (
        <p
          data-testid="toast-summary"
          className={cn('text-xs whitespace-pre-wrap text-muted-foreground', expanded && 'max-h-60 overflow-y-auto')}
        >
          {shown}
        </p>
      ) : null}
      {long ? (
        <button
          type="button"
          onClick={onToggle}
          className="self-start text-xs font-medium text-foreground underline-offset-2 hover:underline"
        >
          {expanded ? 'Show less' : 'Show more'}
        </button>
      ) : null}
    </div>
  )
}

type Open = { label: string; onClick: () => void }

function showDone(task: TaskView, summary: string, open: Open, expanded: boolean) {
  toast('Task done', {
    id: `task.done:${task.id}`,
    position: 'top-right',
    icon: <TaskIcon />,
    description: (
      <DoneBody
        task={task}
        summary={summary}
        expanded={expanded}
        onToggle={() => showDone(task, summary, open, !expanded)}
      />
    ),
    action: open,
    duration: DONE_MS,
  })
}

// The handler to pass to useLiveEvents.
export function useTaskNotifications() {
  const navigate = useNavigate()
  return useCallback(
    (event: LiveEvent) => {
      if (event.type !== 'task.created' && event.type !== 'task.done') {
        return
      }
      const task = event.task
      if (!task.notify) {
        return
      }
      const open: Open = {
        label: 'Open',
        onClick: () => navigate(`/tasks/${encodeURIComponent(task.name)}`),
      }
      if (event.type === 'task.created') {
        toast('Task started', {
          id: `task.created:${task.id}`,
          position: 'top-right',
          icon: <TaskIcon />,
          description: <StartedBody task={task} />,
          action: open,
          duration: STARTED_MS,
        })
      } else {
        showDone(task, event.summary, open, false)
      }
      playChime()
    },
    [navigate],
  )
}

// The task header's actions (ADR 0010): Accept, which confirms how many
// running sandboxes it stops, and Reject…, which asks why; both gone once
// the task is done. Then a red Delete at the far right that asks once in a
// dialog, saying the task's sandboxes and their uncommitted work go with it.
// The acting name `by` is the signed-in person.
import { useState, type FormEvent } from 'react'
import { CircleCheck, Loader2, Undo2 } from 'lucide-react'

import { useAcceptTask, useDeleteTask, useRejectTask } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { ErrorText } from '@/components/error-text'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { humanName } from '@/features/messages/identity'
import { cn } from '@/lib/utils'

import { ConfirmDelete } from './ConfirmDelete'

// The mutations of one task. error is the latest failure, verbatim.
export function useTaskActions() {
  const accept = useAcceptTask()
  const reject = useRejectTask()
  const remove = useDeleteTask()
  return { accept, reject, remove, error: accept.error ?? remove.error }
}

type TaskActionsProps = {
  task: TaskView
  actions: ReturnType<typeof useTaskActions>
  // Runs after the task is deleted.
  onDeleted?: () => void
  className?: string
}

export function TaskActions({ task, actions, onDeleted, className }: TaskActionsProps) {
  const { remove } = actions
  const count = task.sandboxes.length

  return (
    <div className={cn('flex flex-wrap items-center gap-2', className)}>
      {task.status !== 'done' && (
        <>
          <AcceptButton task={task} actions={actions} />
          <NotMet task={task} />
          <RejectButton task={task} actions={actions} />
        </>
      )}
      <ConfirmDelete
        title={`Delete task ${task.name}?`}
        pending={remove.isPending}
        onConfirm={() => remove.mutate(task.name, { onSuccess: () => onDeleted?.() })}
        className="ml-auto"
      >
        {count === 0
          ? 'The task is deleted. It has no sandbox.'
          : `Its ${count === 1 ? 'sandbox' : `${count} sandboxes`} and their uncommitted work are deleted with it.`}
      </ConfirmDelete>
    </div>
  )
}

// Dim "2 not met" after Accept: the criteria the agent's report answers
// no. Accept stays enabled (rtsdesign rule 14: the viewer decides).
function NotMet({ task }: { task: TaskView }) {
  const count = task.report?.criteria.filter((c) => c.met === 'no').length ?? 0
  if (count === 0) {
    return null
  }
  return (
    <span
      data-testid="not-met"
      title={`The agent's report says ${count === 1 ? 'one criterion is' : `${count} criteria are`} not met`}
      className="-ml-0.5 text-[11px] whitespace-nowrap text-muted-foreground"
    >
      {count} not met
    </span>
  )
}

// The sentence on the confirm button: what accepting stops.
export function acceptLabel(running: number) {
  if (running === 0) {
    return 'Accept'
  }
  return `Accept and stop ${running} running ${running === 1 ? 'sandbox' : 'sandboxes'}`
}

function AcceptButton({ task, actions }: { task: TaskView; actions: ReturnType<typeof useTaskActions> }) {
  const { accept } = actions
  const [open, setOpen] = useState(false)
  // Accepting stops every running or starting sandbox (ADR 0010 item 4).
  const running = task.sandboxes.filter((b) => b.status === 'running' || b.status === 'starting').length
  const review = task.status === 'review'
  const confirm = acceptLabel(running)

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button
          variant={review ? 'default' : 'outline'}
          size="sm"
          disabled={accept.isPending}
          title="Accept the task: sets done_at and stops its running sandboxes"
          className={cn(review && 'bg-emerald-600 text-white hover:bg-emerald-700')}
        >
          {accept.isPending ? (
            <Loader2 className="animate-spin" />
          ) : (
            <CircleCheck className={cn(!review && 'text-emerald-600')} />
          )}
          Accept
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Accept {task.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            Accepted by {humanName()}. The task, its messages and its sandboxes stay; a stopped sandbox can be started
            again.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            data-testid="accept-confirm"
            className="bg-emerald-600 text-white hover:bg-emerald-700"
            onClick={() => accept.mutate({ ref: task.name, by: humanName() })}
          >
            <CircleCheck /> {confirm}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

// Reject sends the task back to its agent; the reason is required and is
// the agent's next turn.
function RejectButton({ task, actions }: { task: TaskView; actions: ReturnType<typeof useTaskActions> }) {
  const { reject } = actions
  const [open, setOpen] = useState(false)
  const [why, setWhy] = useState('')
  const reason = why.trim()

  function openChange(next: boolean) {
    setOpen(next)
    if (!next) {
      setWhy('')
      reject.reset()
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!reason || reject.isPending) {
      return
    }
    reject.mutate({ ref: task.name, by: humanName(), why: reason }, { onSuccess: () => openChange(false) })
  }

  return (
    <Dialog open={open} onOpenChange={openChange}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" title="Send the task back to its agent with a reason">
          <Undo2 className="text-amber-600" />
          Reject…
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Reject {task.name}</DialogTitle>
            <DialogDescription>Posted in #{task.name} as the agent's next turn.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="reject-why">Why</Label>
            <Textarea
              id="reject-why"
              data-testid="reject-why"
              rows={3}
              autoFocus
              value={why}
              onChange={(e) => setWhy(e.target.value)}
              placeholder="The Stop icon is still hollow"
            />
          </div>
          <ErrorText error={reject.error} />
          <DialogFooter>
            <Button type="submit" disabled={!reason || reject.isPending}>
              {reject.isPending ? <Loader2 className="animate-spin" /> : <Undo2 />}
              {reason ? 'Reject' : 'Reject: needs a reason'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

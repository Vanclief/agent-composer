// The task header's actions: Mark done (gone once the task is done) and a
// red Delete at the far right that asks once in a dialog, saying the
// task's sandboxes and their uncommitted work go with it.
import { CircleCheck, Loader2 } from 'lucide-react'

import { useDeleteTask, useDoneTask } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { ConfirmDelete } from './ConfirmDelete'

// The mutations of one task. error is the latest failure, verbatim.
export function useTaskActions() {
  const done = useDoneTask()
  const remove = useDeleteTask()
  return { done, remove, error: done.error ?? remove.error }
}

type TaskActionsProps = {
  task: TaskView
  actions: ReturnType<typeof useTaskActions>
  // Runs after the task is deleted.
  onDeleted?: () => void
  className?: string
}

export function TaskActions({ task, actions, onDeleted, className }: TaskActionsProps) {
  const { done, remove } = actions
  const count = task.sandboxes.length

  return (
    <div className={cn('flex items-center gap-2', className)}>
      {task.status !== 'done' && (
        <Button
          variant="outline"
          size="sm"
          disabled={done.isPending}
          onClick={() => done.mutate(task.name)}
          title="Mark the task done (sets done_at)"
        >
          {done.isPending ? <Loader2 className="animate-spin" /> : <CircleCheck className="text-emerald-600" />}
          Mark done
        </Button>
      )}
      <ConfirmDelete
        title={`Delete task ${task.name}?`}
        pending={remove.isPending}
        onConfirm={() => remove.mutate(task.name, { onSuccess: () => onDeleted?.() })}
        className="ml-2"
      >
        {count === 0
          ? 'The task is deleted. It has no sandbox.'
          : `Its ${count === 1 ? 'sandbox' : `${count} sandboxes`} and their uncommitted work are deleted with it.`}
      </ConfirmDelete>
    </div>
  )
}

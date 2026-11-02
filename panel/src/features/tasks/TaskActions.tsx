// Delete for one task, shared by the list rows and the task header. Delete
// is red, sits at the far right and asks for a second click in place
// ("Delete?" for 3s) instead of a dialog.
// TODO(ADR 0008): Stop and Start moved to sandboxes; the tasks feature adds
// Done (useDoneTask) and says what deleting the sandboxes loses.
import { useEffect, useState } from 'react'
import { Loader2, Trash2 } from 'lucide-react'

import { useDeleteTask } from '@/api/hooks'
import type { TaskView } from '@/api/tasks'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const CONFIRM_MS = 3000

// The mutations of one task. error is the latest failure, verbatim.
export function useTaskActions() {
  const remove = useDeleteTask()
  return { remove, error: remove.error }
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
  const [armed, setArmed] = useState(false)

  useEffect(() => {
    if (!armed) {
      return
    }
    const timer = window.setTimeout(() => setArmed(false), CONFIRM_MS)
    return () => window.clearTimeout(timer)
  }, [armed])

  function clickDelete() {
    if (!armed) {
      setArmed(true)
      return
    }
    setArmed(false)
    remove.mutate(task.name, { onSuccess: () => onDeleted?.() })
  }

  return (
    <div className={cn('flex items-center', className)}>
      <Button
        variant={armed ? 'destructive' : 'outline'}
        size="sm"
        disabled={remove.isPending}
        onClick={clickDelete}
        title={armed ? 'Click again to delete' : 'Delete the task and its sandboxes'}
        className={cn(
          'w-[5.5rem]',
          !armed && 'border-red-500/40 text-red-600 hover:bg-red-500/10 hover:text-red-600 dark:text-red-400',
        )}
      >
        {remove.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}
        {armed ? 'Delete?' : 'Delete'}
      </Button>
    </div>
  )
}

// Stop, Start and Delete for one task, shared by the list rows and the task
// header. Delete is red, sits apart at the far right and asks for a second
// click in place ("Delete?" for 3s) instead of a dialog.
import { useEffect, useState } from 'react'
import { Loader2, Play, Square, Trash2 } from 'lucide-react'

import type { Task } from '@/api/client'
import { useDeleteTask, useStartTask, useStopTask } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const CONFIRM_MS = 3000

// The three mutations of one task. error is the latest failure, verbatim.
export function useTaskActions() {
  const stop = useStopTask()
  const start = useStartTask()
  const remove = useDeleteTask()
  return { stop, start, remove, error: stop.error ?? start.error ?? remove.error }
}

type TaskActionsProps = {
  task: Task
  actions: ReturnType<typeof useTaskActions>
  // Runs after the task is deleted.
  onDeleted?: () => void
  className?: string
}

export function TaskActions({ task, actions, onDeleted, className }: TaskActionsProps) {
  const { stop, start, remove } = actions
  const busy = stop.isPending || start.isPending || remove.isPending
  const [armed, setArmed] = useState(false)

  useEffect(() => {
    if (!armed) {
      return
    }
    const timer = window.setTimeout(() => setArmed(false), CONFIRM_MS)
    return () => window.clearTimeout(timer)
  }, [armed])

  // Only the clicked action's error stays on screen.
  function run(action: 'stop' | 'start' | 'remove') {
    for (const key of ['stop', 'start', 'remove'] as const) {
      if (key !== action) {
        actions[key].reset()
      }
    }
    if (action === 'remove') {
      remove.mutate(task.name, { onSuccess: () => onDeleted?.() })
      return
    }
    actions[action].mutate(task.name)
  }

  function clickDelete() {
    if (!armed) {
      setArmed(true)
      return
    }
    setArmed(false)
    run('remove')
  }

  return (
    <div className={cn('flex items-center', className)}>
      {/* Fixed width, so Delete stays put whichever action shows. */}
      <div className="flex w-20 justify-end">
        {task.status === 'running' && (
          <Button variant="outline" size="sm" disabled={busy} onClick={() => run('stop')}>
            {stop.isPending ? <Loader2 className="animate-spin" /> : <Square fill="currentColor" />}
            Stop
          </Button>
        )}
        {task.status === 'stopped' && (
          <Button variant="outline" size="sm" disabled={busy} onClick={() => run('start')}>
            {start.isPending ? <Loader2 className="animate-spin" /> : <Play />}
            Start
          </Button>
        )}
      </div>
      <div className="ml-3 h-5 w-px bg-zinc-200 dark:bg-zinc-800" />
      <Button
        variant={armed ? 'destructive' : 'outline'}
        size="sm"
        disabled={busy}
        onClick={clickDelete}
        title={armed ? 'Click again to delete' : 'Delete the task and its disk'}
        className={cn(
          'ml-3 w-[5.5rem]',
          !armed && 'border-red-500/40 text-red-600 hover:bg-red-500/10 hover:text-red-600 dark:text-red-400',
        )}
      >
        {remove.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}
        {armed ? 'Delete?' : 'Delete'}
      </Button>
    </div>
  )
}

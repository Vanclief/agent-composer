// What stopping or deleting a sandbox loses (ADR 0008 item 6): the dialog
// reads the sandbox fresh, with the work in its clone, and says in place
// what goes, with the numbers verbatim. The confirm button repeats it.
// Used by the Compute pages and by the task's Compute tab.
import { useState, type ReactNode } from 'react'
import { GitBranch, Loader2, type LucideIcon } from 'lucide-react'

import { useSandboxWork } from '@/api/hooks'
import type { SandboxWork } from '@/api/tasks'
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
import { cn } from '@/lib/utils'

function commits(n: number) {
  return `${n} ${n === 1 ? 'commit' : 'commits'}`
}

// The sentence for the dialog and the consequence for the confirm button.
// work null means the clone could not be checked.
export function lostWork(work: SandboxWork | null, verb: string) {
  if (!work) {
    return { sentence: 'Could not check the clone', confirm: `${verb} anyway`, loses: true }
  }
  const { dirty, unpushed } = work
  if (dirty && unpushed > 0) {
    return {
      sentence: `${unpushed} unpushed ${unpushed === 1 ? 'commit' : 'commits'} and uncommitted changes will be lost`,
      confirm: `${verb} and lose ${commits(unpushed)} and changes`,
      loses: true,
    }
  }
  if (unpushed > 0) {
    return {
      sentence: `${unpushed} unpushed ${unpushed === 1 ? 'commit' : 'commits'} will be lost`,
      confirm: `${verb} and lose ${commits(unpushed)}`,
      loses: true,
    }
  }
  if (dirty) {
    return {
      sentence: 'Uncommitted changes will be lost',
      confirm: `${verb} and lose uncommitted changes`,
      loses: true,
    }
  }
  return { sentence: 'Nothing unpushed', confirm: verb, loses: false }
}

type ConfirmSandboxProps = {
  // The sandbox name.
  sandbox: string
  // "Stop" or "Delete": the dialog title and the start of the confirm label.
  verb: string
  icon: LucideIcon
  // The button that opens the dialog.
  trigger: ReactNode
  onConfirm: () => void
  destructive?: boolean
  // More of what the action does, under what is lost.
  children?: ReactNode
}

export function ConfirmSandbox({
  sandbox,
  verb,
  icon: Icon,
  trigger,
  onConfirm,
  destructive,
  children,
}: ConfirmSandboxProps) {
  const [open, setOpen] = useState(false)
  const fresh = useSandboxWork(sandbox, open)
  const checking = fresh.isFetching || fresh.isPending
  const lost = lostWork(fresh.data?.work ?? null, verb)

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {verb} {sandbox}?
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="flex flex-col gap-2">
              <p
                data-testid="lost-work"
                className={cn(
                  'inline-flex items-center gap-1.5',
                  !checking && lost.loses && 'font-medium text-red-700 dark:text-red-400',
                )}
              >
                {checking ? <Loader2 className="size-4 animate-spin" /> : <GitBranch className="size-4" />}
                {checking ? 'Checking the clone' : lost.sentence}
              </p>
              {children && <p>{children}</p>}
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <ErrorText error={fresh.error} />
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant={destructive ? 'destructive' : 'default'}
            disabled={checking}
            onClick={onConfirm}
            className="h-auto min-h-9 whitespace-normal"
          >
            <Icon />
            {checking ? `${verb}: checking the clone` : lost.confirm}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

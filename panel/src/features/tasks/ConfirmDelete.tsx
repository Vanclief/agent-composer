// A red Delete button that asks once in an alert dialog, saying what is
// lost. Used for a task (its sandboxes go too) and for one sandbox.
import { useState, type ReactNode } from 'react'
import { Loader2, Trash2 } from 'lucide-react'

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
import { cn } from '@/lib/utils'

type ConfirmDeleteProps = {
  // The dialog title, e.g. "Delete task review-42?".
  title: string
  // What deleting loses, in place.
  children: ReactNode
  pending: boolean
  onConfirm: () => void
  // Accessible name of the trigger when it is not just "Delete".
  label?: string
  className?: string
}

export function ConfirmDelete({ title, children, pending, onConfirm, label, className }: ConfirmDeleteProps) {
  const [open, setOpen] = useState(false)
  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          aria-label={label}
          className={cn(
            'border-red-500/40 text-red-600 hover:bg-red-500/10 hover:text-red-600 dark:text-red-400',
            className,
          )}
        >
          {pending ? <Loader2 className="animate-spin" /> : <Trash2 />}
          Delete
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{children}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={onConfirm}>
            <Trash2 /> Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

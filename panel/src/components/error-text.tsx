// A failed request's message, in place and verbatim. Pass the error from a
// query or mutation; renders nothing when there is none.
import { errorMessage } from '@/api/client'
import { cn } from '@/lib/utils'

type ErrorTextProps = {
  error: unknown
  className?: string
}

export function ErrorText({ error, className }: ErrorTextProps) {
  if (!error) {
    return null
  }
  return (
    <div
      role="alert"
      className={cn(
        'rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm whitespace-pre-wrap text-red-700 dark:text-red-300',
        className,
      )}
    >
      {errorMessage(error)}
    </div>
  )
}

// The unread count on a channel row and on the Talk group header: neutral,
// pink when an unread message mentions @<me>. Nothing while all is read.
import { cn } from '@/lib/utils'

import { UNREAD_LIMIT } from './unread'

export function UnreadBadge({ count, mention, className }: { count: number; mention: boolean; className?: string }) {
  if (count === 0) {
    return null
  }
  return (
    <span
      data-testid="unread-badge"
      data-mention={mention}
      title={mention ? 'Unread, mentions you' : 'Unread'}
      className={cn(
        'ml-auto min-w-5 rounded-full px-1.5 text-center font-mono text-xs leading-5 font-semibold',
        mention ? 'bg-pink-600 text-white' : 'bg-zinc-700 text-white dark:bg-zinc-200 dark:text-zinc-900',
        className,
      )}
    >
      {count >= UNREAD_LIMIT ? `${UNREAD_LIMIT}+` : count}
    </span>
  )
}

// The outlined Follow / Following toggle (rtsdesign rule 7), on a channel's
// header and on each Browse row. Unfollowing covers every message so far,
// so only a newer mention or DM follows the channel again (follow.ts).
import { Check, Plus } from 'lucide-react'

import { useChannels } from '@/api/hooks'
import { conceptColors } from '@/components/concept-icons'
import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/utils'

import { follow, isFollowing, unfollow, useFollowMap } from './follow'
import { useMyName } from './identity'

export function FollowToggle({ channel, className }: { channel: string; className?: string }) {
  const me = useMyName()
  const map = useFollowMap()
  const channels = useChannels()
  const following = isFollowing(map, channel, me)
  const lastID = channels.data?.find((c) => c.channel === channel)?.last_id ?? 0
  const Icon = following ? Check : Plus
  const inbox = channel === `#${me}`
  return (
    <Toggle
      variant="outline"
      size="sm"
      pressed={following}
      disabled={inbox}
      onPressedChange={(on) => (on ? follow(channel, lastID) : unfollow(channel, lastID))}
      data-testid="follow-toggle"
      title={
        inbox
          ? 'Your inbox is always followed'
          : following
            ? `Unfollow ${channel}: it leaves the sidebar until it mentions you`
            : `Follow ${channel}`
      }
      className={cn('h-7 min-w-24 shrink-0 justify-start gap-1 px-2 text-xs', conceptColors.message.on, className)}
    >
      <Icon aria-hidden className="size-3.5" />
      {inbox ? 'Inbox: always' : following ? 'Following' : 'Follow'}
    </Toggle>
  )
}

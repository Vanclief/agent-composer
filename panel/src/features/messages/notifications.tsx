// Message toasts for me only: a message in my inbox (#<me>), in a DM of
// mine (@<me>/<task>) or one that mentions @<me>, in any channel, followed
// or not, toasts top-right with the chime, showing sender, channel
// and the first 160 characters, with Open to /messages/<channel>. Every
// other message only bumps its badge, and only when its channel is
// followed (follow.ts). My own messages never notify, and
// neither does a message in the channel I am looking at.
import { useCallback, useEffect, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { toast } from 'sonner'

import type { LiveEvent } from '@/api/events'
import type { Message } from '@/api/msg'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
import { cn } from '@/lib/utils'
import { playChime } from '@/shell/sound'

import { useMyName } from './identity'
import { channelPath } from './follow'
import { isDMOf, mentions } from './unread'

export const MESSAGE_PREVIEW = 160
const MESSAGE_MS = 10_000

function MessageBody({ message }: { message: Message }) {
  const long = message.body.length > MESSAGE_PREVIEW
  const count = message.images?.length ?? 0
  const pictures = count > 0 ? `[${count} ${count === 1 ? 'image' : 'images'}]` : ''
  const text = long ? message.body.slice(0, MESSAGE_PREVIEW).trimEnd() + '…' : message.body
  const shown = [text.trim(), pictures].filter(Boolean).join(' ')
  return (
    <div
      data-testid="toast-message"
      data-channel={message.channel}
      data-id={message.id}
      className="flex min-w-0 flex-col gap-1.5"
    >
      <span className="flex flex-wrap items-center gap-1">
        <IconChip
          icon={message.task ? conceptIcons.agent : conceptIcons.human}
          concept={message.task ? 'agent' : 'human'}
          title={message.task ? `Agent in task ${message.task}` : 'Sender'}
        >
          @{message.from}
        </IconChip>
        <IconChip icon={conceptIcons.channel} concept="channel" title="Channel">
          {message.channel}
        </IconChip>
      </span>
      <p data-testid="toast-message-body" className="text-xs whitespace-pre-wrap text-muted-foreground">
        {shown}
      </p>
    </div>
  )
}

// The handler to pass to useLiveEvents, next to the task notifications.
export function useMessageNotifications() {
  const navigate = useNavigate()
  const me = useMyName()
  const { pathname } = useLocation()
  const open = useRef(pathname)
  useEffect(() => {
    open.current = pathname
  }, [pathname])

  return useCallback(
    (event: LiveEvent) => {
      if (event.type !== 'message.posted') {
        return
      }
      const message = event.message
      if (message.from === me) {
        return
      }
      const inbox = message.channel === `#${me}`
      const dm = isDMOf(message.channel, me)
      if (!inbox && !dm && !mentions(message.body, me)) {
        return
      }
      const path = channelPath(message.channel)
      if (open.current === path || decodeURIComponent(open.current) === decodeURIComponent(path)) {
        return
      }
      const Icon = inbox ? conceptIcons.inbox : dm ? conceptIcons.human : conceptIcons.message
      toast(inbox ? 'Inbox' : dm ? 'Direct' : 'Mentioned you', {
        id: `message:${message.channel}:${message.id}`,
        position: 'top-right',
        icon: <Icon aria-hidden className={cn('size-4', conceptColors.message.text)} />,
        description: <MessageBody message={message} />,
        action: { label: 'Open', onClick: () => navigate(path) },
        duration: MESSAGE_MS,
      })
      playChime()
    },
    [me, navigate],
  )
}

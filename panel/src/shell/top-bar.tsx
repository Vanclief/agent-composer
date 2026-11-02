// The top bar's right side: the live feed's state, the sound toggle and
// who you are.
import { Volume2, VolumeX } from 'lucide-react'

import { useLiveConnected } from '@/api/live'
import { Avatar } from '@/components/avatar'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useMyName } from '@/features/messages/identity'
import { cn } from '@/lib/utils'

import { setSoundOn, useSoundOn } from './sound'

// Green "live" while GET /api/events is open; amber "reconnecting" while
// the browser retries it and pages fall back to polling every 5s.
export function LiveDot() {
  const connected = useLiveConnected()
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          data-testid="live-status"
          data-state={connected ? 'connected' : 'reconnecting'}
          className="flex items-center gap-1.5 rounded-md px-2 py-1 font-mono text-xs text-muted-foreground"
        >
          <span
            aria-hidden
            className={cn('size-2 rounded-full', connected ? 'bg-emerald-500' : 'animate-pulse bg-amber-500')}
          />
          {connected ? 'live' : 'live: reconnecting'}
        </span>
      </TooltipTrigger>
      <TooltipContent>
        {connected ? 'Pages update as things change' : 'Event stream lost: retrying, pages poll every 5s'}
      </TooltipContent>
    </Tooltip>
  )
}

// Mutes the notification chime; remembered in localStorage.
export function SoundToggle() {
  const on = useSoundOn()
  const label = on ? 'Mute sounds' : 'Unmute sounds'
  return (
    <Button
      variant="ghost"
      size="icon"
      data-testid="sound-toggle"
      aria-label={label}
      aria-pressed={on}
      title={label}
      onClick={() => setSoundOn(!on)}
    >
      {on ? <Volume2 /> : <VolumeX />}
    </Button>
  )
}

// Who the panel posts as (useMyName, follows "Join as…"): your avatar and
// name.
export function MeChip() {
  const me = useMyName()
  return (
    <span
      data-testid="me"
      title={`Signed in as @${me}`}
      className="flex items-center gap-1.5 rounded-md px-2 py-1 font-mono text-xs text-muted-foreground"
    >
      <Avatar name={me} size="sm" human />@{me}
    </span>
  )
}

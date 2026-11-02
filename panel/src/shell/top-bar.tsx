// The top bar's right side: the live feed's state and the sound toggle.
import { Volume2, VolumeX } from 'lucide-react'

import { useLiveConnected } from '@/api/live'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
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

// The top bar's right side: the live feed's state, the sound toggle and
// who you are.
import { useState, type FormEvent } from 'react'
import { Volume2, VolumeX } from 'lucide-react'
import { Popover } from 'radix-ui'

import { useLiveConnected } from '@/api/live'
import { Avatar } from '@/components/avatar'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { storedName, useDefaults, useIdentity, useMyName } from '@/features/messages/identity'
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
// name, a button opening the name popover. An amber ring says the name
// stored here is not the server's owner (GET /api/defaults), the "@test"
// situation, one click from fixed.
export function MeChip() {
  const me = useMyName()
  const defaults = useDefaults()
  const stored = storedName()
  const mismatch = Boolean(stored && defaults.owner && stored !== defaults.owner)
  const [open, setOpen] = useState(false)
  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          data-testid="me"
          data-mismatch={mismatch || undefined}
          title={mismatch ? `You are @${me} here, the server's owner is @${defaults.owner}` : `Signed in as @${me}`}
          className={cn(
            'flex items-center gap-1.5 rounded-md px-2 py-1 font-mono text-xs text-muted-foreground hover:bg-zinc-500/10 hover:text-foreground focus-visible:ring-1 focus-visible:ring-violet-500/70 focus-visible:outline-none',
            mismatch && 'ring-1 ring-amber-500/80',
          )}
        >
          <Avatar name={me} size="sm" human />@{me}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="end"
          sideOffset={6}
          aria-label="Who you are"
          className="z-50 w-72 rounded-lg border border-border bg-popover p-3 text-sm text-popover-foreground shadow-md"
        >
          <MePopover me={me} owner={defaults.owner} />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

// The current name, the server's owner as a one-click choice when it
// differs, and another name. Renaming leaves the old name and joins the new
// one (useIdentity's rename). It stays open, so the new name and any join
// error (409 while the name is held) show in place.
function MePopover({ me, owner }: { me: string; owner: string }) {
  const identity = useIdentity({ autoJoin: false })
  const [other, setOther] = useState('')
  const next = other.trim().replace(/^@/, '')

  function signIn(name: string) {
    identity.rename(name)
    setOther('')
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (next && next !== me) {
      signIn(next)
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <Avatar name={me} size="md" human />
        <div className="flex min-w-0 flex-col">
          <span className="truncate font-mono">@{me}</span>
          <span className="text-xs text-muted-foreground">
            {owner && owner === me ? "the server's owner" : 'signed in here'}
          </span>
        </div>
      </div>
      {owner && owner !== me && (
        <Button
          size="sm"
          variant="outline"
          data-testid="sign-in-owner"
          className="justify-start border-amber-500/60"
          onClick={() => signIn(owner)}
        >
          <Avatar name={owner} size="xs" human />
          Sign in as @{owner}
        </Button>
      )}
      <form onSubmit={submit} className="flex flex-col gap-2">
        <Input
          aria-label="Another name"
          placeholder="another name"
          value={other}
          onChange={(e) => setOther(e.target.value)}
          className="h-8 font-mono text-xs"
        />
        <Button size="sm" type="submit" variant="secondary" disabled={!next || next === me}>
          {!next ? 'Sign in: needs a name' : next === me ? 'Sign in: same name' : 'Sign in'}
        </Button>
      </form>
      <ErrorText error={identity.error} />
    </div>
  )
}

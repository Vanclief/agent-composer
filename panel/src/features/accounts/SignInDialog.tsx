// Sign in: agc starts a login for the account (POST …/signin), the user
// signs in on the provider's page in a new tab and pastes the code it shows
// back here (POST …/signin/complete). The login feeds usage and identity
// only; the account's token keeps feeding sandboxes.
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ExternalLink, Loader2, LogIn, RotateCcw } from 'lucide-react'

import type { AccountView } from '@/api/accounts'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { useNow } from './format'
import { useBeginSignin, useCompleteSignin } from './signin'

type SignInDialogProps = {
  account: AccountView
  onOpenChange: (open: boolean) => void
}

// Mounted only while open: every mount starts a fresh sign-in.
export function SignInDialog({ account, onOpenChange }: SignInDialogProps) {
  const begin = useBeginSignin()
  const complete = useCompleteSignin()
  const now = useNow(1000)
  // What the user pasted: sent once, cleared on success or failure, never
  // rendered again.
  const [code, setCode] = useState('')
  const started = useRef(false)

  function start() {
    complete.reset()
    begin.mutate(account.name)
  }

  useEffect(() => {
    if (started.current) {
      return
    }
    started.current = true
    begin.mutate(account.name)
  }, [begin, account.name])

  const pending = begin.data
  const expired = Boolean(pending && new Date(pending.expires_at).getTime() <= now)
  const who = account.email || account.name

  const openWhy = begin.isPending ? 'preparing' : expired ? 'expired' : !pending ? 'not started' : ''
  const finishWhy = !pending
    ? 'start the sign-in first'
    : expired
      ? 'the sign-in page expired'
      : !code.trim()
        ? 'paste the code'
        : ''

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (finishWhy || complete.isPending) {
      return
    }
    const pasted = code.trim()
    // Out of state before the request goes, so it is never rendered again.
    setCode('')
    try {
      await complete.mutateAsync({ name: account.name, code: pasted })
      onOpenChange(false)
    } catch {
      // complete.error holds it, shown under the field.
    }
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" data-testid="sign-in-dialog">
        <form onSubmit={(e) => void submit(e)} className="flex min-w-0 flex-col gap-4" autoComplete="off">
          <DialogHeader>
            <DialogTitle>
              Sign in: <span className="font-mono normal-case">{who}</span>
            </DialogTitle>
            <DialogDescription>
              Sign in with this account in the browser, then paste the code it shows.
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-1.5">
            {pending && !openWhy ? (
              <Button asChild variant="outline" data-act="open-sign-in">
                <a href={pending.url} target="_blank" rel="noopener noreferrer">
                  <ExternalLink /> Open the sign-in page
                </a>
              </Button>
            ) : (
              <Button type="button" variant="outline" data-act="open-sign-in" disabled>
                {begin.isPending ? <Loader2 className="animate-spin" /> : <ExternalLink />}
                Open the sign-in page: {openWhy}
              </Button>
            )}
            {expired && (
              <div data-testid="sign-in-expired" className="flex items-center gap-2 text-xs text-muted-foreground">
                The sign-in page expired.
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  data-act="start-again"
                  className="ml-auto h-6 px-2 text-xs"
                  onClick={start}
                >
                  <RotateCcw /> Start again
                </Button>
              </div>
            )}
            <ErrorText error={begin.error} className="text-xs" />
            {begin.isError && <StartAgain onClick={start} />}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sign-in-code">Paste the code shown after signing in</Label>
            <Input
              id="sign-in-code"
              type="password"
              autoComplete="off"
              spellCheck={false}
              className="font-mono"
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
            <ErrorText error={complete.error} className="text-xs" />
            {complete.isError && !expired && <StartAgain onClick={start} />}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" data-act="finish-sign-in" disabled={Boolean(finishWhy) || complete.isPending}>
              {complete.isPending ? <Loader2 className="animate-spin" /> : <LogIn />}
              {finishWhy ? `Finish: ${finishWhy}` : 'Finish'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function StartAgain({ onClick }: { onClick: () => void }) {
  return (
    <Button type="button" size="sm" variant="outline" data-act="start-again" className="self-start" onClick={onClick}>
      <RotateCcw /> Start again
    </Button>
  )
}

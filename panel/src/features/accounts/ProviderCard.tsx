// One provider's card, laid out like Orca's Claude card: the mark and name
// with "Updated …", a bar per usage window of the default account, the
// active account in a dropdown ("Switch to": System default, then each
// account), and Add account / Manage Accounts… at the bottom.
import { useState } from 'react'
import { Check, ChevronDown, LogIn, Plus, Settings2 } from 'lucide-react'

import type { AccountView, ProviderView, UsageView } from '@/api/accounts'
import { useClearDefaultAccount, useMakeDefaultAccount } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

import { AddAccountDialog, ManageAccountsDialog, type AddPrefill } from './AccountDialogs'
import { accountLabel, ago, KIND_LABELS, percent, providerName, tokenRejected, until, useNow } from './format'

const color = conceptColors.account

export function ProviderCard({ provider, accounts }: { provider: ProviderView; accounts: AccountView[] }) {
  const now = useNow()
  const display = providerName(provider.id)
  const active = accounts.find((a) => a.default)
  // The Add dialog is mounted only while open, so its fields start from
  // the prefill every time.
  const [adding, setAdding] = useState<{ prefill?: AddPrefill } | null>(null)
  const [managing, setManaging] = useState(false)

  function openAdd(prefill?: AddPrefill) {
    setAdding({ prefill })
  }

  // Sign in for a rejected token. The device-flow sign-in is a follow-up;
  // until then this opens Add pre-filled, and pasting a new token replaces
  // the account (DELETE, then POST).
  function signIn(account: AccountView) {
    openAdd({ name: account.name, kind: account.kind, email: account.email, replace: account })
  }

  return (
    <section
      aria-label={`${display} accounts`}
      data-card={provider.id}
      className={cn(
        'flex flex-col gap-3 rounded-xl border border-border bg-white p-4 shadow-sm dark:bg-zinc-900/60',
        color.edge,
      )}
    >
      <header className="flex items-center gap-2">
        <ProviderMark name={display} />
        <h3 className="text-sm font-semibold">{display}</h3>
        <span className="font-mono text-xs text-muted-foreground" title="Harnesses this provider serves">
          {provider.harnesses.join(', ')}
        </span>
        {active?.usage && (
          <span
            data-testid="usage-updated"
            className="ml-auto text-xs text-muted-foreground"
            title={new Date(active.usage.updated_at).toLocaleString()}
          >
            Updated {ago(active.usage.updated_at, now)}
          </span>
        )}
      </header>

      {accounts.length === 0 ? (
        <button
          type="button"
          data-testid="accounts-empty"
          onClick={() => openAdd()}
          className="flex min-h-16 items-center justify-center gap-1.5 rounded-md border border-dashed border-zinc-300 px-3 text-sm text-zinc-500 transition-colors hover:border-zinc-400 hover:text-zinc-900 dark:border-zinc-700 dark:hover:border-zinc-500 dark:hover:text-zinc-100"
        >
          <Plus className="size-4" />
          No {display} account yet. Add one.
        </button>
      ) : (
        <>
          <Usage account={active} now={now} onSignIn={signIn} />
          <Switcher display={display} accounts={accounts} active={active} onSignIn={signIn} />
        </>
      )}

      <footer className="flex items-center gap-2 border-t border-border pt-3">
        <Button size="sm" variant="outline" data-act="add-account" onClick={() => openAdd()}>
          <Plus /> Add account
        </Button>
        <Button
          size="sm"
          variant="ghost"
          data-act="manage-accounts"
          className="ml-auto"
          onClick={() => setManaging(true)}
        >
          <Settings2 /> Manage Accounts…
        </Button>
      </footer>

      {adding && (
        <AddAccountDialog
          provider={provider}
          prefill={adding.prefill}
          open
          onOpenChange={(open) => !open && setAdding(null)}
        />
      )}
      <ManageAccountsDialog
        provider={provider}
        accounts={accounts}
        open={managing}
        onOpenChange={setManaging}
        onAdd={() => openAdd()}
      />
    </section>
  )
}

// The provider's initial on a tinted tile; no brand artwork.
function ProviderMark({ name }: { name: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        'flex size-6 items-center justify-center rounded-md border text-xs font-semibold',
        color.bg,
        color.border,
        color.text,
      )}
    >
      {name.slice(0, 1)}
    </span>
  )
}

function Usage({
  account,
  now,
  onSignIn,
}: {
  account: AccountView | undefined
  now: number
  onSignIn: (account: AccountView) => void
}) {
  if (!account) {
    return (
      <p data-testid="usage-none" className="text-xs text-muted-foreground">
        No default account: sandboxes get only what their spec names
      </p>
    )
  }
  if (!account.usage) {
    return (
      <p data-testid="usage-none" className="text-xs text-muted-foreground">
        Usage not available for this account
      </p>
    )
  }
  return <UsageBars usage={account.usage} now={now} account={account} onSignIn={onSignIn} />
}

function UsageBars({
  usage,
  now,
  account,
  onSignIn,
}: {
  usage: UsageView
  now: number
  account: AccountView
  onSignIn: (account: AccountView) => void
}) {
  const Icon = conceptIcons.usage
  return (
    <div className="flex flex-col gap-3">
      {usage.error && (
        <div className="flex flex-col gap-1.5">
          <ErrorText error={new Error(usage.error)} className="text-xs" />
          {tokenRejected(account) && <SignInLine account={account} onSignIn={onSignIn} />}
        </div>
      )}
      {usage.windows.map((w) => (
        <div key={w.id} data-window={w.id} className="flex flex-col gap-1">
          <div className="flex items-center gap-1.5 text-sm">
            <Icon aria-hidden className={cn('size-3.5', conceptColors.usage.text)} />
            <span data-testid="window-label">{w.label}</span>
          </div>
          <div
            role="meter"
            aria-label={`${w.label} used`}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(w.used * 100)}
            className="h-1.5 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
          >
            <div className="h-full rounded-full bg-yellow-500/80" style={{ width: percent(w.used) }} />
          </div>
          <div className="flex justify-between text-xs text-muted-foreground tabular-nums">
            <span data-testid="window-used">{percent(w.used)} used</span>
            <span data-testid="window-resets" title={new Date(w.resets_at).toLocaleString()}>
              Resets in {until(w.resets_at, now)}
            </span>
          </div>
        </div>
      ))}
    </div>
  )
}

function SignInLine({ account, onSignIn }: { account: AccountView; onSignIn: (account: AccountView) => void }) {
  return (
    <div data-testid="sign-in-line" className="flex items-center gap-2 text-xs text-muted-foreground">
      Sign in again to use this account
      <Button size="sm" variant="outline" className="ml-auto h-6 px-2 text-xs" onClick={() => onSignIn(account)}>
        <LogIn /> Sign in
      </Button>
    </div>
  )
}

function Switcher({
  display,
  accounts,
  active,
  onSignIn,
}: {
  display: string
  accounts: AccountView[]
  active: AccountView | undefined
  onSignIn: (account: AccountView) => void
}) {
  const makeDefault = useMakeDefaultAccount()
  const clearDefault = useClearDefaultAccount()
  const pending = makeDefault.isPending || clearDefault.isPending

  function pick(account: AccountView) {
    clearDefault.reset()
    makeDefault.mutate(account.name)
  }

  function system() {
    if (!active) {
      return
    }
    makeDefault.reset()
    clearDefault.mutate(active.name)
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-xs text-muted-foreground">{display} Account</span>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            data-testid="active-account"
            disabled={pending}
            className="w-full justify-between font-normal"
          >
            <span data-testid="active-label" className="truncate">
              {accountLabel(active)}
            </span>
            <ChevronDown className="opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-(--radix-dropdown-menu-trigger-width) min-w-64">
          <DropdownMenuLabel className="text-[11px] tracking-wide text-muted-foreground uppercase">
            Switch to
          </DropdownMenuLabel>
          <DropdownMenuItem data-switch="system" disabled={!active} onSelect={system}>
            System default
            {!active && <ActiveMark />}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          {accounts.map((a) => {
            const isActive = a.default
            const rejected = tokenRejected(a)
            return (
              <DropdownMenuItem
                key={a.name}
                data-switch={a.name}
                disabled={isActive}
                onSelect={() => (rejected ? onSignIn(a) : pick(a))}
                className="flex-col items-start gap-0.5"
              >
                <span className="flex w-full items-center gap-2">
                  <span className={cn('truncate', rejected && 'text-muted-foreground')}>{a.email || a.name}</span>
                  <span className="text-xs text-muted-foreground">{KIND_LABELS[a.kind]}</span>
                  {isActive && <ActiveMark />}
                  {rejected && !isActive && (
                    <span className="ml-auto flex items-center gap-1 text-xs">
                      <LogIn className="size-3.5" /> Sign in
                    </span>
                  )}
                </span>
                {rejected && (
                  <span data-testid="sign-in-hint" className="text-xs text-muted-foreground">
                    Sign in again to use this account
                  </span>
                )}
              </DropdownMenuItem>
            )
          })}
        </DropdownMenuContent>
      </DropdownMenu>
      <ErrorText error={makeDefault.error ?? clearDefault.error} className="text-xs" />
    </div>
  )
}

function ActiveMark() {
  return (
    <span className="ml-auto flex items-center gap-1 text-xs text-muted-foreground">
      <Check className="size-3.5" /> Active
    </span>
  )
}

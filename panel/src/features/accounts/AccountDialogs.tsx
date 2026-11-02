// Add account (also Sign in again, which replaces a rejected token) and
// Manage Accounts… (Remove with a confirm, Make default) for one provider.
import { useState, type FormEvent } from 'react'
import { Loader2, Plus, Star, Trash2 } from 'lucide-react'

import type { AccountKind, AccountView, ProviderView } from '@/api/accounts'
import { useClearDefaultAccount, useCreateAccount, useMakeDefaultAccount, useRemoveAccount } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { EmptySlot } from '@/components/empty-slot'
import { ErrorText } from '@/components/error-text'
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import { ACCOUNT_NAME_RE, KIND_LABELS, kindHint, providerName } from './format'

const chipBase =
  'h-7 border border-border bg-transparent px-2 text-xs font-normal text-muted-foreground shadow-none hover:bg-accent hover:text-accent-foreground'

// The kinds a provider offers: oauth only where it has an oauth variable.
function kindsOf(provider: ProviderView): AccountKind[] {
  return provider.variables.oauth ? ['oauth', 'api_key'] : ['api_key']
}

// What the dialog starts with. replace names the account whose token a
// new one replaces (Sign in again): it is deleted, then created again.
export type AddPrefill = { name: string; kind: AccountKind; email: string; replace: AccountView }

type AddAccountDialogProps = {
  provider: ProviderView
  open: boolean
  onOpenChange: (open: boolean) => void
  prefill?: AddPrefill
}

// Mounted only while open, so the fields start from prefill.
export function AddAccountDialog({ provider, open, onOpenChange, prefill }: AddAccountDialogProps) {
  const kinds = kindsOf(provider)
  const create = useCreateAccount()
  const remove = useRemoveAccount()
  const makeDefault = useMakeDefaultAccount()
  const [name, setName] = useState(prefill?.name ?? '')
  const [kind, setKind] = useState<AccountKind>(prefill?.kind ?? kinds[0])
  // The secret value: sent once, cleared on success or failure, never shown.
  const [value, setValue] = useState('')
  const [email, setEmail] = useState(prefill?.email ?? '')
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const replace = prefill?.replace
  const display = providerName(provider.id)

  const trimmed = name.trim()
  const why = !trimmed
    ? 'needs a name'
    : !ACCOUNT_NAME_RE.test(trimmed)
      ? 'name is a-z, 0-9 and -, up to 32'
      : !value.trim()
        ? `needs the ${KIND_LABELS[kind].toLowerCase()}`
        : ''
  const verb = replace ? 'Replace token' : 'Add'

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (why || pending) {
      return
    }
    const request = { name: trimmed, provider: provider.id, kind, value, email: email.trim() || undefined }
    // Out of state before the request goes, so it is never rendered again.
    setValue('')
    setError(null)
    setPending(true)
    try {
      // Sign in again: the old row goes first (DELETE then POST), then the
      // default it held comes back.
      if (replace) {
        await remove.mutateAsync(replace.name)
      }
      const account = await create.mutateAsync(request)
      if (replace?.default) {
        await makeDefault.mutateAsync(account.name)
      }
      onOpenChange(false)
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="add-account-dialog">
        <form onSubmit={(e) => void submit(e)} className="flex flex-col gap-4" autoComplete="off">
          <DialogHeader>
            <DialogTitle>{replace ? `Sign in again: ${replace.name}` : `Add ${display} account`}</DialogTitle>
            <DialogDescription>
              {replace
                ? 'The new token replaces the rejected one: the account is removed, then added again.'
                : 'Kept as a line of secrets.env; the value is sent once and never shown again.'}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="account-name">Name</Label>
            <Input
              id="account-name"
              className="font-mono"
              autoFocus={!replace}
              readOnly={Boolean(replace)}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="franco-max"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label id="account-kind-label">Kind</Label>
            <ToggleGroup
              type="single"
              spacing={1.5}
              aria-labelledby="account-kind-label"
              value={kind}
              onValueChange={(v) => v && !replace && setKind(v as AccountKind)}
            >
              {kinds.map((k) => (
                <ToggleGroupItem
                  key={k}
                  value={k}
                  data-kind={k}
                  size="sm"
                  disabled={Boolean(replace) && k !== kind}
                  className={chipBase}
                >
                  {KIND_LABELS[k]}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="account-value">{KIND_LABELS[kind]}</Label>
            <Input
              id="account-value"
              type="password"
              autoComplete="new-password"
              autoFocus={Boolean(replace)}
              spellCheck={false}
              className="font-mono"
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
            <p data-testid="value-hint" className="text-xs text-muted-foreground">
              {kindHint(provider.id, kind)}
            </p>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="account-email">Email (optional)</Label>
            <Input
              id="account-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
            />
          </div>
          <ErrorText error={error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={Boolean(why) || pending}>
              {pending ? <Loader2 className="animate-spin" /> : <Plus />}
              {why ? `${verb}: ${why}` : verb}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

type ManageAccountsDialogProps = {
  provider: ProviderView
  accounts: AccountView[]
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdd: () => void
}

// The provider's accounts as rows, each with Make default and Remove.
export function ManageAccountsDialog({ provider, accounts, open, onOpenChange, onAdd }: ManageAccountsDialogProps) {
  const display = providerName(provider.id)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" data-testid="manage-accounts-dialog">
        <DialogHeader>
          <DialogTitle>{display} accounts</DialogTitle>
          <DialogDescription>The default is what a sandbox gets when its spec names no credential.</DialogDescription>
        </DialogHeader>
        {accounts.length === 0 ? (
          <EmptySlot reason={`No ${display} account yet`} />
        ) : (
          <ul
            aria-label={`${display} accounts`}
            className="-mx-1 flex max-h-[50vh] flex-col divide-y divide-border overflow-y-auto px-1"
          >
            {accounts.map((a) => (
              <ManageRow key={a.name} account={a} />
            ))}
          </ul>
        )}
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              onOpenChange(false)
              onAdd()
            }}
          >
            <Plus /> Add account
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ManageRow({ account }: { account: AccountView }) {
  const makeDefault = useMakeDefaultAccount()
  const clearDefault = useClearDefaultAccount()
  const remove = useRemoveAccount()
  const Icon = conceptIcons.account
  return (
    <li data-account={account.name} className="flex flex-col gap-1.5 py-2">
      <div className="flex items-center gap-2">
        <Icon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        <span className="shrink-0 font-mono text-sm whitespace-nowrap">{account.name}</span>
        <span className="min-w-0 truncate font-mono text-xs text-muted-foreground">
          {[account.email, KIND_LABELS[account.kind]].filter(Boolean).join(' · ')}
        </span>
        <div className="ml-auto flex shrink-0 items-center gap-1.5">
          {account.default ? (
            <Button
              size="sm"
              variant="ghost"
              data-act="clear-default"
              disabled={clearDefault.isPending}
              onClick={() => clearDefault.mutate(account.name)}
              title="Back to System default"
            >
              <Star className="fill-current" /> Default
            </Button>
          ) : (
            <Button
              size="sm"
              variant="ghost"
              data-act="make-default"
              disabled={makeDefault.isPending}
              onClick={() => makeDefault.mutate(account.name)}
            >
              {makeDefault.isPending ? <Loader2 className="animate-spin" /> : <Star />} Make default
            </Button>
          )}
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                size="sm"
                variant="ghost"
                data-act="remove"
                disabled={remove.isPending}
                className="text-red-600 hover:text-red-600 dark:text-red-400"
              >
                {remove.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />} Remove
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Remove account {account.name}?</AlertDialogTitle>
                <AlertDialogDescription>Deletes its row and its line in secrets.env.</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={() => remove.mutate(account.name)}>
                  <Trash2 /> Remove
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>
      <ErrorText error={remove.error ?? makeDefault.error ?? clearDefault.error} className="text-xs" />
    </li>
  )
}

// Words and numbers for the Accounts page: percentages, reset times and
// how long ago usage was read.
import { useEffect, useState } from 'react'

import type { AccountKind, AccountProvider, AccountView } from '@/api/accounts'

// The contract's slug.
export const ACCOUNT_NAME_RE = /^[a-z0-9-]{1,32}$/

// How a provider is called on its card.
export const PROVIDER_NAMES: Record<AccountProvider, string> = {
  anthropic: 'Claude',
  openai: 'Codex',
}

export function providerName(id: string) {
  return PROVIDER_NAMES[id as AccountProvider] ?? id
}

export const KIND_LABELS: Record<AccountKind, string> = {
  oauth: 'Subscription token',
  api_key: 'API key',
}

// Where the value comes from, one line.
export function kindHint(provider: string, kind: AccountKind) {
  if (kind === 'oauth') {
    return '`claude setup-token` on a machine signed in to the account'
  }
  if (provider === 'openai') {
    return 'platform.openai.com → API keys'
  }
  return 'console.anthropic.com → API keys'
}

// The default's email, else its name; "System default" when none.
export function accountLabel(account: AccountView | undefined) {
  if (!account) {
    return 'System default'
  }
  return account.email || account.name
}

// A usage.error starting with this means the token no longer works.
export function tokenRejected(account: AccountView) {
  return Boolean(account.usage?.error.startsWith('token rejected'))
}

// 0.164 → "16%".
export function percent(used: number) {
  return `${Math.round(Math.min(Math.max(used, 0), 1) * 100)}%`
}

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

// "4h 11m" under a day, "1d 19h" above; "0m" once passed.
export function until(at: string, now: number) {
  const ms = Math.max(new Date(at).getTime() - now, 0)
  if (ms >= DAY) {
    return `${Math.floor(ms / DAY)}d ${Math.floor((ms % DAY) / HOUR)}h`
  }
  const hours = Math.floor(ms / HOUR)
  const minutes = Math.floor((ms % HOUR) / MINUTE)
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`
}

// "just now", "3 min ago", "2h ago", "4d ago".
export function ago(at: string, now: number) {
  const ms = Math.max(now - new Date(at).getTime(), 0)
  if (ms < MINUTE) {
    return 'just now'
  }
  if (ms < HOUR) {
    return `${Math.floor(ms / MINUTE)} min ago`
  }
  if (ms < DAY) {
    return `${Math.floor(ms / HOUR)}h ago`
  }
  return `${Math.floor(ms / DAY)}d ago`
}

// The current time, refreshed every 30s so relative times move.
export function useNow(intervalMs = 30_000) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}

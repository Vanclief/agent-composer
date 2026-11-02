// Accounts: a credential for one provider, kept as a line of secrets.env
// and described by a row; at most one per provider is the default a
// sandbox gets when its spec names no variable the harness reads.
// Hand-typed from the accounts contract (accounts-model, accounts-usage)
// until docs/openapi.yaml has it; then these can become aliases of
// components['schemas']. Values are sent once on create and never read
// back by any route.
import { apiFetch } from './client'

export type AccountProvider = 'anthropic' | 'openai'

export type AccountKind = 'oauth' | 'api_key'

export type UsageWindow = {
  // session, weekly or a model slug.
  id: string
  label: string
  // 0..1
  used: number
  resets_at: string
}

export type UsageView = {
  updated_at: string
  windows: UsageWindow[]
  // "" when the fetch worked, else why (verbatim).
  error: string
}

export type AccountView = {
  // [a-z0-9-]{1,32}
  name: string
  provider: AccountProvider
  kind: AccountKind
  // The NAME in secrets.env holding the value.
  secret: string
  email: string
  default: boolean
  created_at: string
  // Filled for anthropic oauth accounts by the usage fetcher; else null.
  usage: UsageView | null
}

export type ProviderView = {
  id: AccountProvider
  harnesses: string[]
  variables: { oauth?: string; api_key: string }
}

export type AccountsResponse = { accounts: AccountView[]; providers: ProviderView[] }

export type CreateAccountRequest = {
  name: string
  provider: AccountProvider
  kind: AccountKind
  value: string
  email?: string
}

const enc = encodeURIComponent

export const accounts = {
  list(signal?: AbortSignal) {
    return apiFetch<AccountsResponse>('GET', '/accounts', undefined, signal)
  },
  // 201; the value goes to secrets.env and never comes back.
  create(request: CreateAccountRequest) {
    return apiFetch<AccountView>('POST', '/accounts', request)
  },
  // The provider's previous default loses the flag.
  makeDefault(name: string) {
    return apiFetch<AccountView>('POST', `/accounts/${enc(name)}/default`)
  },
  // Back to no default for the provider.
  clearDefault(name: string) {
    return apiFetch<AccountView>('DELETE', `/accounts/${enc(name)}/default`)
  },
  // 204; refused with ECONFLICT while a running sandbox's spec names it.
  remove(name: string) {
    return apiFetch<void>('DELETE', `/accounts/${enc(name)}`)
  },
}

// Account sign-in (contract shared with accounts-signin): agc runs the
// same login `claude auth login` does, used only for usage and identity.
//
// Hand-typed here, in this one file, until accounts-signin lands the routes
// in docs/openapi.yaml and `make types` regenerates src/api/schema.d.ts;
// then these become aliases of components['schemas'] and AccountView in
// src/api/accounts.ts gains signed_in_at.
//
//   POST   /api/accounts/{name}/signin            → { url, expires_at }
//   POST   /api/accounts/{name}/signin/complete   { code } → AccountView
//   DELETE /api/accounts/{name}/signin            → AccountView
import { useMutation, useQueryClient } from '@tanstack/react-query'

import type { AccountView } from '@/api/accounts'
import { apiFetch } from '@/api/client'
import { queryKeys } from '@/api/hooks'

// AccountView plus the contract's signed_in_at (RFC3339, null when the
// account has no login).
export type SignedAccountView = AccountView & { signed_in_at?: string | null }

export type SigninStart = {
  // The provider's authorize page; opened in a new tab.
  url: string
  // RFC3339; after it the pending sign-in is gone and must start again.
  expires_at: string
}

// What the user pasted, `code#state` or a bare code. Never kept.
export type SigninCompleteRequest = { code: string }

const enc = encodeURIComponent

export const signin = {
  begin(name: string) {
    return apiFetch<SigninStart>('POST', `/accounts/${enc(name)}/signin`)
  },
  complete(name: string, request: SigninCompleteRequest) {
    return apiFetch<SignedAccountView>('POST', `/accounts/${enc(name)}/signin/complete`, request)
  },
  signOut(name: string) {
    return apiFetch<SignedAccountView>('DELETE', `/accounts/${enc(name)}/signin`)
  },
}

function useSigninMutation<TVariables, TData>(mutationFn: (variables: TVariables) => Promise<TData>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.accounts }),
  })
}

export function useBeginSignin() {
  return useMutation({ mutationFn: (name: string) => signin.begin(name) })
}

export function useCompleteSignin() {
  return useSigninMutation(({ name, code }: { name: string; code: string }) => signin.complete(name, { code }))
}

export function useSignOut() {
  return useSigninMutation((name: string) => signin.signOut(name))
}

// The two usage errors a sign-in fixes, by the prefix the poller writes.
export type SigninNeed = 'usage' | 'expired'

export function signinNeed(account: AccountView): SigninNeed | null {
  const error = account.usage?.error ?? ''
  if (error.startsWith('inference-only token')) {
    return 'usage'
  }
  if (error.startsWith('sign-in expired')) {
    return 'expired'
  }
  return null
}

export const SIGNIN_LINES: Record<SigninNeed, string> = {
  usage: 'Sign in for usage',
  expired: 'Sign in again to use this account',
}

export function signedInAt(account: AccountView) {
  return (account as SignedAccountView).signed_in_at ?? null
}

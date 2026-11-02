// /accounts: one card per provider (GET /api/accounts' providers), each
// with its default account's usage, the account switcher and Add /
// Manage Accounts….
import { useAccounts } from '@/api/hooks'
import { ErrorText } from '@/components/error-text'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/shell/page-title'

import { ProviderCard } from './ProviderCard'

export function AccountsPage() {
  usePageTitle('Accounts')
  const query = useAccounts()

  if (query.isPending) {
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-3 p-6" aria-label="Loading accounts">
        <Skeleton className="h-64" />
        <Skeleton className="h-64" />
      </div>
    )
  }
  if (!query.data) {
    return (
      <div className="p-6">
        <ErrorText error={query.error} />
      </div>
    )
  }
  const { accounts, providers } = query.data
  return (
    <div data-accounts-page className="flex flex-col gap-4 p-6">
      <ErrorText error={query.error} />
      <div className="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] items-start gap-3">
        {providers.map((p) => (
          <ProviderCard key={p.id} provider={p} accounts={accounts.filter((a) => a.provider === p.id)} />
        ))}
      </div>
    </div>
  )
}

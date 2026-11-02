// Owned by the accounts feature (AccountsPage). The card, switch, empty
// state and sign-in tests serve GET /api/accounts from a fixture with
// page.route, so they never see another test's changes; the add and remove
// tests use the shared mock with unique names. Every value is made up.
import { expect, test, type Page, type Route } from '@playwright/test'

const MIN = 60_000
const HOUR = 60 * MIN
const DAY = 24 * HOUR

type Fixture = { accounts: Record<string, unknown>[]; providers: Record<string, unknown>[] }

const PROVIDERS = [
  {
    id: 'anthropic',
    harnesses: ['claude_code'],
    variables: { oauth: 'CLAUDE_CODE_OAUTH_TOKEN', api_key: 'ANTHROPIC_API_KEY' },
  },
  { id: 'openai', harnesses: ['codex_cli'], variables: { api_key: 'CODEX_API_KEY' } },
]

function iso(offset: number) {
  return new Date(Date.now() + offset).toISOString()
}

function account(name: string, fields: Record<string, unknown> = {}) {
  return {
    name,
    provider: 'anthropic',
    kind: 'oauth',
    secret: 'ACCOUNT_' + name.toUpperCase().replace(/-/g, '_'),
    email: '',
    default: false,
    created_at: iso(-DAY),
    usage: null,
    ...fields,
  }
}

function fixture(): Fixture {
  return {
    providers: PROVIDERS,
    accounts: [
      account('main-max', {
        email: 'main@example.com',
        default: true,
        usage: {
          updated_at: iso(-10_000),
          windows: [
            { id: 'session', label: 'Session', used: 0.164, resets_at: iso(4 * HOUR + 11 * MIN + 30_000) },
            { id: 'weekly', label: 'Weekly', used: 0.17, resets_at: iso(DAY + 19 * HOUR + 30 * MIN) },
            { id: 'opus', label: 'Opus', used: 0.2, resets_at: iso(DAY + 19 * HOUR + 30 * MIN) },
          ],
          error: '',
        },
      }),
      account('team-key', { kind: 'api_key' }),
      account('stale-max', {
        email: 'stale@example.com',
        usage: { updated_at: iso(-5 * MIN), windows: [], error: 'token rejected: 401 invalid credentials' },
      }),
    ],
  }
}

function apiError(route: Route, status: number, code: string, message: string) {
  return route.fulfill({ status, json: { error: { code, message, request_id: 'test' } } })
}

// Serves /api/accounts and its default routes from data, changing it like
// the server would. Returns the calls made, in order.
async function serve(page: Page, data: Fixture) {
  const calls: string[] = []
  await page.route(/\/api\/accounts(\/.*)?$/, async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname.replace(/^\/api/, '')
    const method = request.method()
    if (method === 'GET' && path === '/accounts') {
      return route.fulfill({ json: data })
    }
    calls.push(`${method} ${path}`)
    const [, , name, verb] = path.split('/')
    const found = data.accounts.find((a) => a.name === name)
    if (verb === 'default' && found) {
      if (method === 'POST') {
        for (const a of data.accounts) {
          if (a.provider === found.provider) {
            a.default = false
          }
        }
      }
      found.default = method === 'POST'
      return route.fulfill({ json: found })
    }
    if (method === 'DELETE' && !verb && found) {
      data.accounts = data.accounts.filter((a) => a !== found)
      return route.fulfill({ status: 204 })
    }
    if (method === 'POST' && path === '/accounts') {
      const body = request.postDataJSON() as Record<string, unknown>
      const created = account(String(body.name), { kind: body.kind, email: body.email ?? '' })
      data.accounts.push(created)
      return route.fulfill({ status: 201, json: created })
    }
    return apiError(route, 404, 'ENOTFOUND', 'no such endpoint')
  })
  return calls
}

const card = (page: Page, provider: string) => page.locator(`[data-card="${provider}"]`)

test('the card shows usage bars, relative times and the active account', async ({ page }) => {
  await serve(page, fixture())
  await page.goto('/panel/accounts')

  const claude = card(page, 'anthropic')
  await expect(claude.getByRole('heading', { name: 'Claude' })).toBeVisible()
  await expect(claude.getByTestId('usage-updated')).toHaveText('Updated just now')
  await expect(claude.getByTestId('window-label')).toHaveText(['Session', 'Weekly', 'Opus'])
  const session = claude.locator('[data-window="session"]')
  await expect(session.getByTestId('window-used')).toHaveText('16% used')
  await expect(session.getByTestId('window-resets')).toHaveText('Resets in 4h 11m')
  await expect(session.getByRole('meter')).toHaveAttribute('aria-valuenow', '16')
  await expect(claude.locator('[data-window="weekly"]').getByTestId('window-resets')).toHaveText('Resets in 1d 19h')
  await expect(claude.locator('[data-window="opus"]').getByTestId('window-used')).toHaveText('20% used')
  await expect(claude.getByTestId('active-label')).toHaveText('main@example.com')

  // The dropdown: System default first, the active one greyed, the
  // rejected one asks to sign in again.
  await claude.getByTestId('active-account').click()
  const menu = page.getByRole('menu')
  await expect(menu.getByText('Switch to')).toBeVisible()
  await expect(menu.getByRole('menuitem')).toHaveText([
    /^System default/,
    /^main@example\.com.*Active/,
    /^team-key/,
    /^stale@example\.com.*Sign in.*Sign in again to use this account/,
  ])
  await expect(menu.locator('[data-switch="main-max"]')).toHaveAttribute('data-disabled', '')

  // Codex has no account: the empty state in place, no bars.
  await page.keyboard.press('Escape')
  await expect(card(page, 'openai').getByTestId('accounts-empty')).toHaveText('No Codex account yet. Add one.')
})

test('usage: null says so in one line, and a usage error is shown verbatim', async ({ page }) => {
  const data = fixture()
  for (const a of data.accounts) {
    a.default = a.name === 'team-key'
  }
  await serve(page, data)
  await page.goto('/panel/accounts')
  const claude = card(page, 'anthropic')
  await expect(claude.getByTestId('usage-none')).toHaveText('Usage not available for this account')
  await expect(claude.getByRole('meter')).toHaveCount(0)
  await expect(claude.getByTestId('active-label')).toHaveText('team-key')

  for (const a of data.accounts) {
    a.default = a.name === 'stale-max'
  }
  await page.reload()
  await expect(claude.getByRole('alert')).toHaveText('token rejected: 401 invalid credentials')
  await expect(claude.getByTestId('sign-in-line')).toContainText('Sign in again to use this account')
})

test('switching posts the default route, System default deletes it, the label follows', async ({ page }) => {
  const calls = await serve(page, fixture())
  await page.goto('/panel/accounts')
  const claude = card(page, 'anthropic')
  await expect(claude.getByTestId('active-label')).toHaveText('main@example.com')

  await claude.getByTestId('active-account').click()
  await page.getByRole('menu').locator('[data-switch="team-key"]').click()
  await expect(claude.getByTestId('active-label')).toHaveText('team-key')
  expect(calls).toEqual(['POST /accounts/team-key/default'])
  await expect(page.getByRole('menu')).toHaveCount(0)

  await claude.getByTestId('active-account').click()
  await page.getByRole('menu').locator('[data-switch="system"]').click()
  await expect(claude.getByTestId('active-label')).toHaveText('System default')
  expect(calls).toEqual(['POST /accounts/team-key/default', 'DELETE /accounts/team-key/default'])
  await expect(claude.getByTestId('usage-none')).toHaveText(
    'No default account: sandboxes get only what their spec names',
  )
})

test('a failed switch shows the backend error verbatim and keeps the label', async ({ page }) => {
  await serve(page, fixture())
  await page.route('**/api/accounts/team-key/default', (route) =>
    apiError(route, 409, 'ECONFLICT', 'account team-key is being rotated'),
  )
  await page.goto('/panel/accounts')
  const claude = card(page, 'anthropic')
  await claude.getByTestId('active-account').click()
  await page.getByRole('menu').locator('[data-switch="team-key"]').click()
  await expect(claude.getByRole('alert')).toHaveText('account team-key is being rotated')
  await expect(claude.getByTestId('active-label')).toHaveText('main@example.com')
})

test('the empty state adds an account; OpenAI offers only an API key', async ({ page }) => {
  const data = fixture()
  data.accounts = []
  await serve(page, data)
  await page.goto('/panel/accounts')
  await expect(card(page, 'anthropic').getByTestId('accounts-empty')).toHaveText('No Claude account yet. Add one.')
  await expect(card(page, 'anthropic').getByTestId('active-account')).toHaveCount(0)

  await card(page, 'openai').getByTestId('accounts-empty').click()
  const dialog = page.getByTestId('add-account-dialog')
  await expect(dialog.getByRole('heading')).toHaveText('Add Codex account')
  await expect(dialog.getByRole('radio')).toHaveText(['API key'])
  await expect(dialog.getByTestId('value-hint')).toHaveText('platform.openai.com → API keys')
  await expect(dialog.getByRole('button', { name: 'Add: needs a name' })).toBeDisabled()
  await dialog.getByLabel('Name').fill('Bad Name')
  await expect(dialog.getByRole('button', { name: 'Add: name is a-z, 0-9 and -, up to 32' })).toBeDisabled()
  await dialog.getByLabel('Name').fill('codex-team')
  await expect(dialog.getByRole('button', { name: 'Add: needs the api key' })).toBeDisabled()
})

test('a failed add posts the value once, shows the error verbatim and clears the value', async ({ page }) => {
  const value = 'made-up-value-' + Date.now()
  let posts = 0
  page.on('request', (r) => {
    if (r.method() === 'POST' && new URL(r.url()).pathname === '/api/accounts') {
      posts += 1
    }
  })
  await page.goto('/panel/accounts')
  const claude = card(page, 'anthropic')
  await claude.locator('[data-act="add-account"]').click()
  const dialog = page.getByTestId('add-account-dialog')
  await expect(dialog.getByTestId('value-hint')).toHaveText(
    '`claude setup-token` on a machine signed in to the account',
  )
  // franco-max is seeded in the mock: the server refuses a second one.
  await dialog.getByLabel('Name').fill('franco-max')
  await dialog.getByLabel('Subscription token', { exact: true }).fill(value)
  await expect(dialog.getByLabel('Subscription token', { exact: true })).toHaveAttribute('type', 'password')
  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/accounts'))
  await dialog.getByRole('button', { name: 'Add', exact: true }).click()
  expect((await posted).postDataJSON()).toMatchObject({
    name: 'franco-max',
    provider: 'anthropic',
    kind: 'oauth',
    value,
  })

  await expect(dialog.getByRole('alert')).toHaveText('account franco-max already exists')
  await expect(dialog.getByLabel('Subscription token', { exact: true })).toHaveValue('')
  await expect(dialog.getByRole('button', { name: 'Add: needs the subscription token' })).toBeDisabled()
  expect(posts).toBe(1)
  expect(await page.content()).not.toContain(value)
})

test('adding an account lists it and never shows the value again', async ({ page }) => {
  const name = 'codex-' + Date.now().toString(36)
  const value = 'made-up-key-' + Date.now()
  await page.goto('/panel/accounts')
  const codex = card(page, 'openai')
  await codex.locator('[data-act="add-account"]').click()
  const dialog = page.getByTestId('add-account-dialog')
  await dialog.getByLabel('Name').fill(name)
  await dialog.getByLabel('API key', { exact: true }).fill(value)
  await dialog.getByLabel('Email (optional)').fill(`${name}@example.com`)
  await dialog.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(dialog).toHaveCount(0)

  await codex.locator('[data-act="add-account"]').click()
  await expect(page.getByTestId('add-account-dialog').getByLabel('API key', { exact: true })).toHaveValue('')
  await page.keyboard.press('Escape')

  await codex.locator('[data-act="manage-accounts"]').click()
  await expect(page.getByTestId('manage-accounts-dialog').locator(`[data-account="${name}"]`)).toContainText(
    `${name}@example.com`,
  )
  expect(await page.content()).not.toContain(value)
})

test('Remove confirms, then removes the account', async ({ page, request }) => {
  const name = 'gone-' + Date.now().toString(36)
  const created = await request.post('/api/accounts', {
    data: { name, provider: 'anthropic', kind: 'api_key', value: 'made-up-value' },
  })
  expect(created.status()).toBe(201)

  await page.goto('/panel/accounts')
  await card(page, 'anthropic').locator('[data-act="manage-accounts"]').click()
  const row = page.getByTestId('manage-accounts-dialog').locator(`[data-account="${name}"]`)
  await row.locator('[data-act="remove"]').click()
  const confirm = page.getByRole('alertdialog')
  await expect(confirm.getByRole('heading')).toHaveText(`Remove account ${name}?`)
  const deleted = page.waitForRequest((r) => r.method() === 'DELETE' && r.url().endsWith(`/api/accounts/${name}`))
  await confirm.getByRole('button', { name: 'Remove' }).click()
  await deleted
  await expect(row).toHaveCount(0)
})

test('a refused remove shows the backend refusal verbatim', async ({ page }) => {
  await serve(page, fixture())
  await page.route('**/api/accounts/team-key', (route) =>
    route.request().method() === 'DELETE'
      ? apiError(route, 409, 'ECONFLICT', 'account team-key is in use by running sandbox review-42')
      : route.fallback(),
  )
  await page.goto('/panel/accounts')
  await card(page, 'anthropic').locator('[data-act="manage-accounts"]').click()
  const row = page.getByTestId('manage-accounts-dialog').locator('[data-account="team-key"]')
  await row.locator('[data-act="remove"]').click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Remove' }).click()
  await expect(row.getByRole('alert')).toHaveText('account team-key is in use by running sandbox review-42')
  await expect(row).toBeVisible()
})

test('Sign in on a rejected token replaces it: DELETE, then POST', async ({ page }) => {
  const calls = await serve(page, fixture())
  await page.goto('/panel/accounts')
  const claude = card(page, 'anthropic')
  await claude.getByTestId('active-account').click()
  await page.getByRole('menu').locator('[data-switch="stale-max"]').click()

  const dialog = page.getByTestId('add-account-dialog')
  await expect(dialog.getByRole('heading')).toHaveText('Sign in again: stale-max')
  await expect(dialog.getByLabel('Name')).toHaveValue('stale-max')
  await expect(dialog.getByLabel('Email (optional)')).toHaveValue('stale@example.com')
  await expect(dialog.getByRole('radio', { name: 'Subscription token' })).toHaveAttribute('data-state', 'on')
  await dialog.getByLabel('Subscription token', { exact: true }).fill('made-up-new-token')
  await dialog.getByRole('button', { name: 'Replace token', exact: true }).click()
  await expect(dialog).toHaveCount(0)
  expect(calls).toEqual(['DELETE /accounts/stale-max', 'POST /accounts'])
})

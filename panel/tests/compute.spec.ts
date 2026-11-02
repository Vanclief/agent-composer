// Owned by the compute feature (ComputePage, SandboxPage). Every test
// shares one mock running in parallel, so counts are checked against the
// API's answer, and sandboxes a test changes are its own (or review-42-2,
// which only the Stop/Start test touches and leaves stopped).
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

const group = (page: Page, status: string) => page.getByRole('region', { name: status, exact: true })
const card = (page: Page, name: string) => page.getByRole('article', { name, exact: true })

// A fresh task with one allocated sandbox, starting for two seconds.
async function allocate(request: APIRequestContext) {
  const name = `compute-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`
  const created = await request.post('/api/tasks', {
    data: { title: name, name, brief: 'Compute test', assignee: 'claude_code', for: '' },
  })
  expect(created.ok()).toBe(true)
  const sandbox = await request.post(`/api/tasks/${name}/sandboxes`, {
    data: { image: 'agc-agent', cpus: 1, memory: 1024, command: 'claude' },
  })
  expect(sandbox.status()).toBe(201)
  expect(await sandbox.json()).toMatchObject({ name, status: 'starting' })
  return name
}

test('sandboxes are grouped by status in order, with counts', async ({ page, request }) => {
  const fresh = await allocate(request)
  await page.goto('/panel/compute')

  await expect(card(page, fresh)).toBeVisible()
  await expect(group(page, 'starting').getByRole('article', { name: fresh })).toBeVisible()
  await expect(group(page, 'running').getByRole('article', { name: 'review-42', exact: true })).toBeVisible()
  await expect(group(page, 'stopped').getByRole('article', { name: 'docs-pass' })).toBeVisible()
  const failed = group(page, 'failed').getByRole('article', { name: 'boot-fail' })
  await expect(failed).toBeVisible()
  await expect(failed.getByTestId('status-error')).toHaveText('clone failed: repository not found')
  // A failed sandbox can only be deleted.
  await expect(failed.getByRole('button')).toHaveText(['Delete'])

  // Groups come in the fixed order: by status light, attention first.
  const order = ['failed', 'starting', 'running', 'unknown', 'stopped']
  const labels = await page.getByRole('region').evaluateAll((sections) => sections.map((s) => s.ariaLabel ?? ''))
  const shown = labels.filter((label) => order.includes(label))
  expect(shown).toEqual(order.filter((status) => shown.includes(status)))
  expect(shown.filter((s) => s !== 'unknown').slice(0, 4)).toEqual(['failed', 'starting', 'running', 'stopped'])

  // Each count matches its cards and the API.
  for (const status of ['running', 'failed', 'stopped']) {
    await expect
      .poll(async () => {
        const { sandboxes } = (await (await request.get('/api/sandboxes')).json()) as {
          sandboxes: { status: string }[]
        }
        const want = sandboxes.filter((s) => s.status === status).length
        const label = await group(page, status).getByTestId('group-count').textContent()
        const cards = await group(page, status).getByRole('article').count()
        return [Number(label), cards].every((n) => n === want)
      })
      .toBe(true)
  }

  // Once booted, the row moves to running.
  await expect(group(page, 'running').getByRole('article', { name: fresh })).toBeVisible({ timeout: 10_000 })
})

test('a row shows its task linking to the task, and opens its sandbox', async ({ page }) => {
  await page.goto('/panel/compute')
  const review = card(page, 'review-42')
  await expect(review.getByTitle('CPUs')).toHaveText('2')
  await expect(review.getByTitle('Memory')).toHaveText('4 GB')
  await expect(review.getByTitle('Image')).toHaveText('agc-agent')
  const chip = review.getByRole('link', { name: 'Review 42' })
  await expect(chip).toHaveAttribute('href', '/panel/tasks/review-42')
  await chip.click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42/)

  await page.goto('/panel/compute')
  // Until the task titles load, the task link reads docs-pass too.
  await expect(card(page, 'docs-pass').getByRole('link', { name: 'Docs pass' })).toBeVisible()
  await card(page, 'docs-pass').getByRole('link', { name: 'docs-pass' }).click()
  await expect(page).toHaveURL(/\/panel\/compute\/docs-pass$/)
  await expect(page.getByTestId('page-title')).toHaveText('docs-pass')
})

// review-42-2's clone is clean, so Stop says nothing is lost.
test('Start then Stop on review-42-2', async ({ page }) => {
  await page.goto('/panel/compute')
  const older = card(page, 'review-42-2')
  await expect(older.getByRole('button', { name: 'Stop' })).toHaveCount(0)
  await older.getByRole('button', { name: 'Start' }).click()
  await expect(group(page, 'running').getByRole('article', { name: 'review-42-2' })).toBeVisible()
  await card(page, 'review-42-2').getByRole('button', { name: 'Stop' }).click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText('Nothing unpushed')
  await dialog.getByRole('button', { name: 'Stop', exact: true }).click()
  await expect(group(page, 'stopped').getByRole('article', { name: 'review-42-2' })).toBeVisible()
  await expect(card(page, 'review-42-2').getByRole('button', { name: 'Start' })).toBeVisible()
})

test('Delete asks first, then removes the card', async ({ page, request }) => {
  const name = await allocate(request)
  await page.goto('/panel/compute')
  await card(page, name).getByRole('button', { name: 'Delete' }).click()
  const dialog = page.getByRole('alertdialog')
  // Still starting, so the clone cannot be checked.
  await expect(dialog.getByTestId('lost-work')).toHaveText('Could not check the clone')
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(card(page, name)).toBeVisible()

  await card(page, name).getByRole('button', { name: 'Delete' }).click()
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: /^Delete/ })
    .click()
  await expect(card(page, name)).toHaveCount(0)
  expect((await request.get(`/api/sandboxes/${name}`)).status()).toBe(404)
})

test('a starting sandbox page says it is booting, then shows its agent terminal', async ({ page, request }) => {
  const name = await allocate(request)
  await page.goto(`/panel/compute/${name}`)
  await expect(page.getByText('Booting: terminals appear when the sandbox is running')).toBeVisible()
  // Polled every 2s while starting, so it shows up well before the 5s poll.
  const agent = page.getByRole('list', { name: 'Terminals' }).getByRole('button').first()
  await expect(agent).toHaveAttribute('title', 'agent', { timeout: 4500 })
  await expect(page.getByTestId('sandbox-header').locator('[data-status]')).toHaveText('running')
})

test('a failed sandbox page shows its error and only Delete', async ({ page }) => {
  await page.goto('/panel/compute/boot-fail')
  await expect(page.getByTestId('status-error')).toHaveText('clone failed: repository not found')
  await expect(page.getByTestId('sandbox-header').getByRole('button')).toHaveText(['Delete'])
  await expect(page.getByRole('list', { name: 'Terminals' })).toHaveCount(0)
})

test('deleting from the sandbox page goes back to Compute', async ({ page, request }) => {
  const name = await allocate(request)
  await page.goto(`/panel/compute/${name}`)
  await page.getByRole('button', { name: 'Delete' }).click()
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: /^Delete/ })
    .click()
  await expect(page).toHaveURL(/\/panel\/compute$/)
  await expect(card(page, name)).toHaveCount(0)
})

// review-42's clone has uncommitted changes and 2 unpushed commits. Every
// dialog is cancelled, so the sandbox stays running for other tests.
test('Stop and Delete on review-42 say 2 commits and changes are lost', async ({ page }) => {
  const lost = '2 unpushed commits and uncommitted changes will be lost'
  await page.goto('/panel/compute')
  const review = card(page, 'review-42')

  await review.getByRole('button', { name: 'Delete' }).click()
  let dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText(lost)
  await expect(dialog.getByRole('button', { name: 'Delete and lose 2 commits and changes' })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Cancel' }).click()

  await review.getByRole('button', { name: 'Stop' }).click()
  dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText(lost)
  await expect(dialog.getByRole('button', { name: 'Stop and lose 2 commits and changes' })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(group(page, 'running').getByRole('article', { name: 'review-42', exact: true })).toBeVisible()

  // The task's Compute tab asks the same.
  await page.goto('/panel/tasks/review-42/compute')
  await page.locator('[data-sandbox="review-42"]').getByRole('button', { name: 'Delete' }).click()
  dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText(lost)
  await expect(dialog).toContainText('The task review-42 stays.')
  await dialog.getByRole('button', { name: 'Cancel' }).click()
})

test('a stopped sandbox says the clone could not be checked', async ({ page }) => {
  await page.goto('/panel/compute')
  await card(page, 'docs-pass').getByRole('button', { name: 'Delete' }).click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText('Could not check the clone')
  await expect(dialog.getByRole('button', { name: 'Delete anyway' })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Cancel' }).click()
})

test('the sandbox API reads the work only when asked', async ({ request }) => {
  const one = await (await request.get('/api/sandboxes/review-42')).json()
  expect(one.work).toMatchObject({ dirty: true, unpushed: 2 })
  const plain = (await (await request.get('/api/sandboxes')).json()) as { sandboxes: { work: unknown }[] }
  expect(plain.sandboxes.every((s) => s.work === null)).toBe(true)
  const withWork = (await (await request.get('/api/sandboxes?work=1')).json()) as {
    sandboxes: { name: string; work: unknown }[]
  }
  expect(withWork.sandboxes.find((s) => s.name === 'review-42')?.work).toMatchObject({ dirty: true, unpushed: 2 })
})

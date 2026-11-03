// Owned by the agents feature (AgentsPage, AgentPage). Every test shares
// one mock running in parallel: only the New agent test creates a
// reviewer, the seeds orchestrator-1, reviewer-2, ada and writer-1 are
// never changed, and the other tests make their own writer or
// orchestrator to keep, wake and sleep.
import { expect, test, type APIRequestContext } from '@playwright/test'

async function createAgent(request: APIRequestContext, role: string) {
  const created = await request.post('/api/agents', { data: { role } })
  expect(created.status()).toBe(201)
  return ((await created.json()) as { name: string }).name
}

test('the list shows awake, then waking, then asleep, each state verbatim', async ({ page }) => {
  await page.goto('/panel/agents')
  const rows = page.getByRole('list', { name: 'Agents' }).locator('li[data-agent]')
  await expect(rows.first()).toBeVisible()

  const seeds = ['orchestrator-1', 'writer-1', 'reviewer-2', 'ada']
  const order = (await rows.evaluateAll((els) => els.map((el) => el.getAttribute('data-agent')))).filter((n) =>
    seeds.includes(n ?? ''),
  )
  expect(order.indexOf('orchestrator-1')).toBeLessThan(order.indexOf('writer-1'))
  expect(order.indexOf('writer-1')).toBeLessThan(order.indexOf('reviewer-2'))
  expect(order.indexOf('writer-1')).toBeLessThan(order.indexOf('ada'))

  // States in the order the rows come, never relabeled.
  const states = await rows.getByTestId('agent-state').allTextContents()
  const rank = ['awake', 'waking', 'asleep']
  expect(states.map((s) => rank.indexOf(s))).toEqual([...states.map((s) => rank.indexOf(s))].sort((a, b) => a - b))

  const row = (name: string) => page.locator(`li[data-agent="${name}"]`)
  await expect(row('orchestrator-1').getByTestId('agent-state')).toHaveText('awake')
  await expect(row('writer-1').getByTestId('agent-state')).toHaveText('waking')
  await expect(row('reviewer-2').getByTestId('agent-state')).toHaveText('asleep')
  await expect(row('orchestrator-1').getByTestId('agent-tasks')).toHaveText('2')
  await expect(row('orchestrator-1').getByTestId('agent-role')).toHaveText('Orchestrator')
  // Kept agents are named; numbered ones say so.
  await expect(row('reviewer-2').getByTestId('agent-numbered')).toHaveText('numbered')
  await expect(row('ada').getByTestId('agent-numbered')).toHaveCount(0)
})

test('New agent creates reviewer-3 and lands on its page', async ({ page }) => {
  await page.goto('/panel/agents')
  await page.getByRole('button', { name: 'New agent' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('button', { name: 'Create: needs a role' })).toBeDisabled()
  await dialog.getByLabel('Role').fill('Reviewer')
  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/agents'))
  await dialog.getByRole('button', { name: 'Create', exact: true }).click()
  expect((await posted).postDataJSON()).toMatchObject({ role: 'Reviewer', harness: 'claude_code' })

  await expect(page).toHaveURL(/\/panel\/agents\/reviewer-3$/)
  await expect(page.getByTestId('agent-name')).toHaveText('reviewer-3')
  await expect(page.getByTestId('agent-state')).toHaveText('asleep')
  await expect(page.locator('[data-card="home"]')).toContainText('Not woken yet')
  await expect(page.locator('[data-card="tasks"]')).toContainText('Manages no tasks')
})

test('Keep sends name and role and the page shows the agent kept', async ({ page, request }) => {
  const numbered = await createAgent(request, 'Writer')
  const kept = `kept-${Date.now().toString(36)}`
  await page.goto(`/panel/agents/${numbered}`)
  await expect(page.getByTestId('agent-numbered')).toBeVisible()

  await page.getByRole('button', { name: 'Keep…' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByLabel('Role')).toHaveValue('Writer')
  await expect(dialog.getByRole('button', { name: 'Keep: needs a name' })).toBeDisabled()
  await dialog.getByLabel('Name').fill('Bad Name')
  await expect(dialog.getByRole('button', { name: 'Keep: lowercase letters, digits and dashes' })).toBeDisabled()
  await dialog.getByLabel('Name').fill(kept)
  await dialog.getByLabel('Role').fill('Editor')
  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith(`/api/agents/${numbered}/keep`))
  await dialog.getByRole('button', { name: 'Keep', exact: true }).click()
  expect((await posted).postDataJSON()).toEqual({ name: kept, role: 'Editor' })

  await expect(page).toHaveURL(new RegExp(`/panel/agents/${kept}$`))
  await expect(page.getByTestId('agent-name')).toHaveText(kept)
  await expect(page.getByTestId('agent-kept')).toHaveText('kept')
  await expect(page.getByTestId('agent-role')).toHaveText('Editor')
  await expect(page.getByRole('button', { name: 'Rename…' })).toBeVisible()
})

test('Wake sends the request; the chip reads waking, then awake, and Sleep puts it back', async ({ page, request }) => {
  const name = await createAgent(request, 'Orchestrator')
  await page.goto(`/panel/agents/${name}`)
  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
  await expect(page.getByTestId('agent-state')).toHaveText('asleep')

  // dispatchEvent: toasts stacking over the actions row made a real click
  // wait for the pointer path and time out under load (see live.spec.ts).
  await page.getByRole('button', { name: 'Wake' }).dispatchEvent('click')
  // The mock answers waking and, two seconds later, awake through an
  // agent.state event; under load the first state may already be gone when
  // the assertion runs, so accept either, then wait for awake.
  await expect(page.getByTestId('agent-state')).toHaveText(/^(waking|awake)$/)
  await expect(page.getByTestId('agent-state')).toHaveText('awake', { timeout: 10000 })
  await expect(page.locator('[data-card="home"]').getByRole('link')).toHaveAttribute('href', `/panel/compute/${name}`)

  await page.getByRole('button', { name: 'Sleep' }).dispatchEvent('click')
  await expect(page.getByTestId('agent-state')).toHaveText('asleep')
  await expect(page.getByRole('button', { name: 'Wake' })).toBeVisible()
})

test('Sleep is disabled while waking, saying so in its label', async ({ page }) => {
  await page.goto('/panel/agents/writer-1')
  await expect(page.getByTestId('agent-state')).toHaveText('waking')
  await expect(page.getByRole('button', { name: 'Waking…' })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Wake' })).toHaveCount(0)
})

test('the detail page links home, tasks and conversations, the DM first', async ({ page }) => {
  await page.goto('/panel/agents/orchestrator-1')
  await expect(page.locator('[data-card="home"]').getByRole('link')).toHaveAttribute(
    'href',
    '/panel/compute/orchestrator-1',
  )
  const tasks = page.locator('[data-card="tasks"] a[data-task]')
  await expect(tasks).toHaveCount(2)
  await expect(page.locator('a[data-task="review-42"]')).toHaveAttribute('href', '/panel/tasks/review-42')
  await expect(page.locator('a[data-task="review-42"] [data-status]')).toHaveText('review')
  const channels = page.locator('[data-card="conversations"] a[data-channel]')
  await expect(channels.first()).toHaveAttribute('data-channel', '#orchestrator-1')
  await expect(channels.first()).toHaveAttribute('href', '/panel/messages/orchestrator-1')
  await expect(channels).toHaveCount(3)
})

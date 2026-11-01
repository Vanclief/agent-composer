// Owned by the tasks feature (TasksPage, TaskPage, SpecTab).
import { expect, test, type APIRequestContext } from '@playwright/test'

// All tests share one mock, so flows that change a task use their own.
async function createTask(request: APIRequestContext, title: string, spec: Record<string, unknown> = {}) {
  const response = await request.post('/api/tasks', { data: { title, spec } })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { name: string; title: string }
}

function unique(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`
}

test('tasks list shows each task with its title and backend status', async ({ page }) => {
  await page.goto('/panel/tasks')
  await expect(page.getByTestId('page-title')).toHaveText('Tasks')
  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()

  const review = page.getByRole('listitem').filter({ hasText: 'Review 42' })
  await expect(review.getByRole('link', { name: 'Review 42' })).toBeVisible()
  await expect(review.getByText('review-42', { exact: true })).toBeVisible()
  await expect(review.locator('[data-status]')).toHaveText('running')
  await expect(review.getByRole('button', { name: 'Stop' })).toBeVisible()

  const flaky = page.getByRole('listitem').filter({ hasText: 'Flaky CI' })
  await expect(flaky.locator('[data-status]')).toHaveText('stopped')
  await expect(flaky.getByRole('button', { name: 'Start' })).toBeVisible()
  await expect(flaky.getByText('3h', { exact: true })).toBeVisible()

  const docs = page.getByRole('listitem').filter({ hasText: 'docs-pass' })
  await expect(docs.locator('[data-status]')).toHaveText('unknown')
  await expect(docs.locator('[data-status]')).toHaveAttribute('title', 'msb: connection refused')
})

test('a row opens its task', async ({ page }) => {
  await page.goto('/panel/tasks')
  await page.getByRole('link', { name: 'Flaky CI' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks\/flaky-ci\/terminals$/)
})

test('stop and start a task from the list', async ({ page, request }) => {
  const task = await createTask(request, unique('Stop start'))
  await page.goto('/panel/tasks')
  const row = page.getByRole('listitem').filter({ hasText: task.name })

  // Created tasks boot for 2s, then run.
  await expect(row.locator('[data-status]')).toHaveText('running', { timeout: 10_000 })
  await row.getByRole('button', { name: 'Stop' }).click()
  await expect(row.locator('[data-status]')).toHaveText('stopped')

  await row.getByRole('button', { name: 'Start' }).click()
  await expect(row.locator('[data-status]')).toHaveText('starting')
  await expect(row.getByRole('button', { name: 'Start' })).toHaveCount(0)
})

test('a failed action shows the backend error in its row', async ({ page, request }) => {
  const task = await createTask(request, unique('Race'))
  await page.goto('/panel/tasks')
  const row = page.getByRole('listitem').filter({ hasText: task.name })
  await expect(row.getByRole('button', { name: 'Stop' })).toBeVisible({ timeout: 10_000 })

  // Stopped elsewhere before the click lands.
  const stopped = await request.post(`/api/tasks/${task.name}/stop`)
  expect(stopped.ok()).toBe(true)
  await row.getByRole('button', { name: 'Stop' }).click()
  await expect(row.getByRole('alert')).toHaveText(`task ${task.name} is stopped, not running`)
})

test('delete asks for a second click in place', async ({ page, request }) => {
  const task = await createTask(request, unique('Delete me'))
  await page.goto('/panel/tasks')
  const row = page.getByRole('listitem').filter({ hasText: task.name })

  await row.getByRole('button', { name: 'Delete' }).click()
  await expect(row.getByRole('button', { name: 'Delete?' })).toBeVisible()
  // Unconfirmed, it disarms after 3s.
  await expect(row.getByRole('button', { name: 'Delete', exact: true })).toBeVisible({ timeout: 5000 })

  await row.getByRole('button', { name: 'Delete' }).click()
  await row.getByRole('button', { name: 'Delete?' }).click()
  await expect(row).toHaveCount(0)
})

test('a task opens on its terminals tab under its header', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/terminals$/)
  await expect(page.getByRole('heading', { level: 2, name: 'Review 42' })).toBeVisible()
  await expect(page.getByTestId('page-title')).toHaveText('Review 42')
  const header = page.getByTestId('task-header')
  await expect(header.locator('[data-status]')).toHaveText('running')
  await expect(header.getByRole('button', { name: 'Stop' })).toBeVisible()
  await expect(header.getByText('review-42', { exact: true })).toBeVisible()

  const tabs = page.getByRole('navigation', { name: 'Task' })
  await expect(tabs.getByRole('link')).toHaveText(['Terminals', 'Messages', 'Spec'])
  await tabs.getByRole('link', { name: 'Spec' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/spec$/)
})

test('deleting from the task page goes back to tasks', async ({ page, request }) => {
  const task = await createTask(request, unique('Delete page'))
  await page.goto(`/panel/tasks/${task.name}/spec`)
  const header = page.getByTestId('task-header')
  await header.getByRole('button', { name: 'Delete' }).click()
  await header.getByRole('button', { name: 'Delete?' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(page.getByRole('listitem').filter({ hasText: task.name })).toHaveCount(0)
})

test('spec tab shows the spec as cards and toggles to YAML', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/spec')

  const machine = page.getByRole('region', { name: 'Machine' })
  await expect(machine.getByText('agc-agent', { exact: true })).toBeVisible()
  await expect(machine.getByText('2 CPU', { exact: true })).toBeVisible()
  await expect(machine.getByText('4 GB', { exact: true })).toBeVisible()

  const code = page.getByRole('region', { name: 'Code' })
  await expect(code.getByText('github.com/vanclief/agent-composer', { exact: true })).toBeVisible()
  await expect(code.getByText('master', { exact: true })).toBeVisible()
  await expect(code.getByText('/home/agent/agent-composer', { exact: true })).toBeVisible()
  await expect(code.getByText('go mod download')).toHaveCount(0)
  await code.getByRole('button', { name: /Setup/ }).click()
  await expect(code.getByText('go mod download')).toBeVisible()

  const secrets = page.getByRole('region', { name: 'Secrets' })
  await expect(secrets.locator('[title="api.github.com, github.com"]')).toHaveText('GITHUB_TOKEN')

  await expect(page.getByRole('region', { name: 'Env' })).toContainText('GOFLAGS=-mod=mod')
  await expect(page.getByRole('region', { name: 'Volumes' })).toContainText('go-mod-cache')

  const channels = page.getByRole('region', { name: 'Channels' })
  await expect(channels.locator('[data-mode="write"]')).toHaveText('releases')
  await expect(channels.locator('[data-mode="read"]')).toHaveText('alerts')

  await page.getByRole('button', { name: 'Raw YAML' }).click()
  await expect(page.getByText('image: agc-agent')).toBeVisible()
  await expect(machine).toHaveCount(0)
})

test('spec tab shows empty slots and memory in GB', async ({ page, request }) => {
  const task = await createTask(request, unique('Small'), {
    cpus: 1,
    memory: 1536,
    volumes: ['host:/srv/data:/data'],
  })
  await page.goto(`/panel/tasks/${task.name}/spec`)
  await expect(page.getByRole('region', { name: 'Machine' }).getByText('1.5 GB', { exact: true })).toBeVisible()
  await expect(page.getByRole('region', { name: 'Code' }).getByText('No repo')).toBeVisible()
  await expect(page.getByRole('region', { name: 'Secrets' }).getByText('No secrets')).toBeVisible()
  await expect(page.getByRole('region', { name: 'Volumes' }).getByRole('img', { name: 'Read-only' })).toBeVisible()
})

test('an unknown task shows the backend error verbatim', async ({ page }) => {
  await page.goto('/panel/tasks/nope/spec')
  await expect(page.getByRole('alert')).toHaveText('task nope not found')
  await page.getByRole('link', { name: 'Tasks' }).last().click()
  await expect(page).toHaveURL(/\/panel\/tasks$/)
})

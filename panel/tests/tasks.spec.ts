// Owned by the tasks feature (TasksPage, TaskPage, BriefTab, ComputeTab).
// TODO(ADR 0008): smoke tests left by the foundation pass; the tasks feature
// rewrites them with its pages.
import { expect, test, type APIRequestContext } from '@playwright/test'

// All tests share one mock, so flows that change a task use their own.
async function createTask(request: APIRequestContext, title: string, extra: Record<string, unknown> = {}) {
  const response = await request.post('/api/tasks', {
    data: { title, brief: 'Do it.', assignee: 'claude_code', for: '', ...extra },
  })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { name: string; title: string; status: string; sandboxes: { status: string }[] }
}

function unique(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`
}

test('tasks list shows each task with its title and backend status', async ({ page }) => {
  await page.goto('/panel/tasks')
  await expect(page.getByTestId('page-title')).toHaveText('Tasks')
  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()

  const statuses: [string, string][] = [
    ['Review 42', 'running'],
    ['Flaky CI', 'open'],
    ['Docs pass', 'done'],
    ['Boot fail', 'open'],
  ]
  for (const [title, status] of statuses) {
    const row = page.getByRole('listitem').filter({ hasText: title })
    await expect(row.getByRole('link', { name: title })).toBeVisible()
    await expect(row.locator('[data-status]')).toHaveText(status)
  }
})

test('a task opens on its brief tab, with compute and messages tabs', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/brief$/)
  await expect(page.getByRole('heading', { level: 2, name: 'Review 42' })).toBeVisible()
  await expect(page.getByTestId('page-title')).toHaveText('Review 42')
  await expect(page.getByTestId('task-header').locator('[data-status]')).toHaveText('running')
  await expect(page.getByRole('heading', { name: 'BriefTab' })).toBeVisible()

  const tabs = page.getByRole('navigation', { name: 'Task' })
  await expect(tabs.getByRole('link')).toHaveText(['Brief', 'Compute', 'Messages'])
  await tabs.getByRole('link', { name: 'Compute' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/compute$/)
  await expect(page.getByRole('heading', { name: 'ComputeTab' })).toBeVisible()
})

test('delete asks for a second click in place', async ({ page, request }) => {
  const task = await createTask(request, unique('Delete me'))
  await page.goto('/panel/tasks')
  const row = page.getByRole('listitem').filter({ hasText: task.name })

  await row.getByRole('button', { name: 'Delete' }).click()
  await row.getByRole('button', { name: 'Delete?' }).click()
  await expect(row).toHaveCount(0)
})

test('an unknown task shows the backend error verbatim', async ({ page }) => {
  await page.goto('/panel/tasks/nope/brief')
  await expect(page.getByRole('alert')).toHaveText('task nope not found')
})

test('creating returns at once; allocation starts, then runs', async ({ request }) => {
  const task = await createTask(request, unique('Alloc'), { sandbox: { image: 'agc-agent', command: 'claude' } })
  expect(task.status).toBe('open')
  expect(task.sandboxes.map((b) => b.status)).toEqual(['starting'])

  const second = await request.post(`/api/tasks/${task.name}/sandboxes`, { data: { image: 'agc-agent' } })
  expect(second.status()).toBe(201)
  expect(await second.json()).toMatchObject({ name: `${task.name}-2`, task: task.name, status: 'starting' })

  await expect
    .poll(async () => (await (await request.get(`/api/tasks/${task.name}`)).json()).status, { timeout: 10_000 })
    .toBe('running')
  const terminals = await (await request.get(`/api/sandboxes/${task.name}/terminals`)).json()
  expect(terminals.terminals).toMatchObject([{ id: 'agent', command: ['claude'], running: true }])

  const done = await request.post(`/api/tasks/${task.name}/done`)
  expect((await done.json()).status).toBe('done')
})

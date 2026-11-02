// The live feed (GET /api/events): pages update without a reload, and
// tasks with notify on toast when they start and when they are done.
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

function unique(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`
}

async function createTask(request: APIRequestContext, title: string, extra: Record<string, unknown> = {}) {
  const response = await request.post('/api/tasks', {
    data: { title, brief: 'Do it.', assignee: 'claude_code', for: '', ...extra },
  })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { name: string; title: string; notify: boolean }
}

// Other tests share the mock and create tasks too, so a toast is found by
// its task.
function toast(page: Page, heading: string, task: string) {
  return page
    .locator('[data-sonner-toast]')
    .filter({ hasText: heading })
    .filter({
      has: page.locator(`[data-testid="toast-task"][data-task="${task}"]`),
    })
}

async function openLive(page: Page) {
  await page.goto('/panel/tasks')
  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
}

test('a task created through the API appears without a reload and toasts "Task started"', async ({ page, request }) => {
  await openLive(page)
  const task = await createTask(request, unique('Live start'), {
    sandbox: { image: 'agc-agent', command: 'claude' },
  })

  await expect(page.locator(`[data-task="${task.name}"]`)).toBeVisible()
  const started = toast(page, 'Task started', task.name)
  await expect(started).toBeVisible()
  await expect(started).toContainText(task.title)
  await expect(started).toContainText('claude_code (claude-opus-5-5)')
  await expect(started).toContainText(task.name)

  await started.getByRole('button', { name: 'Open' }).click()
  await expect(page).toHaveURL(new RegExp(`/panel/tasks/${task.name}/brief$`))
  await expect(page.getByTestId('notify-chip')).toBeVisible()
})

test('marking a task done through the API toasts "Task done" with the summary', async ({ page, request }) => {
  const task = await createTask(request, unique('Live done'))
  const agent = await request.post('/api/msg/join', {
    data: { name: `bot-${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`, task: task.name },
  })
  expect(agent.ok()).toBe(true)
  const { id } = (await agent.json()) as { id: string }
  const summary = 'Fixed the flaky test. ' + 'All green. '.repeat(40)
  const posted = await request.post('/api/msg/post', {
    data: { participant_id: id, channel: `#${task.name}`, body: summary },
  })
  expect(posted.ok()).toBe(true)

  await openLive(page)
  const done = await request.post(`/api/tasks/${task.name}/done`)
  expect(done.ok()).toBe(true)

  const toastDone = toast(page, 'Task done', task.name)
  await expect(toastDone).toBeVisible()
  await expect(toastDone).toContainText(task.title)
  const text = toastDone.getByTestId('toast-summary')
  await expect(text).toContainText('Fixed the flaky test.')
  // 240 characters and an ellipsis until "Show more".
  expect((await text.textContent())?.length).toBe(241)
  await toastDone.getByRole('button', { name: 'Show more' }).click()
  await expect(text).toHaveText(summary)
})

test('a task with notify false shows no toast', async ({ page, request }) => {
  await openLive(page)
  const quiet = await createTask(request, unique('Live quiet'), { notify: false })
  expect(quiet.notify).toBe(false)
  // The card arriving proves the event did; no toast came with it.
  await expect(page.locator(`[data-task="${quiet.name}"]`)).toBeVisible()
  await request.post(`/api/tasks/${quiet.name}/done`)
  await expect(page.locator(`[data-task="${quiet.name}"]`).getByText('done', { exact: true })).toBeVisible()
  await page.waitForTimeout(500)
  await expect(page.locator(`[data-testid="toast-task"][data-task="${quiet.name}"]`)).toHaveCount(0)
})

test('the speaker toggle persists', async ({ page }) => {
  await page.goto('/panel/tasks')
  const toggle = page.getByTestId('sound-toggle')
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'false')
  expect(await page.evaluate(() => localStorage.getItem('agc.panel.sound'))).toBe('off')

  await page.reload()
  await expect(page.getByTestId('sound-toggle')).toHaveAttribute('aria-pressed', 'false')
  await page.getByTestId('sound-toggle').click()
  await expect(page.getByTestId('sound-toggle')).toHaveAttribute('aria-pressed', 'true')
})

test('the create dialog sends notify from its switch', async ({ page }) => {
  await page.goto('/panel/tasks')
  await page.getByRole('button', { name: 'New task' }).click()
  const dialog = page.getByRole('dialog')
  const notify = dialog.locator('[data-notify]')
  await expect(notify).toHaveAttribute('aria-checked', 'true')
  await notify.click()
  await expect(notify).toHaveAttribute('aria-checked', 'false')
  await dialog.getByRole('textbox', { name: 'Title' }).fill(unique('Live switch'))
  await dialog.locator('[data-compute]').click()
  await dialog.locator('[data-brief]').fill('Do it.')
  const created = page.waitForResponse((r) => r.url().endsWith('/api/tasks') && r.request().method() === 'POST')
  await dialog.getByRole('button', { name: 'Create task' }).click()
  const body = (await (await created).request().postDataJSON()) as { notify: boolean }
  expect(body.notify).toBe(false)
  await expect(page.getByTestId('task-header')).toBeVisible()
  await expect(page.getByTestId('notify-chip')).toHaveCount(0)
})

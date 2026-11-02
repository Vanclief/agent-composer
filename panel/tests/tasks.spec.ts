// Owned by the tasks feature (TasksPage, TaskPage, BriefTab, ComputeTab).
import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test'

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

function card(page: Page, name: string) {
  return page.locator(`[data-task="${name}"]`)
}

// One chip of a single-choice chip row (a radiogroup named after it).
function chip(form: Locator, group: string, text: string) {
  return form.getByRole('radiogroup', { name: group }).getByRole('radio', { name: text, exact: true })
}

test('the list puts tasks in review first, then open, running and done, newest first', async ({ page }) => {
  await page.goto('/panel/tasks')
  await expect(page.getByTestId('page-title')).toHaveText('Tasks')
  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()
  await expect(card(page, 'review-42')).toBeVisible()

  // Other tests add their own tasks to the shared mock; only the seeded
  // ones have a fixed place.
  const seeded = ['review-42', 'boot-fail', 'flaky-ci', 'docs-pass']
  const order = await page.locator('[data-task]').evaluateAll((els) => els.map((el) => el.getAttribute('data-task')))
  expect(order.filter((name) => seeded.includes(name ?? ''))).toEqual(seeded)

  const statuses: [string, string][] = [
    ['review-42', 'review'],
    ['boot-fail', 'open'],
    ['flaky-ci', 'open'],
    ['docs-pass', 'done'],
  ]
  for (const [name, status] of statuses) {
    await expect(card(page, name).getByTestId('task-status')).toHaveText(status)
  }
})

test('each card shows assignee, manager, owner, for and its sandboxes', async ({ page }) => {
  await page.goto('/panel/tasks')

  const review = card(page, 'review-42')
  await expect(review.getByRole('link', { name: 'Review 42' })).toBeVisible()
  await expect(review.getByTestId('assignee')).toHaveText('claude_code · claude-opus-5-5')
  await expect(review.getByTestId('assignee')).toHaveAttribute('data-kind', 'harness')
  await expect(review.getByTestId('manager')).toHaveText('@agent-composer-da')
  await expect(review.getByTestId('manager').locator('[title]')).toHaveAttribute('title', 'manager agent-composer-da')
  await expect(review.getByTestId('owner')).toHaveText('@franco')
  await expect(review.getByTestId('owner').locator('[title]')).toHaveAttribute('title', 'owner franco')
  const forLink = review.getByTestId('for')
  await expect(forLink).toHaveText('attach-rulesets-to-task-specs')
  await expect(forLink).toHaveAttribute('href', '/panel/think/actions/attach-rulesets-to-task-specs')
  await expect(review.getByTestId('sandbox-count')).toHaveText('2')
  await expect(review.getByTestId('sandbox-summary').locator('[data-status]')).toHaveText('running')

  const flaky = card(page, 'flaky-ci')
  await expect(flaky.getByTestId('assignee')).toHaveText('franco')
  await expect(flaky.getByTestId('assignee')).toHaveAttribute('data-kind', 'human')
  await expect(flaky.getByTestId('sandbox-summary')).toHaveText('no sandbox')

  await expect(card(page, 'docs-pass').getByTestId('assignee')).toHaveText('codex_cli · gpt-6.1-sol')
  await expect(card(page, 'boot-fail').getByTestId('sandbox-summary').locator('[data-status]')).toHaveText('failed')

  await forLink.click()
  await expect(page).toHaveURL(/\/panel\/think\/actions\/attach-rulesets-to-task-specs$/)
})

test('a task opens on its brief tab with its chips, and has compute and messages tabs', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/brief$/)
  await expect(page.getByRole('heading', { level: 2, name: 'Review 42' })).toBeVisible()
  await expect(page.getByTestId('page-title')).toHaveText('Review 42')
  const header = page.getByTestId('task-header')
  await expect(header.locator('[data-status]')).toHaveText('review')
  await expect(header.getByTestId('delivered-chip')).toHaveText(/^Delivered \d+m ago$/)
  await expect(header.getByTestId('assignee')).toHaveText('claude_code · claude-opus-5-5')
  await expect(header.getByTestId('manager')).toHaveText('@agent-composer-da')
  await expect(header.getByTestId('owner')).toHaveText('@franco')
  await expect(header.getByTestId('for')).toHaveText('attach-rulesets-to-task-specs')
  await expect(header.getByRole('button', { name: 'Accept', exact: true })).toBeVisible()
  await expect(header.getByRole('button', { name: 'Reject…' })).toBeVisible()

  const tabs = page.getByRole('navigation', { name: 'Task' })
  await expect(tabs.getByRole('link')).toHaveText(['Brief', 'Compute', 'Messages'])
  await tabs.getByRole('link', { name: 'Compute' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/compute$/)
  // Newest first: the running sandbox, then the older stopped one.
  const rows = page.locator('[data-sandbox]')
  await expect(rows.getByTestId('sandbox-name')).toHaveText(['review-42', 'review-42-2'])
  const running = page.locator('[data-sandbox="review-42"]')
  await expect(running.locator('[data-status]')).toHaveText('running')
  await expect(running.getByRole('button', { name: 'Stop' })).toBeVisible()
  await expect(running.getByRole('button', { name: 'Start' })).toHaveCount(0)
  await expect(running.getByRole('link', { name: 'Open' })).toHaveAttribute('href', '/panel/compute/review-42')
  const stopped = page.locator('[data-sandbox="review-42-2"]')
  await expect(stopped.locator('[data-status]')).toHaveText('stopped')
  await expect(stopped.getByRole('button', { name: 'Start' })).toBeVisible()
})

test('the brief tab shows the brief and the instructions the agent received, verbatim', async ({ page, request }) => {
  const task = await (await request.get('/api/tasks/review-42')).json()
  await page.goto('/panel/tasks/review-42/brief')
  await expect(page.getByTestId('brief')).toHaveText(task.brief)
  const instructions = page.getByTestId('instructions')
  await expect(instructions).toHaveText(task.sandboxes[0].instructions)
  await expect(instructions).toContainText('You are claude_code (claude-opus-5-5) working for the task `review-42`')
  await expect(page.getByRole('button', { name: 'Copy' })).toBeVisible()

  await page.goto('/panel/tasks/flaky-ci/brief')
  await expect(page.getByTestId('brief')).toHaveText('Find why CI fails one run in five and fix it.')
  await expect(page.getByText('No sandbox allocated yet')).toBeVisible()
})

test('allocate compute from a saved spec adds a starting sandbox', async ({ page, request }) => {
  const task = await createTask(request, unique('Alloc ui'))
  await page.goto(`/panel/tasks/${task.name}/compute`)
  await expect(page.getByText('No sandbox allocated yet')).toBeVisible()

  await page.getByRole('button', { name: 'Allocate compute' }).click()
  const form = page.getByRole('region', { name: 'Allocate compute' })
  await chip(form, 'Saved spec', 'backend-review').click()
  // The preset fills CPUs and memory; then CPUs is changed by hand.
  await expect(chip(form, 'CPUs', '2')).toHaveAttribute('data-state', 'on')
  await expect(chip(form, 'Memory', '4 GB')).toHaveAttribute('data-state', 'on')
  await chip(form, 'CPUs', '4').click()
  await form.getByRole('button', { name: 'Allocate', exact: true }).click()

  await expect(form).toHaveCount(0)
  const row = page.locator(`[data-sandbox="${task.name}"]`)
  await expect(row.locator('[data-status]')).toHaveText('starting')
  await expect(row.getByTitle('CPUs')).toHaveText('4')
  await expect(row.getByTitle('Memory')).toHaveText('4 GB')
  await expect(row.getByTitle('Image')).toHaveText('agc-agent')

  // Deleting the sandbox says what is lost, then the task is back to none.
  await row.getByRole('button', { name: 'Delete' }).click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText(/Could not check the clone|will be lost|Nothing unpushed/)
  await dialog.getByRole('button', { name: /^Delete/ }).click()
  await expect(row).toHaveCount(0)
})

test('the brief tab lists the criteria as unchecked boxes, and hides them when there are none', async ({
  page,
  request,
}) => {
  await page.goto('/panel/tasks/review-42/brief')
  const criteria = page.getByRole('region', { name: 'Accepted when' })
  await expect(criteria.getByRole('heading', { name: 'Accepted when' })).toBeVisible()
  await expect(criteria).toContainText('agc does not check these; the manager or owner does')
  const boxes = criteria.getByRole('checkbox')
  await expect(boxes).toHaveCount(2)
  await expect(criteria.getByTestId('criterion')).toHaveText([
    '`go test ./...` passes',
    'every finding is posted in #review-42',
  ])
  for (const box of await boxes.all()) {
    await expect(box).not.toBeChecked()
  }
  // Delivered, not accepted: the summary shows, no verdict yet.
  await expect(page.getByTestId('summary')).toHaveText('Reviewed PR 42: two findings, both fixed on agc/review-42.')
  await expect(page.getByTestId('accepted')).toHaveCount(0)

  await page.goto('/panel/tasks/flaky-ci/brief')
  await expect(page.getByRole('region', { name: 'Accepted when' }).getByRole('checkbox')).toHaveCount(1)

  const plain = await createTask(request, unique('No criteria'))
  await page.goto(`/panel/tasks/${plain.name}/brief`)
  await expect(page.getByTestId('brief')).toHaveText('Do it.')
  await expect(page.getByRole('region', { name: 'Accepted when' })).toHaveCount(0)
  await expect(page.getByRole('region', { name: 'Delivery' })).toHaveCount(0)
})

test('the review status shows verbatim in its own amber pill', async ({ page }) => {
  await page.goto('/panel/tasks')
  const pill = card(page, 'review-42').getByTestId('task-status').locator('[data-status]')
  await expect(pill).toHaveText('review')
  await expect(pill).toHaveAttribute('data-status', 'review')
  await expect(pill).toHaveClass(/amber/)
})

test('a done task shows who accepted it and its summary, read-only', async ({ page }) => {
  await page.goto('/panel/tasks/docs-pass/brief')
  await expect(page.getByTestId('accepted')).toHaveText(/^Accepted by franco \d+d ago$/)
  await expect(page.getByTestId('summary')).toHaveText('Brought docs/ up to date with the REST API.')
  const header = page.getByTestId('task-header')
  await expect(header.getByRole('button', { name: 'Accept', exact: true })).toHaveCount(0)
  await expect(header.getByRole('button', { name: 'Reject…' })).toHaveCount(0)
})

test('accept says what it stops, sends by, and the task becomes done', async ({ page, request }) => {
  const task = await createTask(request, unique('Accept me'), { sandbox: { image: 'agc-agent', command: 'claude' } })
  await expect
    .poll(async () => (await (await request.get(`/api/tasks/${task.name}`)).json()).status, { timeout: 10_000 })
    .toBe('running')
  await page.goto(`/panel/tasks/${task.name}/brief`)
  const header = page.getByTestId('task-header')
  await expect(header.locator('[data-status]').first()).toHaveText('running')

  await header.getByRole('button', { name: 'Accept', exact: true }).click()
  const dialog = page.getByRole('alertdialog')
  const confirm = dialog.getByTestId('accept-confirm')
  await expect(confirm).toHaveText('Accept and stop 1 running sandbox')
  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith(`/api/tasks/${task.name}/accept`))
  await confirm.click()
  expect((await posted).postDataJSON()).toEqual({ by: 'franco' })

  await expect(header.locator('[data-status]').first()).toHaveText('done')
  await expect(page.getByTestId('accepted')).toHaveText('Accepted by franco just now')
  await expect(header.getByRole('button', { name: 'Accept', exact: true })).toHaveCount(0)
  await expect(header.getByRole('button', { name: 'Reject…' })).toHaveCount(0)
  // Accepting stopped the sandbox and kept it.
  const after = await (await request.get(`/api/tasks/${task.name}`)).json()
  expect(after.accepted_by).toBe('franco')
  expect(after.sandboxes.map((b: { status: string }) => b.status)).toEqual(['stopped'])
  await page.getByRole('navigation', { name: 'Task' }).getByRole('link', { name: 'Compute' }).click()
  await expect(page.getByRole('button', { name: 'Allocate compute: task is done' })).toBeDisabled()
})

test('accept with no running sandbox confirms with "Accept" alone', async ({ page, request }) => {
  const task = await createTask(request, unique('Accept plain'))
  await page.goto(`/panel/tasks/${task.name}`)
  await page.getByTestId('task-header').getByRole('button', { name: 'Accept', exact: true }).click()
  await expect(page.getByRole('alertdialog').getByTestId('accept-confirm')).toHaveText('Accept')
})

test('reject needs a reason, sends by and why, and returns the task to running', async ({ page, request }) => {
  const task = await createTask(request, unique('Reject me'), { sandbox: { image: 'agc-agent' } })
  await expect
    .poll(async () => (await (await request.get(`/api/tasks/${task.name}`)).json()).status, { timeout: 10_000 })
    .toBe('running')
  const delivered = await request.post(`/api/tasks/${task.name}/deliver`, { data: { summary: 'All done.' } })
  expect((await delivered.json()).status).toBe('review')

  await page.goto(`/panel/tasks/${task.name}/brief`)
  const header = page.getByTestId('task-header')
  await expect(header.locator('[data-status]').first()).toHaveText('review')
  await header.getByRole('button', { name: 'Reject…' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('button', { name: 'Reject: needs a reason' })).toBeDisabled()
  await dialog.getByRole('textbox', { name: 'Why' }).fill('The Stop icon is still hollow.')
  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith(`/api/tasks/${task.name}/reject`))
  await dialog.getByRole('button', { name: 'Reject', exact: true }).click()
  expect((await posted).postDataJSON()).toEqual({ by: 'franco', why: 'The Stop icon is still hollow.' })

  await expect(dialog).toHaveCount(0)
  await expect(header.locator('[data-status]').first()).toHaveText('running')
  await expect(header.getByTestId('delivered-chip')).toHaveCount(0)
  // The summary is kept so the history reads.
  await expect(page.getByTestId('summary')).toHaveText('All done.')
})

test('delete asks in a dialog, then returns to the list', async ({ page, request }) => {
  const task = await createTask(request, unique('Delete me'), { sandbox: { image: 'agc-agent' } })
  await page.goto(`/panel/tasks/${task.name}`)
  await page.getByTestId('task-header').getByRole('button', { name: 'Delete' }).click()

  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toContainText(`Delete task ${task.name}?`)
  await expect(dialog).toContainText('Its sandbox and their uncommitted work are deleted')
  await dialog.getByRole('button', { name: 'Delete' }).click()

  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(card(page, 'review-42')).toBeVisible()
  await expect(card(page, task.name)).toHaveCount(0)
})

test('the messages tab opens with agc addressing the manager and owner, then the instructions', async ({
  page,
  request,
}) => {
  const task = await (await request.get('/api/tasks/review-42')).json()
  await page.addInitScript(() => localStorage.setItem('agc.panel.human', `t${Date.now().toString(36)}`))
  await page.goto('/panel/tasks/review-42/messages')
  const messages = page.getByTestId('message-list').getByTestId('message')
  await expect(messages.first()).toContainText(
    '@agent-composer-da (manager), @franco (owner): task `review-42` is open: Review 42.\n' +
      'Agent: claude_code (claude-opus-5-5) on sandbox `review-42`, starting.\n' +
      "The agent's instructions are the next message. Reply here to talk to it; " +
      'it escalates to #franco when only the owner can decide. Post /done to close the task.',
  )
  await expect(messages.nth(1)).toContainText(task.sandboxes[0].instructions)
  await expect(page.getByTestId('message-list').getByTestId('author').first()).toHaveText('@agc')
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

  const done = await request.post(`/api/tasks/${task.name}/accept`, { data: {} })
  const accepted = await done.json()
  expect(accepted.status).toBe('done')
  // by defaults to the owner; both sandboxes are stopped.
  expect(accepted.accepted_by).toBe('franco')
  expect(accepted.sandboxes.map((b: { status: string }) => b.status)).toEqual(['stopped', 'stopped'])
})

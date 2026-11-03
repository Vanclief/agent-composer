// Owned by the tasks feature (TasksPage, TaskPage, TaskRail, ComputeTab).
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

test('the list puts tasks in review first, then running, open and done, newest first', async ({ page }) => {
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

test('a task opens on Messages with the facts in the rail', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42$/)
  await expect(page.getByRole('heading', { level: 2, name: 'Review 42' })).toBeVisible()
  await expect(page.getByTestId('page-title')).toHaveText('Review 42')
  const rail = page.getByTestId('task-header')
  await expect(rail.locator('[data-status]')).toHaveText('review')
  await expect(rail).toContainText('review-42 · #review-42')
  await expect(rail.getByTestId('delivered-chip')).toHaveText(/^Delivered \d+m ago$/)
  await expect(rail.getByTestId('assignee')).toHaveText('claude_code · claude-opus-5-5')
  await expect(rail.getByTestId('manager')).toHaveText('@agent-composer-da')
  await expect(rail.getByTestId('owner')).toHaveText('@franco')
  await expect(rail.getByTestId('for')).toHaveText('attach-rulesets-to-task-specs')
  await expect(rail.getByRole('button', { name: 'Accept', exact: true })).toBeVisible()
  await expect(rail.getByRole('button', { name: 'Reject…' })).toBeVisible()
  // The rail is on the right of the box.
  const box = page.getByRole('region', { name: 'Task work' })
  const railBox = (await rail.boundingBox())!
  const workBox = (await box.boundingBox())!
  expect(railBox.x).toBeGreaterThan(workBox.x + workBox.width - 1)
  expect(railBox.width).toBeGreaterThan(340)
  expect(railBox.width).toBeLessThan(380)

  // Messages is the default view; the old tab bar is gone.
  const view = page.getByRole('navigation', { name: 'Task view' })
  await expect(view.getByRole('link')).toHaveText(['Compute2', 'Messages'])
  await expect(view.getByRole('link', { name: 'Messages' })).toHaveAttribute('aria-current', 'page')
  await expect(box.getByTestId('message-list')).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Task', exact: true })).toHaveCount(0)
})

test('the segmented control switches compute and messages, and the URL follows', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  const view = page.getByRole('navigation', { name: 'Task view' })
  await view.getByRole('link', { name: 'Compute' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/compute$/)
  await expect(view.getByRole('link', { name: 'Compute' })).toHaveAttribute('aria-current', 'page')
  // Newest first, as rows: the running sandbox, then the older stopped one.
  const rows = page.locator('[data-sandbox]')
  await expect(rows.getByTestId('sandbox-name')).toHaveText(['review-42', 'review-42-2'])
  const running = page.locator('[data-sandbox="review-42"]')
  await expect(running.locator('[data-status]')).toHaveText('running')
  await expect(running).toContainText('agc-agent')
  await expect(running.getByRole('link', { name: 'Open' })).toHaveAttribute('href', '/panel/compute/review-42')
  await expect(page.locator('[data-sandbox="review-42-2"]').locator('[data-status]')).toHaveText('stopped')

  // The newest is selected: its parts, work and terminals are below the rows.
  await expect(running).toHaveAttribute('aria-current', 'true')
  let parts = page.getByTestId('sandbox-parts')
  await expect(parts).toHaveAttribute('data-sandbox-parts', 'review-42')
  await expect(parts.getByTestId('work')).toHaveText('2 unpushed · uncommitted changes')
  await expect(parts.getByRole('button', { name: 'Stop' })).toBeVisible()
  await expect(parts.getByRole('button', { name: 'Start' })).toHaveCount(0)
  await expect(parts.getByRole('list', { name: 'Terminals' })).toBeVisible()

  // Selecting the stopped one is kept in the URL.
  await page.locator('[data-sandbox="review-42-2"]').getByTestId('sandbox-name').click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/compute\?sandbox=review-42-2$/)
  parts = page.getByTestId('sandbox-parts')
  await expect(parts).toHaveAttribute('data-sandbox-parts', 'review-42-2')
  await expect(parts.getByRole('button', { name: 'Start' })).toBeVisible()
  await expect(parts.getByText('Sandbox stopped')).toBeVisible()

  await view.getByRole('link', { name: 'Messages' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/messages$/)
  await expect(page.getByTestId('message-list')).toBeVisible()
  await expect(page.getByTestId('task-header').locator('[data-status]')).toHaveText('review')
})

test('/brief redirects to the task, which opens on Messages', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42$/)
  await expect(
    page.getByRole('navigation', { name: 'Task view' }).getByRole('link', { name: 'Messages' }),
  ).toHaveAttribute('aria-current', 'page')
})

test('on a narrow screen the rail is a card above the box, details folded', async ({ page }) => {
  await page.setViewportSize({ width: 1000, height: 900 })
  await page.goto('/panel/tasks/review-42/compute')
  const rail = page.getByTestId('task-header')
  const box = page.getByRole('region', { name: 'Task work' })
  const railBox = (await rail.boundingBox())!
  const workBox = (await box.boundingBox())!
  expect(railBox.y + railBox.height).toBeLessThanOrEqual(workBox.y)
  await expect(page.getByRole('navigation', { name: 'Task view' })).toBeVisible()
  await expect(rail.getByTestId('brief')).toBeHidden()
  await rail.getByRole('button', { name: /^Details/ }).click()
  await expect(rail.getByTestId('brief')).toBeVisible()
})

test('the rail shows the brief and the instructions the agent received, verbatim', async ({ page, request }) => {
  const task = await (await request.get('/api/tasks/review-42')).json()
  await page.goto('/panel/tasks/review-42')
  await expect(page.getByTestId('brief')).toHaveText(task.brief)
  await page.getByRole('button', { name: 'TASK.md of review-42' }).click()
  const instructions = page.getByTestId('instructions')
  await expect(instructions).toHaveText(task.sandboxes[0].instructions)
  await expect(instructions).toContainText('You are claude_code (claude-opus-5-5) working for the task `review-42`')
  await expect(page.getByTestId('task-header').getByRole('button', { name: 'Copy', exact: true })).toBeVisible()

  await page.goto('/panel/tasks/flaky-ci')
  await expect(page.getByTestId('brief')).toHaveText('Find why CI fails one run in five and fix it.')
  await expect(page.getByTestId('task-header').getByText('No sandbox allocated yet')).toBeVisible()
})

test('a long brief is clamped until Show more', async ({ page, request }) => {
  const brief = Array.from({ length: 20 }, (_, i) => `Line ${i + 1} of the brief.`).join('\n')
  const task = await createTask(request, unique('Long brief'), { brief })
  await page.goto(`/panel/tasks/${task.name}`)
  const shown = page.getByTestId('brief')
  await expect(shown).toHaveText(brief)
  const clamped = (await shown.boundingBox())!.height
  await page.getByRole('button', { name: 'Show more' }).click()
  await expect(page.getByRole('button', { name: 'Show less' })).toBeVisible()
  expect((await shown.boundingBox())!.height).toBeGreaterThan(clamped * 2)
})

test('allocate compute from a saved spec adds a starting sandbox', async ({ page, request }) => {
  const task = await createTask(request, unique('Alloc ui'))
  await page.goto(`/panel/tasks/${task.name}/compute`)
  await expect(page.getByRole('region', { name: 'Task work' }).getByText('No sandbox allocated yet')).toBeVisible()

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
  await expect(row).toContainText('agc-agent')
  const parts = page.getByTestId('sandbox-parts')
  await expect(parts).toHaveAttribute('data-sandbox-parts', task.name)
  await expect(parts.getByTitle('CPUs')).toHaveText('4')
  await expect(parts.getByTitle('Memory')).toHaveText('4 GB')
  await expect(parts.getByTitle('Image')).toHaveText('agc-agent')

  // Deleting the sandbox says what is lost, then the task is back to none.
  await parts.getByRole('button', { name: 'Delete' }).click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog.getByTestId('lost-work')).toHaveText(/Could not check the clone|will be lost|Nothing unpushed/)
  await dialog.getByRole('button', { name: /^Delete/ }).click()
  await expect(row).toHaveCount(0)
  await expect(page.getByRole('region', { name: 'Task work' }).getByText('No sandbox allocated yet')).toBeVisible()
})

test('the rail lists the criteria as unchecked boxes until delivered, and hides them when there are none', async ({
  page,
  request,
}) => {
  await page.goto('/panel/tasks/flaky-ci')
  const criteria = page.getByRole('region', { name: 'Accepted when' })
  await expect(criteria.getByRole('heading', { name: 'Accepted when' })).toBeVisible()
  await expect(criteria).toContainText('agc does not check these; the manager or owner does')
  const boxes = criteria.getByRole('checkbox')
  await expect(boxes).toHaveCount(1)
  await expect(criteria.getByTestId('criterion')).toHaveText(['CI passes ten runs in a row'])
  await expect(boxes).not.toBeChecked()
  await expect(page.getByRole('region', { name: 'Review' })).toHaveCount(0)

  // Delivered: the Review section takes the criteria; the summary shows, no
  // verdict yet.
  await page.goto('/panel/tasks/review-42')
  await expect(page.getByRole('region', { name: 'Accepted when' })).toHaveCount(0)
  await expect(page.getByTestId('summary')).toHaveText('Reviewed PR 42: two findings, both fixed on agc/review-42.')
  await expect(page.getByTestId('accepted')).toHaveCount(0)

  const plain = await createTask(request, unique('No criteria'))
  await page.goto(`/panel/tasks/${plain.name}`)
  await expect(page.getByTestId('brief')).toHaveText('Do it.')
  await expect(page.getByRole('region', { name: 'Accepted when' })).toHaveCount(0)
  await expect(page.getByRole('region', { name: 'Delivery' })).toHaveCount(0)
})

// ---------- review: what the reviewer accepts ----------

test('review lists each criterion with the report answer as a light, in order', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  const review = page.getByRole('region', { name: 'Review' })
  const rows = review.getByTestId('criterion')
  await expect(rows).toHaveCount(3)
  await expect(rows.nth(0)).toHaveAttribute('data-met', 'yes')
  await expect(rows.nth(1)).toHaveAttribute('data-met', 'partly')
  await expect(rows.nth(2)).toHaveAttribute('data-met', 'no')
  await expect(rows.nth(0)).toContainText('`go test ./...` passes')
  await expect(rows.nth(0)).toContainText('All packages pass on agc/review-42.')
  await expect(rows.nth(2).locator('[data-light="no"]')).toBeVisible()
  await expect(review.getByTestId('empty-light')).toHaveCount(0)

  // The section sits above the brief, and the summary right under it.
  const reviewBox = (await review.boundingBox())!
  const summaryBox = (await page.getByTestId('summary').boundingBox())!
  const briefBox = (await page.getByTestId('brief').boundingBox())!
  expect(reviewBox.y + reviewBox.height).toBeLessThanOrEqual(summaryBox.y)
  expect(summaryBox.y).toBeLessThan(briefBox.y)

  // Checks, left out: one row each, the failed check lit red.
  await expect(review.getByTestId('check')).toHaveCount(2)
  await expect(review.locator('[data-testid="check"][data-passed="false"] [data-light="no"]')).toHaveCount(1)
  await expect(review.getByTestId('check').nth(1)).toContainText('go vet ./...')
  await expect(review.getByTestId('left-out')).toHaveText(['The flaky third failureDid not reproduce in 20 runs.'])
})

test('review links the branch to the GitHub compare and shows the short sha', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  const branch = page.getByTestId('changes-branch')
  await expect(branch).toHaveText('agc/review-42')
  await expect(branch).toHaveAttribute(
    'href',
    'https://github.com/vanclief/agent-composer/compare/master...agc/review-42',
  )
  await expect(branch).toHaveAttribute('target', '_blank')
  await expect(page.getByTestId('changes-sha')).toHaveText('3f9c2ab')
  await expect(page.getByTestId('changes-files')).toHaveText('4 files')
})

test('needs you rows carry the attention mark', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  const needs = page.getByRole('region', { name: 'Review' }).getByTestId('need')
  await expect(needs).toHaveText(['Approve PR 42 on GitHub', 'Decide whether the vet warning blocks the merge'])
  for (const need of await needs.all()) {
    await expect(need.getByTestId('attention-mark')).toHaveCount(1)
  }
})

test('a report with a no puts a dim not-met count next to Accept, which stays enabled', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  const header = page.getByTestId('task-header')
  await expect(header.getByTestId('not-met')).toHaveText('1 not met')
  await expect(header.getByRole('button', { name: 'Accept', exact: true })).toBeEnabled()
})

test('with no report every criterion shows an empty slot and no report parts', async ({ page, request }) => {
  // A done fixture delivered before reports: its criterion has a slot.
  await page.goto('/panel/tasks/docs-pass')
  const done = page.getByRole('region', { name: 'Review' })
  await expect(done.getByTestId('criterion')).toHaveText(['every route in docs/openapi.yaml is described in docs/'])
  await expect(done.getByTestId('empty-light')).toHaveCount(1)

  const task = await createTask(request, unique('No report'), { accept: ['one thing', 'another thing'] })
  const delivered = await request.post(`/api/tasks/${task.name}/deliver`, { data: { summary: 'Done.' } })
  expect((await delivered.json()).report).toBeNull()
  await page.goto(`/panel/tasks/${task.name}`)
  const review = page.getByRole('region', { name: 'Review' })
  const rows = review.getByTestId('criterion')
  await expect(rows).toHaveText(['one thing', 'another thing'])
  await expect(review.getByTestId('empty-light')).toHaveCount(2)
  await expect(rows.first()).not.toHaveAttribute('data-met')
  await expect(review.getByTestId('changes')).toHaveCount(0)
  await expect(review.getByTestId('need')).toHaveCount(0)
  await expect(page.getByTestId('not-met')).toHaveCount(0)
})

test('a delivered report with two no answers counts both', async ({ page, request }) => {
  const task = await createTask(request, unique('Two no'), { accept: ['a', 'b', 'c'] })
  const report = {
    criteria: [
      { met: 'no', note: '' },
      { met: 'yes', note: '' },
      { met: 'no', note: '' },
    ],
    checks: [],
    left_out: [],
    needs: [],
  }
  await request.post(`/api/tasks/${task.name}/deliver`, { data: { summary: 'Done.', report } })
  await page.goto(`/panel/tasks/${task.name}`)
  await expect(page.getByTestId('not-met')).toHaveText('2 not met')
  // Empty lists show nothing, not even a label.
  const review = page.getByRole('region', { name: 'Review' })
  await expect(review.getByRole('heading', { name: 'Checks' })).toHaveCount(0)
  await expect(review.getByRole('heading', { name: 'Needs you' })).toHaveCount(0)
})

test('the review status shows verbatim in its own yellow pill', async ({ page }) => {
  await page.goto('/panel/tasks')
  const pill = card(page, 'review-42').getByTestId('task-status').locator('[data-status]')
  await expect(pill).toHaveText('review')
  await expect(pill).toHaveAttribute('data-status', 'review')
  await expect(pill).toHaveClass(/yellow/)
})

test('a done task shows who accepted it and its summary, read-only', async ({ page }) => {
  await page.goto('/panel/tasks/docs-pass')
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
  await page.goto(`/panel/tasks/${task.name}`)
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
  await page.getByRole('navigation', { name: 'Task view' }).getByRole('link', { name: 'Compute' }).click()
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

  await page.goto(`/panel/tasks/${task.name}`)
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
  await page.getByTestId('task-header').getByRole('button', { name: 'Delete', exact: true }).click()

  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toContainText(`Delete task ${task.name}?`)
  await expect(dialog).toContainText('Its sandbox and their uncommitted work are deleted')
  await dialog.getByRole('button', { name: 'Delete' }).click()

  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(card(page, 'review-42')).toBeVisible()
  await expect(card(page, task.name)).toHaveCount(0)
})

test('messages open with agc addressing the manager and owner, then the instructions', async ({ page, request }) => {
  const task = await (await request.get('/api/tasks/review-42')).json()
  await page.addInitScript(() => localStorage.setItem('agc.panel.human', `t${Date.now().toString(36)}`))
  await page.goto('/panel/tasks/review-42/messages')
  const messages = page.getByTestId('message-list').getByTestId('message')
  // Bodies render as markdown: `code` loses its backticks.
  await expect(messages.first()).toContainText(
    '@agent-composer-da (manager), @franco (owner): task review-42 is open: Review 42.\n' +
      'Agent: claude_code (claude-opus-5-5) on sandbox review-42, starting.\n' +
      "The agent's instructions are the next message. Reply here to talk to it;\n" +
      'its manager raises to franco what only the owner can decide.\n' +
      'Post /accept to accept the task, /reject <why> to send it back.',
  )
  // The instructions collapse to one line until expanded.
  await messages.nth(1).getByRole('button', { name: 'Instructions' }).click()
  await expect(messages.nth(1)).toContainText(`You are ${task.assignee} (${task.model}) working for the task`)
  await expect(page.getByTestId('message-list').getByTestId('author').first()).toHaveText('@agc')
})

test('an unknown task shows the backend error verbatim', async ({ page }) => {
  await page.goto('/panel/tasks/nope')
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

// ---------- attention is personal ----------

function uniqueName(prefix: string) {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`
}

async function signIn(page: Page, name: string) {
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), name)
}

// A task delivered for review, managed by manager.
async function reviewTask(request: APIRequestContext, manager: string) {
  const task = await createTask(request, unique('Attn'), { manager, owner: 'franco' })
  const delivered = await request.post(`/api/tasks/${task.name}/deliver`, { data: { summary: 'Done.' } })
  expect(delivered.ok()).toBe(true)
  return task
}

test('a review I manage waits on me: amber mark, accept chip, counted on the Tasks badge', async ({
  page,
  request,
}) => {
  const me = uniqueName('m')
  const task = await reviewTask(request, me)
  await signIn(page, me)
  await page.goto('/panel/tasks')

  const row = card(page, task.name)
  await expect(row).toHaveAttribute('data-attention', 'me')
  await expect(row).toHaveAttribute('data-tier', 'attention')
  await expect(row.getByTestId('attention-mark')).toBeVisible()
  await expect(row.getByTestId('act')).toHaveText(['accept'])
  await expect(row.getByTestId('waits-on')).toHaveCount(0)

  // The badge counts the rows waiting on me, and only those.
  const badge = page.getByRole('navigation', { name: 'Main' }).getByTestId('tasks-badge')
  await expect
    .poll(async () => Number(await badge.textContent()) === (await page.locator('[data-attention="me"]').count()))
    .toBe(true)

  await row.getByTestId('act').click()
  await expect(page).toHaveURL(new RegExp(`/panel/tasks/${task.name}$`))
  const rail = page.getByTestId('task-header')
  await expect(rail.getByRole('button', { name: /^Accept/ })).toBeInViewport()
  await expect(rail.getByTestId('waiting-on')).toHaveAttribute('data-attention', 'me')
  await expect(rail.getByTestId('waiting-on')).toContainText('Waiting on you:')
  await expect(rail.getByTestId('waiting-on').getByTestId('act')).toHaveText('accept')
})

test('a review someone else manages shows their name, dim, and is not counted', async ({ page, request }) => {
  const me = uniqueName('m')
  const manager = uniqueName('boss')
  // The manager joined messaging, so the review waits on them, not the owner.
  expect((await request.post('/api/msg/join', { data: { name: manager, task: '' } })).ok()).toBe(true)
  const task = await reviewTask(request, manager)
  // A manager who never joined: the review waits on the owner.
  const absent = await reviewTask(request, uniqueName('ghost'))
  await signIn(page, me)
  await page.goto('/panel/tasks')

  const row = card(page, task.name)
  await expect(row).toHaveAttribute('data-attention', 'other')
  await expect(row.getByTestId('waits-on')).toHaveText(`· @${manager}`)
  await expect(row.getByTestId('act')).toHaveCount(0)
  await expect(row.getByTestId('attention-mark')).toHaveCount(0)
  await expect(row).toHaveAttribute('data-tier', 'lit')

  await expect(card(page, absent.name).getByTestId('waits-on')).toHaveText('· @franco')
  // A failed sandbox waits on the owner too.
  await expect(card(page, 'boot-fail').getByTestId('waits-on')).toHaveText('· @franco')

  // Nothing of mine here: the badge counts only the rows waiting on me.
  const badge = page.getByRole('navigation', { name: 'Main' }).getByTestId('tasks-badge')
  await expect
    .poll(async () => {
      const mine = await page.locator('[data-attention="me"]').count()
      return mine === 0 ? (await badge.count()) === 0 : Number(await badge.textContent()) === mine
    })
    .toBe(true)

  await page.goto(`/panel/tasks/${task.name}`)
  const waiting = page.getByTestId('task-header').getByTestId('waiting-on')
  await expect(waiting).toHaveText(`Waiting on @${manager}`)
  await expect(waiting).toHaveAttribute('data-attention', 'other')
})

test('an open question for me in the task channel adds an answer chip to the thread', async ({ page, request }) => {
  const me = uniqueName('m')
  const task = await reviewTask(request, me)
  const agent = await (await request.post('/api/msg/join', { data: { name: uniqueName('a'), task: task.name } })).json()
  const asked = await (
    await request.post('/api/msg/post', {
      data: { participant_id: agent.id, channel: `#${task.name}`, body: 'Squash or keep?', kind: 'question' },
    })
  ).json()
  await signIn(page, me)
  await page.goto('/panel/tasks')

  const row = card(page, task.name)
  await expect(row.getByTestId('act')).toHaveText(['accept', 'answer'])
  await row.locator('[data-act="answer"]').click()
  await expect(page).toHaveURL(new RegExp(`/panel/messages/${task.name}\\?thread=${asked.id}$`))
  await expect(page.getByTestId('thread-pane')).toContainText('Squash or keep?')
})

test("a sandbox's aliased secret reads NAME → ENV in its parts", async ({ page, request }) => {
  const task = await createTask(request, unique('Alias check'), {
    sandbox: {
      image: 'agc-agent',
      secrets: ['GITHUB_TOKEN', 'ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN@api.anthropic.com'],
    },
  })
  await page.goto(`/panel/tasks/${task.name}/compute`)
  const parts = page.getByTestId('sandbox-parts')
  await expect(parts.locator('[data-spec-secret="GITHUB_TOKEN"]')).toHaveText('GITHUB_TOKEN')
  const alias = parts.locator('[data-spec-secret="ANTHROPIC_WORK"]')
  await expect(alias).toHaveText('ANTHROPIC_WORK → CLAUDE_CODE_OAUTH_TOKEN')
  await expect(alias.locator('[title]')).toHaveAttribute(
    'title',
    'Secret ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN@api.anthropic.com, sent only to api.anthropic.com',
  )
})

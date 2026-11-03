// Following (src/features/messages/follow.ts): the Talk sidebar lists only
// channels I opened or posted in, where a person mentioned me or an agent
// asked me, my inbox and my DMs; never because agc named me; a done task's
// channel falls away. Browse lists every channel in groups; notifications
// only for what is addressed to me.
import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test'

function suffix() {
  return `${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`
}

async function joinAgent(request: APIRequestContext) {
  const response = await request.post('/api/msg/join', { data: { name: `bot-${suffix()}`, task: 'review-42' } })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { id: string; name: string }
}

// A person: joining with no task joins as a human in the mock.
async function joinPerson(request: APIRequestContext) {
  const response = await request.post('/api/msg/join', { data: { name: `p-${suffix()}`, task: '' } })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { id: string; name: string }
}

async function post(
  request: APIRequestContext,
  participant: string,
  channel: string,
  body: string,
  extra: Record<string, unknown> = {},
) {
  const response = await request.post('/api/msg/post', {
    data: { participant_id: participant, channel, body, ...extra },
  })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { id: number }
}

// agc's participant (it posted the seeded opening of #review-42).
async function agcID(request: APIRequestContext) {
  const response = await request.get('/api/msg/participants?channel=%23review-42')
  const { participants } = (await response.json()) as { participants: { id: string; name: string }[] }
  const agc = participants.find((p) => p.name === 'agc')
  expect(agc).toBeTruthy()
  return agc?.id ?? ''
}

// A task owned and managed by me.
async function createTask(request: APIRequestContext, me: string, prefix = 'quiet') {
  const name = `${prefix}-${suffix()}`
  const response = await request.post('/api/tasks', {
    data: { title: name, name, brief: 'Do it.', assignee: 'claude_code', owner: me, manager: me },
  })
  expect(response.ok()).toBe(true)
  return name
}

async function accept(request: APIRequestContext, task: string, by: string) {
  const response = await request.post(`/api/tasks/${task}/accept`, { data: { by } })
  expect(response.ok()).toBe(true)
}

function rows(page: Page) {
  return page.getByTestId('channel-rows')
}

function row(page: Page, channel: string) {
  return rows(page).locator(`[data-channel="${channel}"]`)
}

function talkBadge(page: Page) {
  return page.getByTestId('area-badge-talk').getByTestId('unread-badge')
}

// A name of its own, so no other test mentions it or posts in its inbox.
async function actAs(page: Page) {
  const me = `me-${suffix()}`
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), me)
  return me
}

// Other workers post into the shared mock, so a toast can sit over the
// top-right controls; dispatch the click instead of moving the mouse.
async function clickPastToasts(locator: Locator) {
  await expect(locator).toBeVisible()
  await locator.dispatchEvent('click')
}

async function openLive(page: Page, path: string) {
  await page.goto(path)
  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
}

test('a fresh browser shows the inbox and no task channels until one is opened', async ({ page }) => {
  await actAs(page)
  await page.goto('/panel/messages')
  await expect(page.getByTestId('inbox-row')).toBeVisible()
  await expect(page.getByTestId('browse-link')).toBeVisible()
  await expect(page.getByRole('group', { name: 'Tasks' })).toHaveCount(0)
  await expect(page.getByRole('group', { name: 'Other' })).toHaveCount(0)
  await expect(row(page, '#review-42')).toHaveCount(0)
  await expect(row(page, '#releases')).toHaveCount(0)

  await page.goto('/panel/messages/flaky-ci')
  await expect(page.getByRole('group', { name: 'Tasks' }).locator('[data-channel="#flaky-ci"]')).toBeVisible()
  await expect(page.getByTestId('follow-toggle')).toHaveText('Following')
  await expect(row(page, '#review-42')).toHaveCount(0)
  // It stays followed after a reload.
  await page.goto('/panel/messages')
  await expect(row(page, '#flaky-ci')).toBeVisible()
})

test('franco sees his DMs and the task that tagged him, nothing else', async ({ page }) => {
  await page.goto('/panel/messages')
  const direct = page.getByRole('group', { name: 'Direct' })
  const pitch = direct.locator('[data-channel="@franco/yc-pitch"]')
  await expect(pitch).toContainText('@pitcher')
  await expect(pitch).toContainText('yc-pitch')
  await expect(pitch.getByTestId('unread-badge')).toHaveText('1')
  const review = direct.locator('[data-channel="@franco/review-42"]')
  await expect(review).toContainText('@reviewer')
  await expect(review.getByTestId('unread-badge')).toHaveCount(0)
  // franco posted in #review-42 and an agent asks @franco a question there.
  await expect(page.getByRole('group', { name: 'Tasks' }).locator('[data-channel="#review-42"]')).toBeVisible()
  for (const channel of ['#docs-pass', '#boot-fail']) {
    await expect(row(page, channel)).toHaveCount(0)
  }

  await pitch.click()
  await expect(page).toHaveURL(/\/panel\/messages\/%40franco%2Fyc-pitch$/)
  await expect(page.getByTestId('page-title')).toHaveText('Direct @franco/yc-pitch')
  await expect(page.getByTestId('message-list').getByText('Draft 3 of the YC pitch')).toBeVisible()
  await expect(pitch.getByTestId('unread-badge')).toHaveCount(0)
})

test("a person's mention in an unfollowed channel follows it and toasts", async ({ page, request }) => {
  const me = await actAs(page)
  await openLive(page, '/panel/tasks')
  const channel = `#follow-${suffix()}`
  const agent = await joinPerson(request)
  const message = await post(request, agent.id, channel, `@${me} take a look`)
  await expect(page.locator(`[data-testid="toast-message"][data-id="${message.id}"]`)).toBeVisible()
  await expect(talkBadge(page)).toHaveText('1')
  await page.getByRole('link', { name: 'Channels', exact: true }).click()
  await expect(row(page, channel).getByTestId('unread-badge')).toHaveAttribute('data-mention', 'true')

  // Unfollowing drops it until the next mention.
  await row(page, channel).click()
  await clickPastToasts(page.getByTestId('follow-toggle'))
  await expect(page.getByTestId('follow-toggle')).toHaveText('Follow')
  await expect(row(page, channel)).toHaveCount(0)
  await post(request, agent.id, channel, 'plain follow-up')
  await page.waitForTimeout(500)
  await expect(row(page, channel)).toHaveCount(0)
  await page.goto('/panel/messages')
  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
  await expect(row(page, channel)).toHaveCount(0)
  await post(request, agent.id, channel, `again @${me}`)
  await expect(row(page, channel)).toBeVisible()
})

test('a plain message in an unfollowed channel changes nothing', async ({ page, request }) => {
  await actAs(page)
  await openLive(page, '/panel/messages')
  await expect(page.getByTestId('browse-link')).toBeVisible()
  const before = await rows(page).locator('[data-channel]').count()
  const channel = `#quiet-${suffix()}`
  const agent = await joinAgent(request)
  const message = await post(request, agent.id, channel, 'nothing for anyone here')
  await page.waitForTimeout(800)
  await expect(row(page, channel)).toHaveCount(0)
  await expect(rows(page).locator('[data-channel]')).toHaveCount(before)
  await expect(page.locator(`[data-testid="toast-message"][data-id="${message.id}"]`)).toHaveCount(0)
  await expect(talkBadge(page)).toHaveCount(0)

  // Browse still lists it, unfollowed.
  await page.getByTestId('browse-link').click()
  const browsed = page.locator(`[data-testid="browse-row"][data-channel="${channel}"]`)
  await expect(browsed).toBeVisible()
  await expect(browsed.getByTestId('follow-toggle')).toHaveText('Follow')
})

test('Browse lists every channel, filters by name, and the toggle follows and unfollows', async ({ page, request }) => {
  await actAs(page)
  await page.goto('/panel/messages/browse')
  await expect(page.getByTestId('page-title')).toHaveText('Browse channels')
  const { channels } = (await (await request.get('/api/msg/channels')).json()) as { channels: { channel: string }[] }
  const browse = page.getByTestId('browse-rows')
  for (const c of ['#review-42', '#releases', '#flaky-ci', '@franco/yc-pitch']) {
    await expect(browse.locator(`[data-channel="${c}"]`)).toBeVisible()
  }
  // Done tasks are collapsed until opened.
  const doneToggle = page.getByRole('group', { name: 'Done tasks' }).getByTestId('browse-group-toggle')
  await expect(doneToggle).toHaveAttribute('aria-expanded', 'false')
  await doneToggle.click()
  expect(await browse.getByTestId('browse-row').count()).toBeGreaterThanOrEqual(channels.length)
  await expect(browse.locator('[data-channel="#releases"]').getByTestId('browse-last')).toHaveText(/ago|just now/)

  await page.getByRole('textbox', { name: 'Filter channels' }).fill('rele')
  await expect(browse.getByTestId('browse-row')).toHaveCount(1)
  const releases = browse.locator('[data-channel="#releases"]')
  const toggle = releases.getByTestId('follow-toggle')
  await expect(toggle).toHaveText('Follow')
  await clickPastToasts(toggle)
  await expect(toggle).toHaveText('Following')
  await expect(row(page, '#releases')).toBeVisible()
  await clickPastToasts(toggle)
  await expect(toggle).toHaveText('Follow')
  await expect(row(page, '#releases')).toHaveCount(0)

  await releases.getByRole('link', { name: '#releases' }).click()
  await expect(page).toHaveURL(/\/panel\/messages\/releases$/)
  await expect(row(page, '#releases')).toBeVisible()
})

test("agc's opening message does not follow a new task's channel; an agent's question does", async ({
  page,
  request,
}) => {
  const me = await actAs(page)
  await openLive(page, '/panel/messages')
  const task = await createTask(request, me)
  const channel = `#${task}`
  // As tasks.OpeningMessage: agc names the manager and the owner.
  await post(request, await agcID(request), channel, `@${me} (manager), @${me} (owner): task \`${task}\` is open.`)
  const agent = await joinAgent(request)
  // An agent's plain mention is not a question for me either.
  await post(request, agent.id, channel, `@${me} started on it.`)
  await page.waitForTimeout(800)
  await expect(row(page, channel)).toHaveCount(0)
  await expect(talkBadge(page)).toHaveCount(0)
  await page.reload()
  await expect(page.getByTestId('browse-link')).toBeVisible()
  await page.waitForTimeout(500)
  await expect(row(page, channel)).toHaveCount(0)

  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
  await post(request, agent.id, channel, `@${me} should I squash?`, { kind: 'question' })
  await expect(page.getByRole('group', { name: 'Tasks' }).locator(`[data-channel="${channel}"]`)).toBeVisible()
})

test("a person's mention follows a channel; an agent's question in my inbox needs no mention", async ({
  page,
  request,
}) => {
  const me = await actAs(page)
  await openLive(page, '/panel/messages')
  const person = await joinPerson(request)
  const mentioned = `#person-${suffix()}`
  await post(request, person.id, mentioned, `@${me} have a look`)
  await expect(row(page, mentioned)).toBeVisible()
  await expect(row(page, mentioned).getByTestId('unread-badge')).toHaveAttribute('data-mention', 'true')
  // A person's message that does not mention me follows nothing.
  const plain = `#person-${suffix()}`
  await post(request, person.id, plain, 'hello everyone')
  await page.waitForTimeout(500)
  await expect(row(page, plain)).toHaveCount(0)
})

test("a done task's channel leaves the sidebar; opening it follows it again", async ({ page, request }) => {
  const me = await actAs(page)
  const task = await createTask(request, me)
  const channel = `#${task}`
  const agent = await joinAgent(request)
  await post(request, agent.id, channel, 'Working on it.')
  await openLive(page, `/panel/messages/${task}`)
  await expect(row(page, channel)).toBeVisible()
  await page.getByTestId('browse-link').click()
  await expect(page.getByTestId('page-title')).toHaveText('Browse channels')

  await accept(request, task, me)
  await expect(row(page, channel)).toHaveCount(0)
  // Browse keeps it, under Done tasks, unfollowed.
  const done = page.getByRole('group', { name: 'Done tasks' })
  await done.getByTestId('browse-group-toggle').click()
  const browsed = done.locator(`[data-testid="browse-row"][data-channel="${channel}"]`)
  await expect(browsed.getByTestId('follow-toggle')).toHaveText('Follow')
  await expect(browsed.locator('[data-status]')).toHaveText('done')

  // Opening it again follows it, and it stays.
  await browsed.getByRole('link', { name: channel }).click()
  await expect(row(page, channel)).toBeVisible()
  await page.goto('/panel/messages')
  await expect(page.getByTestId('browse-link')).toBeVisible()
  await page.waitForTimeout(800)
  await expect(row(page, channel)).toBeVisible()
})

test("a done task's channel stays while a question waits for me, and leaves once answered", async ({
  page,
  request,
}) => {
  const me = await actAs(page)
  await openLive(page, '/panel/messages')
  const task = await createTask(request, me)
  const channel = `#${task}`
  const agent = await joinAgent(request)
  const question = await post(request, agent.id, channel, `@${me} merge now or wait?`, { kind: 'question' })
  await expect(row(page, channel)).toBeVisible()
  await expect(row(page, channel).getByTestId('question-badge')).toHaveText('1')

  await accept(request, task, me)
  await page.waitForTimeout(800)
  await expect(row(page, channel)).toBeVisible()

  const person = await joinPerson(request)
  await post(request, person.id, channel, 'Merge now.', { reply_to: question.id })
  await expect(row(page, channel)).toHaveCount(0)
})

test('the Talk badge counts open questions for me and unread in followed channels, nothing else', async ({
  page,
  request,
}) => {
  const me = await actAs(page)
  const agent = await joinAgent(request)
  const person = await joinPerson(request)
  // Followed: my inbox (2 unread) and a channel a person mentioned me in (1).
  await post(request, agent.id, `#${me}`, 'one for you')
  await post(request, agent.id, `#${me}`, 'two for you')
  const mentioned = `#count-${suffix()}`
  await post(request, person.id, mentioned, `@${me} see this`)
  // Not followed: plenty of messages, none counted.
  const quiet = `#count-${suffix()}`
  for (let i = 0; i < 5; i++) {
    await post(request, agent.id, quiet, `noise ${i}`)
  }
  // The questions for me, fixed: the shared mock answers for= with every
  // question of every test.
  const questions = [1, 2].map((i) => ({
    id: 900000 + i,
    channel: quiet,
    from: 'asker',
    participant_id: 'p_asker',
    task: 'review-42',
    kind: 'question',
    body: `question ${i}?`,
    created_at: new Date().toISOString(),
    answered_by: '',
    answered_at: null,
  }))
  await page.route('**/api/msg/questions?*', (route) => route.fulfill({ json: { questions } }))
  await page.goto('/panel/tasks')

  const badges = page.getByTestId('area-badge-talk')
  await expect(badges.getByTestId('question-badge')).toHaveText('2')
  await expect(badges.getByTestId('unread-badge')).toHaveText('3')
  const inbox = page.getByTestId('inbox-row')
  await expect(inbox.getByTestId('unread-badge')).toHaveText('2')
  // Every open question for me, though both were asked outside the inbox.
  await expect(inbox.getByTestId('question-badge')).toHaveText('2')
  await expect(row(page, quiet)).toHaveCount(0)
  // Nowhere in the sidebar a channel's total message count.
  await expect(page.getByTestId('sidebar').getByText('5', { exact: true })).toHaveCount(0)
})

test('Tasks and Other list at most 8 followed channels, newest first, then "N more…" to Browse', async ({
  page,
  request,
}) => {
  const me = await actAs(page)
  const agent = await joinAgent(request)
  const tasks: string[] = []
  for (let i = 0; i < 10; i++) {
    const task = await createTask(request, me, `many${i}`)
    await post(request, agent.id, `#${task}`, `update ${i}`)
    tasks.push(`#${task}`)
  }
  const others: string[] = []
  for (let i = 0; i < 9; i++) {
    const channel = `#other${i}-${suffix()}`
    await post(request, agent.id, channel, `note ${i}`)
    others.push(channel)
  }
  const follows = Object.fromEntries([...tasks, ...others].map((c) => [c, { on: true, at: 0 }]))
  await page.addInitScript((map) => localStorage.setItem('agc.panel.follow', JSON.stringify(map)), follows)
  await page.goto('/panel/messages')

  const group = page.getByRole('group', { name: 'Tasks' })
  await expect(group.locator('[data-channel]')).toHaveCount(8)
  // Newest activity first: the last task posted in leads, the first two are cut.
  await expect(group.locator('[data-channel]').first()).toHaveAttribute('data-channel', tasks[9])
  await expect(group.locator(`[data-channel="${tasks[0]}"]`)).toHaveCount(0)
  await expect(group.getByTestId('more-row')).toHaveText('2 more…')
  const other = page.getByRole('group', { name: 'Other' })
  await expect(other.locator('[data-channel]')).toHaveCount(8)
  await expect(other.getByTestId('more-row')).toHaveText('1 more…')

  await group.getByTestId('more-row').click()
  await expect(page).toHaveURL(/\/panel\/messages\/browse$/)
})

test('Browse groups channels: questions for me, running tasks, done tasks collapsed, other', async ({
  page,
  request,
}) => {
  const me = await actAs(page)
  const agent = await joinAgent(request)
  const running = await createTask(request, me, 'run')
  await post(request, agent.id, `#${running}`, 'on it')
  const finished = await createTask(request, me, 'fin')
  await post(request, agent.id, `#${finished}`, 'done here')
  await accept(request, finished, me)
  const asked = `#ask-${suffix()}`
  await post(request, agent.id, asked, `@${me} which one?`, { kind: 'question' })
  await page.goto('/panel/messages/browse')

  const questions = page.getByRole('group', { name: 'Questions for you' })
  const askedRow = questions.locator(`[data-testid="browse-row"][data-channel="${asked}"]`)
  await expect(askedRow.getByTestId('question-badge')).toHaveText('1')
  await expect(askedRow.getByTestId('unread-badge')).toHaveText('1')
  await expect(askedRow.getByTestId('follow-toggle')).toBeVisible()

  const runningRow = page.getByRole('group', { name: 'Running tasks' }).locator(`[data-channel="#${running}"]`)
  await expect(runningRow.locator('[data-status]')).toBeVisible()
  await expect(runningRow.getByTestId('follow-toggle')).toHaveText('Follow')

  const done = page.getByRole('group', { name: 'Done tasks' })
  const toggle = done.getByTestId('browse-group-toggle')
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect(done.locator(`[data-channel="#${finished}"]`)).toHaveCount(0)
  await toggle.click()
  await expect(done.locator(`[data-channel="#${finished}"]`)).toBeVisible()

  await expect(page.getByRole('group', { name: 'Other' }).locator('[data-channel="#releases"]')).toBeVisible()
})

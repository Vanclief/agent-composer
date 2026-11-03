// Owned by the messages feature (MessagesTab, MessagesPage).
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

// Every test shares one mock, and a joined name stays held, so each test
// posts under its own name unless it means to collide.
function uniqueName(prefix = 'h') {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`
}

async function actAs(page: Page, name: string) {
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), name)
}

test('a task messages tab shows its channel with human and agent authors', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/tasks/review-42/messages')
  const list = page.getByTestId('message-list')
  for (const body of [
    'Cloned the repo and ran the tests. 3 failures in workflow/.',
    'Reproduced 2 of 3 locally. The third looks flaky.',
    'Fixed the loop node. Pushing to review-42.',
  ]) {
    await expect(list.getByText(body, { exact: true })).toBeVisible()
  }
  // franco's reply lives in the first message's thread.
  await expect(list.getByText('Look at the loop node first.')).toHaveCount(0)
  await expect(list.locator('[data-kind="agent"]', { hasText: '@reviewer' }).first()).toBeVisible()
  await expect(list.locator('[data-kind="agent"]', { hasText: '@tester' }).first()).toBeVisible()
  await list.getByTestId('thread-footer').first().click()
  const thread = page.getByTestId('thread-pane')
  await expect(thread.getByText('Look at the loop node first.')).toBeVisible()
  await expect(thread.locator('[data-kind="human"]', { hasText: '@franco' })).toBeVisible()
})

test('joining and posting shows the message in the list', async ({ page }) => {
  const name = uniqueName()
  const slug = uniqueName('c')
  await actAs(page, name)
  await page.goto(`/panel/messages/${slug}`)
  await expect(page.getByText('No messages')).toBeVisible()
  await expect(page.getByRole('button', { name: `Identity @${name}` })).toBeVisible()

  const send = page.getByRole('button', { name: 'Send: empty' })
  await expect(send).toBeDisabled()

  const box = page.getByRole('textbox', { name: `Message #${slug}` })
  await box.fill('first line')
  await box.press('Shift+Enter')
  await box.pressSequentially('second line')
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled()
  await box.press('Enter')

  const list = page.getByTestId('message-list')
  await expect(list.getByText('first line\nsecond line')).toBeVisible()
  await expect(list.locator('[data-kind="human"]', { hasText: `@${name}` })).toBeVisible()
  await expect(box).toHaveValue('')

  // The participant is kept, so a reload does not trip over its own name.
  await page.reload()
  await expect(list.getByText('first line\nsecond line')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Send: empty' })).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('a taken name shows the 409 verbatim and Join as… picks another', async ({ page }) => {
  // @reviewer is an active agent in the mock.
  await actAs(page, 'reviewer')
  await page.goto('/panel/tasks/review-42/messages')
  await expect(page.getByRole('alert')).toHaveText('name @reviewer is already in use')
  await expect(page.getByRole('button', { name: 'Send: join first' })).toBeDisabled()

  const name = uniqueName()
  await page.getByRole('button', { name: 'Join as…' }).click()
  await page.getByRole('textbox', { name: 'Name' }).fill(name)
  await page.getByRole('button', { name: 'Join', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.getByRole('button', { name: `Identity @${name}` })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Send: empty' })).toBeVisible()
})

test('messages page asks for a channel and opens one from the sidebar', async ({ page }) => {
  // Only followed channels are in the sidebar (follow.ts).
  await page.addInitScript(() =>
    localStorage.setItem(
      'agc.panel.follow',
      JSON.stringify({ '#releases': { on: true, at: 0 }, '#review-42': { on: true, at: 0 } }),
    ),
  )
  await actAs(page, uniqueName())
  await page.goto('/panel/messages')
  await expect(page.getByTestId('page-title')).toHaveText('Messages')
  await expect(page.getByText('Pick a channel on the left')).toBeVisible()
  // The sidebar owns the channel list; the page draws none of its own.
  await expect(page.getByRole('navigation', { name: 'Channels' })).toHaveCount(0)

  const nav = page.getByRole('navigation', { name: 'Main' })
  await nav.getByRole('link', { name: /^#releases\b/ }).click()
  await expect(page).toHaveURL(/\/panel\/messages\/releases$/)
  await expect(page.getByTestId('page-title')).toHaveText('Messages #releases')
  await expect(page.getByTestId('message-list').getByText('v0.4.0 tagged.')).toBeVisible()
})

test('the live feed shows a message another client posts', async ({ page, request }) => {
  const slug = uniqueName('c')
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  await expect(page.getByText('No messages')).toBeVisible()
  // The read already answered, so only the wait loop can deliver it.

  const joined = await request.post('/api/msg/join', { data: { name: uniqueName('o') } })
  expect(joined.ok()).toBeTruthy()
  const { id } = await joined.json()
  const posted = await request.post('/api/msg/post', {
    data: { participant_id: id, channel: `#${slug}`, body: 'hello from elsewhere' },
  })
  expect(posted.ok()).toBeTruthy()

  await expect(page.getByTestId('message-list').getByText('hello from elsewhere')).toBeVisible()
})

test('a long channel opens at the bottom and new messages above the fold show a pill', async ({ page, request }) => {
  const slug = uniqueName('c')
  const joined = await request.post('/api/msg/join', { data: { name: uniqueName('o') } })
  const { id } = await joined.json()
  const postAs = (body: string) =>
    request.post('/api/msg/post', { data: { participant_id: id, channel: `#${slug}`, body } })
  for (let i = 1; i <= 40; i++) {
    await postAs(`line ${i}`)
  }

  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const list = page.getByTestId('message-list')
  await expect(list.getByText('line 40', { exact: true })).toBeInViewport()

  await list.evaluate((el) => el.scrollTo({ top: 0 }))
  await expect(list.getByText('line 1', { exact: true })).toBeInViewport()
  await postAs('line 41')
  const pill = page.getByRole('button', { name: 'New messages' })
  await expect(pill).toBeVisible()
  await pill.click()
  await expect(list.getByText('line 41', { exact: true })).toBeInViewport()
  await expect(pill).toHaveCount(0)
})

test('consecutive messages from one sender collapse under one header', async ({ page, request }) => {
  const slug = uniqueName('c')
  const a = uniqueName('a')
  const b = uniqueName('b')
  const ids: Record<string, string> = {}
  for (const name of [a, b]) {
    const joined = await request.post('/api/msg/join', { data: { name } })
    ids[name] = (await joined.json()).id
  }
  const postAs = (name: string, body: string) =>
    request.post('/api/msg/post', { data: { participant_id: ids[name], channel: `#${slug}`, body } })
  await postAs(a, 'one')
  await postAs(a, 'two')
  await postAs(a, 'three')
  await postAs(b, 'four')
  await postAs(a, 'five')

  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const groups = page.getByTestId('message-list').getByTestId('message-group')
  await expect(groups).toHaveCount(3)
  const first = groups.first()
  await expect(first.getByTestId('message')).toHaveCount(3)
  await expect(first.getByTestId('author')).toHaveText([`@${a}`])
  await expect(groups.nth(1).getByTestId('author')).toHaveText(`@${b}`)
  await expect(groups.nth(2).getByTestId('author')).toHaveText(`@${a}`)

  // A following message shows its time only on hover.
  const second = first.getByTestId('message').nth(1)
  const time = second.locator('time')
  await expect(time).toHaveCSS('opacity', '0')
  await second.hover()
  await expect(time).toHaveCSS('opacity', '1')
})

test('senders show a human or a task icon, and agents link to their task', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/messages/review-42')
  const list = page.getByTestId('message-list')
  const groupOf = (name: string) =>
    list.getByTestId('message-group').filter({ has: page.getByTestId('author').filter({ hasText: name }) })

  // franco only answers in threads.
  await list.getByTestId('thread-footer').first().click()
  const franco = page
    .getByTestId('thread-pane')
    .getByTestId('message-group')
    .filter({ has: page.getByTestId('author').filter({ hasText: '@franco' }) })
  await expect(franco.locator('svg[data-concept="human"]')).toHaveCount(1)
  await expect(franco.getByRole('link')).toHaveCount(0)
  const tester = groupOf('@tester')
  await expect(tester.locator('svg[data-concept="task"]')).toHaveCount(1)
  const link = tester.getByRole('link', { name: 'review-42' })
  await expect(link).toHaveAttribute('href', '/panel/tasks/review-42')
  await link.click()
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42/)
})

test('the members strip lists distinct senders, humans first, with a count', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/messages/review-42')
  const strip = page.getByRole('group', { name: 'Members' })
  // The reserved sender agc (ADR 0009) posts with an empty task, so the
  // strip counts it with the humans until messages carry a human flag.
  await expect(strip.getByTestId('members-count')).toHaveText('2 agents · 2 humans')
  await expect(strip.locator('[title]')).toHaveText(['@agc', '@franco', '@reviewer', '@tester'])
})

// ---------- markdown, system lines, threads, questions ----------

async function joinAs(request: APIRequestContext, name = uniqueName('o'), task = '') {
  const joined = await request.post('/api/msg/join', { data: { name, task } })
  expect(joined.ok()).toBeTruthy()
  return (await joined.json()) as { id: string; name: string }
}

async function postAs(
  request: APIRequestContext,
  participant: string,
  channel: string,
  body: string,
  extra: { reply_to?: number; kind?: 'message' | 'question' } = {},
) {
  const posted = await request.post('/api/msg/post', {
    data: { participant_id: participant, channel, body, ...extra },
  })
  expect(posted.ok()).toBeTruthy()
  return (await posted.json()) as { id: number; from: string }
}

// The JSON body of the next POST /api/msg/post the page sends.
function nextPost(page: Page) {
  return page
    .waitForRequest((r) => r.url().endsWith('/api/msg/post') && r.method() === 'POST')
    .then((r) => r.postDataJSON() as { reply_to?: number; body: string })
}

test('a body renders as markdown: a code block with copy, a channel chip link, a mention', async ({
  page,
  request,
}) => {
  const slug = uniqueName('c')
  const agent = await joinAs(request)
  await postAs(
    request,
    agent.id,
    `#${slug}`,
    'Run this, then tell #releases and @someone:\n\n```sh\ngo test ./...\n```\n\nSee [the docs](https://example.com/docs).',
  )
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const list = page.getByTestId('message-list')
  const code = list.getByTestId('code-block')
  await expect(code.locator('pre')).toHaveText('go test ./...')
  await expect(code.getByRole('button', { name: 'Copy code' })).toBeAttached()
  await expect(code.locator('pre')).toHaveCSS('font-family', /mono/i)
  const chip = list.getByTestId('channel-chip')
  await expect(chip).toHaveText('releases')
  await expect(chip).toHaveAttribute('href', '/panel/messages/releases')
  await expect(list.getByTestId('mention-chip')).toHaveText('@someone')
  await expect(list.getByRole('link', { name: 'the docs' })).toHaveAttribute('target', '_blank')
  await chip.click()
  await expect(page).toHaveURL(/\/panel\/messages\/releases$/)
})

test('a long body clamps with Show more', async ({ page, request }) => {
  const slug = uniqueName('c')
  const agent = await joinAs(request)
  const lines = Array.from({ length: 30 }, (_, i) => `row ${i + 1}`).join('\n')
  await postAs(request, agent.id, `#${slug}`, lines)
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const body = page.getByTestId('message-body')
  await expect(body).toHaveAttribute('data-clamped', 'true')
  expect((await body.boundingBox())!.height).toBeLessThanOrEqual(240)
  await page.getByRole('button', { name: 'Show more' }).click()
  await expect(body).toHaveAttribute('data-clamped', 'false')
  await expect(page.getByRole('button', { name: 'Show less' })).toBeVisible()
})

test('agc posts are compact system lines; the instructions collapse and expand', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/messages/review-42')
  const list = page.getByTestId('message-list')
  const lines = list.getByTestId('system-line')
  await expect(lines).toHaveCount(2)
  // Not chat bubbles: no group, no visible author header.
  await expect(list.getByTestId('message-group').getByTestId('system-line')).toHaveCount(0)
  await expect(lines.first().getByTestId('author')).toHaveClass(/sr-only/)

  const instructions = list.locator('[data-testid="system-line"][data-system="instructions"]')
  const toggle = instructions.getByRole('button', { name: 'Instructions' })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect(instructions).not.toContainText('How this task works')
  expect((await instructions.boundingBox())!.height).toBeLessThan(30)
  await toggle.click()
  await expect(instructions.getByText('How this task works')).toBeVisible()
  await toggle.click()
  await expect(instructions).not.toContainText('How this task works')
})

test('status lines from agc carry the delivery and criteria icons', async ({ page, request }) => {
  const created = await request.post('/api/tasks', {
    data: { title: `Status ${uniqueName()}`, brief: 'b', assignee: 'claude_code', for: '' },
  })
  expect(created.ok()).toBeTruthy()
  const task = (await created.json()) as { name: string }
  expect((await request.post(`/api/tasks/${task.name}/deliver`, { data: { summary: 'all done' } })).ok()).toBeTruthy()
  expect(
    (await request.post(`/api/tasks/${task.name}/reject`, { data: { by: 'franco', why: 'no' } })).ok(),
  ).toBeTruthy()
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${task.name}`)
  const list = page.getByTestId('message-list')
  await expect(list.locator('[data-system="delivered"] [data-status-icon="delivered"]')).toBeVisible()
  await expect(list.locator('[data-system="rejected"] [data-status-icon="rejected"]')).toBeVisible()
})

test('a reply sits in its thread: the footer opens the pane and posting there sends reply_to', async ({
  page,
  request,
}) => {
  const slug = uniqueName('c')
  const agent = await joinAs(request)
  const parent = await postAs(request, agent.id, `#${slug}`, 'parent message')
  await postAs(request, agent.id, `#${slug}`, 'first reply', { reply_to: parent.id })
  await postAs(request, agent.id, `#${slug}`, 'later top level')
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const list = page.getByTestId('message-list')
  await expect(list.getByText('later top level')).toBeVisible()
  await expect(list.getByText('first reply')).toHaveCount(0)
  const footer = list.getByTestId('thread-footer')
  await expect(footer).toContainText('1 reply')

  await footer.click()
  await expect(page).toHaveURL(new RegExp(`thread=${parent.id}`))
  const pane = page.getByTestId('thread-pane')
  await expect(pane.getByText('parent message')).toBeVisible()
  await expect(pane.getByText('first reply')).toBeVisible()

  const box = pane.getByRole('textbox', { name: 'Reply in thread' })
  await expect(pane.getByRole('button', { name: 'Send: empty' })).toBeVisible()
  await box.fill('second reply')
  const sent = nextPost(page)
  await box.press('Enter')
  expect((await sent).reply_to).toBe(parent.id)
  await expect(pane.getByTestId('thread-messages').getByText('second reply')).toBeVisible()
  await expect(footer).toContainText('2 replies')
  await expect(list.getByText('second reply')).toHaveCount(0)

  await pane.getByRole('button', { name: 'Close thread' }).click()
  await expect(pane).toHaveCount(0)

  // Hover shows "Reply in thread" on any message.
  const later = list.getByTestId('message').filter({ hasText: 'later top level' })
  await later.hover()
  await later.getByRole('button', { name: 'Reply in thread' }).click()
  await expect(page.getByTestId('thread-pane').getByText('later top level')).toBeVisible()
  await expect(page.getByTestId('thread-pane')).toContainText('0 replies')
})

test('a question shows an Answer box; answering sends reply_to and the card flips to answered', async ({
  page,
  request,
}) => {
  const slug = uniqueName('c')
  const agent = await joinAs(request, uniqueName('a'), 'review-42')
  const question = await postAs(request, agent.id, `#${slug}`, 'Merge **now** or wait?', { kind: 'question' })
  const me = uniqueName()
  await actAs(page, me)
  await page.goto(`/panel/messages/${slug}`)
  const card = page.getByTestId('question-card')
  await expect(card).toHaveAttribute('data-state', 'open')
  await expect(card.getByTestId('author')).toHaveText(`@${agent.name}`)
  await expect(card.locator('strong')).toHaveText('now')
  const answer = card.getByRole('textbox', { name: `Answer @${agent.name}` })
  await expect(card.getByRole('button', { name: 'Answer: empty' })).toBeDisabled()
  await answer.fill('Wait for CI.')
  const sent = nextPost(page)
  await card.getByRole('button', { name: 'Answer', exact: true }).click()
  expect((await sent).reply_to).toBe(question.id)
  await expect(card).toHaveAttribute('data-state', 'answered')
  await expect(card.getByTestId('answered-by')).toHaveText(`Answered by @${me}`)
  await expect(card.getByTestId('answer')).toContainText('Wait for CI.')
  await expect(card.getByRole('textbox')).toHaveCount(0)
  // The answer is a reply: in the thread, not in the feed.
  await expect(card.getByTestId('thread-footer')).toContainText('1 reply')
})

test('open questions for me are pinned as "Waiting for you" and counted on the channel row', async ({
  page,
  request,
}) => {
  const slug = uniqueName('c')
  const agent = await joinAs(request, uniqueName('a'), 'review-42')
  await postAs(request, agent.id, `#${slug}`, 'First question?', { kind: 'question' })
  await postAs(request, agent.id, `#${slug}`, 'Just a note.')
  await postAs(request, agent.id, `#${slug}`, 'Second question?', { kind: 'question' })
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const strip = page.getByTestId('waiting-strip')
  await expect(strip).toHaveText(/Waiting for you: 2/)
  const row = page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: new RegExp(`^#${slug}\\b`) })
  await expect(row.getByTestId('question-badge')).toHaveText('2')
  await expect(page.getByTestId('area-badge-talk').getByTestId('question-badge')).toBeVisible()

  await strip.click()
  await expect(page.getByTestId('question-card').first()).toBeInViewport()
  const card = page.getByTestId('question-card').filter({ hasText: 'First question?' })
  await card.getByRole('textbox').fill('Yes.')
  await card.getByRole('textbox').press('Enter')
  await expect(strip).toHaveText(/Waiting for you: 1/)
  await expect(row.getByTestId('question-badge')).toHaveText('1')
})

test('the seeded questions: one waits in #review-42, one is answered', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/messages/review-42')
  await expect(page.getByTestId('waiting-strip')).toHaveText(/Waiting for you: 1/)
  const answered = page.locator('[data-testid="question-card"][data-state="answered"]')
  await expect(answered.getByTestId('answered-by')).toHaveText('Answered by @franco')
  await expect(answered.getByTestId('answer')).toContainText('Out of scope')
})

test('day separators, relative times, and the unread line at the read cursor', async ({ page, request }) => {
  const slug = uniqueName('c')
  const agent = await joinAs(request)
  const read = await postAs(request, agent.id, `#${slug}`, 'already read')
  await postAs(request, agent.id, `#${slug}`, 'new one')
  await postAs(request, agent.id, `#${slug}`, 'newer one')
  await page.addInitScript(
    ([channel, id]) => localStorage.setItem('agc.panel.read', JSON.stringify({ [channel]: id })),
    [`#${slug}`, read.id] as const,
  )
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const list = page.getByTestId('message-list')
  await expect(list.getByTestId('day-separator')).toHaveText(['Today'])
  const time = list.getByTestId('message-group').first().locator('time').first()
  await expect(time).toHaveText('just now')
  await expect(time).toHaveAttribute('title', /\d/)
  // The line sits right before the first message past the cursor.
  const items = list.locator('ol > li')
  const texts = await items.allTextContents()
  const at = texts.findIndex((t) => t.includes('Unread'))
  expect(at).toBeGreaterThan(0)
  expect(texts[at + 1]).toContain('new one')
  expect(texts[at - 1]).toContain('already read')
})

// A made-up 1x1 PNG the mock keeps and serves as a delivered shot.
const SHOT_PNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=='

test("agc's Delivered message shows its Before and After lines as labelled images", async ({ page, request }) => {
  const created = await request.post('/api/tasks', {
    data: { title: `Shots ${uniqueName('t')}`, brief: 'b', assignee: 'claude_code', manager: 'reviewer-1' },
  })
  const task = (await created.json()) as { name: string }
  const report = {
    criteria: [],
    checks: [{ name: 'npm run check', passed: true, note: '' }],
    left_out: [],
    needs: [],
    shots: [
      { label: 'before', file: '/tmp/before.png' },
      { label: 'after', file: '/tmp/after.png' },
    ],
  }
  const delivered = await request.post(`/api/tasks/${task.name}/deliver`, {
    data: { summary: 'Shots shown.', report, files: { before: SHOT_PNG, after: SHOT_PNG } },
  })
  expect(delivered.ok()).toBeTruthy()
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${task.name}`)

  const line = page.locator('[data-system="delivered"]')
  await expect(line).toContainText('Delivered: Shots shown.')
  const images = line.getByTestId('shot-images')
  await expect(images.locator('figcaption')).toHaveText(['Before', 'After'])
  for (const label of ['before', 'after']) {
    const url = `/api/tasks/${task.name}/shots/${label}.png`
    const img = images.locator(`img[src="${url}"]`)
    await expect(img).toBeVisible()
    // The root-relative url loads from the panel's own origin.
    await expect.poll(() => img.evaluate((el) => (el as unknown as { naturalWidth: number }).naturalWidth)).toBe(1)
    await expect(img.locator('xpath=..')).toHaveAttribute('target', '_blank')
  }
  await expect(line).not.toContainText(`/api/tasks/${task.name}/shots/before.png`)
})

test("a person's Before:/After: png lines are images; a missing one shows its url, other lines stay text", async ({
  page,
  request,
}) => {
  const channel = `#${uniqueName('c')}`
  const author = await joinAs(request)
  await postAs(
    request,
    author.id,
    channel,
    [
      'Look at this:',
      'Before: /api/tasks/made-up-missing/shots/before.png',
      'After: https://img.example.com/after.png',
      'Before: the old list',
      '```',
      'After: /api/tasks/made-up-missing/shots/after.png',
      '```',
    ].join('\n'),
  )
  await page.route('https://img.example.com/**', (route) =>
    route.fulfill({ status: 200, contentType: 'image/png', body: Buffer.from(SHOT_PNG, 'base64') }),
  )
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${channel.slice(1)}`)

  const message = page.getByTestId('message').filter({ hasText: 'Look at this:' })
  const images = message.getByTestId('shot-images')
  await expect(images.locator('figure')).toHaveCount(2)
  const missing = images.locator('[data-image="Before"]')
  await expect(missing).toHaveAttribute('data-broken', 'true')
  await expect(missing.getByRole('img')).toHaveCount(0)
  await expect(missing).toContainText('/api/tasks/made-up-missing/shots/before.png')
  await expect(images.locator('[data-image="After"]').getByRole('img', { name: 'After' })).toBeVisible()
  // Not an image url, and inside a code fence: text as before.
  await expect(message).toContainText('Before: the old list')
  await expect(message.getByTestId('code-block')).toContainText('After: /api/tasks/made-up-missing/shots/after.png')
})

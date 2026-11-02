// Owned by the messages feature: the question card in the reviewer's shape
// (title, Changes/Asks rows, Before/After images, a muted Diff link) and
// its Accept and Send back…
// buttons posting the ADR 0010 channel commands.
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

function uniqueName(prefix = 'h') {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`
}

async function actAs(page: Page, name: string) {
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), name)
}

async function joinAs(request: APIRequestContext, name = uniqueName('r'), task = '') {
  const joined = await request.post('/api/msg/join', { data: { name, task } })
  expect(joined.ok()).toBeTruthy()
  return (await joined.json()) as { id: string; name: string }
}

async function ask(request: APIRequestContext, participant: string, channel: string, body: string) {
  const posted = await request.post('/api/msg/post', {
    data: { participant_id: participant, channel, body, kind: 'question' },
  })
  expect(posted.ok()).toBeTruthy()
  return (await posted.json()) as { id: number }
}

// A task owned by owner, delivered, so in review.
async function taskInReview(request: APIRequestContext, owner: string) {
  const created = await request.post('/api/tasks', {
    data: { title: `Card ${uniqueName('t')}`, brief: 'b', assignee: 'claude_code', manager: 'reviewer-1', owner },
  })
  expect(created.ok()).toBeTruthy()
  const task = (await created.json()) as { name: string }
  expect((await request.post(`/api/tasks/${task.name}/deliver`, { data: { summary: 'done' } })).ok()).toBeTruthy()
  return task.name
}

function nextPost(page: Page) {
  return page
    .waitForRequest((r) => r.url().endsWith('/api/msg/post') && r.method() === 'POST')
    .then((r) => r.postDataJSON() as { reply_to?: number; body: string; channel: string })
}

const DIFF = 'https://github.com/example/repo/compare/master...agc/made-up'
// Made-up fixture images: a 1x1 PNG served by the test, never fetched.
const BEFORE = 'https://img.example.com/shots/before.png'
const AFTER = 'https://img.example.com/shots/after.png?v=2'
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
  'base64',
)

async function serveImages(page: Page) {
  await page.route('https://img.example.com/**', (route) =>
    route.fulfill({ status: 200, contentType: 'image/png', body: PNG }),
  )
}

function shaped(task: string) {
  return [
    `Accept ${task}?`,
    'Lets you accept from the card.',
    'Changes: UI: a narrower question card',
    'Asks: nothing',
    `Diff: ${DIFF}`,
  ].join('\n')
}

test('a five-line question renders a title, labelled rows and a muted Diff link, at most 72ch wide', async ({
  page,
  request,
}) => {
  const me = uniqueName()
  const task = await taskInReview(request, me)
  const reviewer = await joinAs(request)
  await ask(request, reviewer.id, `#${me}`, shaped(task))
  await actAs(page, me)
  await page.goto(`/panel/messages/${me}`)

  const card = page.getByTestId('question-card')
  const body = card.getByTestId('question-body')
  await expect(body.locator('p').first()).toHaveText(`Accept ${task}?`)
  await expect(body.getByText('Lets you accept from the card.')).toBeVisible()
  await expect(card.locator('[data-row="Changes"]')).toContainText('UI: a narrower question card')
  await expect(card.locator('[data-row="Asks"]')).toContainText('nothing')
  // Not a button: a small muted link after the rows, never the raw URL.
  await expect(card.getByRole('link', { name: 'View diff' })).toHaveCount(0)
  const diff = card.getByTestId('diff-link')
  await expect(diff).toHaveText('Diff')
  await expect(diff).toHaveAttribute('href', DIFF)
  await expect(diff).toHaveAttribute('target', '_blank')
  await expect(diff).toHaveCSS('font-size', '12px')
  await expect(card).not.toContainText(DIFF)
  await expect(card.locator('[data-row="Diff"]')).toHaveCount(0)
  const rowsBottom = (await card.locator('[data-row="Asks"]').boundingBox())!
  expect((await diff.boundingBox())!.y).toBeGreaterThan(rowsBottom.y)

  // Larger than an ordinary message (text-sm, 14px) and at most 72ch.
  await expect(body.getByText('Lets you accept from the card.')).toHaveCSS('font-size', '16px')
  const width = await card.evaluate((el) => {
    // 1ch in the card's own font: the width of a "0".
    const zero = el.ownerDocument.createElement('span')
    zero.textContent = '0'
    zero.style.position = 'absolute'
    el.appendChild(zero)
    const ch = zero.getBoundingClientRect().width
    zero.remove()
    return {
      width: el.getBoundingClientRect().width,
      max: parseFloat(el.ownerDocument.defaultView!.getComputedStyle(el).maxWidth),
      ch,
    }
  })
  expect(Math.abs(width.max - 72 * width.ch)).toBeLessThan(2)
  expect(width.width).toBeLessThanOrEqual(width.max + 0.5)
  const list = await page.getByTestId('message-list').boundingBox()
  expect(width.width).toBeLessThan(list!.width - 100)
  await expect(card.getByTestId('author')).toHaveText(`@${reviewer.name}`)
})

test('Accept posts /accept <task> as a reply and the card shows the backend state', async ({ page, request }) => {
  const me = uniqueName()
  const task = await taskInReview(request, me)
  const reviewer = await joinAs(request)
  const question = await ask(request, reviewer.id, `#${me}`, shaped(task))
  await actAs(page, me)
  await page.goto(`/panel/messages/${me}`)

  const card = page.getByTestId('question-card')
  await expect(card.getByTestId('accept-actions').locator('[data-status="review"]')).toBeVisible()
  const sent = nextPost(page)
  await card.getByRole('button', { name: 'Accept', exact: true }).click()
  const post = await sent
  expect(post).toMatchObject({ body: `/accept ${task}`, reply_to: question.id, channel: `#${me}` })

  const state = card.getByTestId('accept-state')
  await expect(state.locator('[data-status="done"]')).toBeVisible()
  await expect(state.getByTestId('accepted-by')).toHaveText(`Accepted by @${me}`)
  await expect(card.getByRole('button', { name: /^Accept/ })).toHaveCount(0)
  await expect(card.getByRole('button', { name: 'Send back…' })).toHaveCount(0)
  // The answer box stays for free text until answered; the command answered it.
  await expect(card).toHaveAttribute('data-state', 'answered')
})

test('Send back… needs feedback, posts /reject <feedback> in #<task> and answers the question', async ({
  page,
  request,
}) => {
  const me = uniqueName()
  const task = await taskInReview(request, me)
  const reviewer = await joinAs(request)
  const question = await ask(request, reviewer.id, `#${me}`, shaped(task))
  await actAs(page, me)
  await page.goto(`/panel/messages/${me}`)

  const card = page.getByTestId('question-card')
  await expect(card.getByRole('button', { name: /^Reject/ })).toHaveCount(0)
  await card.getByRole('button', { name: 'Send back…' }).click()
  await expect(card.getByRole('button', { name: 'Send back: write feedback' })).toBeDisabled()
  await card.getByRole('textbox', { name: `Feedback for ${task}` }).fill('the card is too wide')
  const posts: { reply_to?: number; body: string; channel: string }[] = []
  page.on('request', (r) => {
    if (r.url().endsWith('/api/msg/post') && r.method() === 'POST') {
      posts.push(r.postDataJSON())
    }
  })
  await card.getByRole('button', { name: 'Send back', exact: true }).click()

  // Rejected: no longer in review, status verbatim from the tasks query.
  const state = card.getByTestId('accept-state')
  await expect(state.locator('[data-status="open"]')).toBeVisible()
  await expect(card.getByRole('button', { name: 'Send back…' })).toHaveCount(0)
  // The same feedback answers the inbox question.
  await expect(card).toHaveAttribute('data-state', 'answered')
  await expect(card.getByTestId('answer')).toContainText('the card is too wide')
  expect(posts).toEqual([
    expect.objectContaining({ channel: `#${task}`, body: '/reject the card is too wide' }),
    expect.objectContaining({ channel: `#${me}`, body: 'the card is too wide', reply_to: question.id }),
  ])
  expect(posts[0].reply_to).toBeUndefined()
  const rejected = await request.get(`/api/msg/read?channel=${encodeURIComponent('#' + task)}`)
  const bodies = ((await rejected.json()) as { messages: { body: string }[] }).messages.map((m) => m.body)
  expect(bodies).toContain(`Rejected by ${me}: the card is too wide`)
})

test('Before and After image rows render side by side and open full size; other values stay text', async ({
  page,
  request,
}) => {
  const me = uniqueName()
  const slug = uniqueName('c')
  const reviewer = await joinAs(request)
  await ask(
    request,
    reviewer.id,
    `#${slug}`,
    ['Accept made-up-task?', 'Changes: UI: new card', `Before: ${BEFORE}`, `After: ${AFTER}`, `Diff: ${DIFF}`].join(
      '\n',
    ),
  )
  await ask(
    request,
    reviewer.id,
    `#${slug}`,
    ['Accept other-made-up?', 'Before: the old list', 'After: nothing yet'].join('\n'),
  )
  await serveImages(page)
  await actAs(page, me)
  await page.goto(`/panel/messages/${slug}`)

  const card = page.getByTestId('question-card').filter({ hasText: 'Accept made-up-task?' })
  const images = card.getByTestId('question-images')
  const before = images.locator('[data-image="Before"]')
  const after = images.locator('[data-image="After"]')
  await expect(before.locator('figcaption')).toHaveText('Before')
  await expect(after.locator('figcaption')).toHaveText('After')
  await expect(before.getByRole('img', { name: 'Before' })).toHaveAttribute('src', BEFORE)
  await expect(after.getByRole('img', { name: 'After' })).toHaveAttribute('src', AFTER)
  await expect(before.getByRole('link')).toHaveAttribute('href', BEFORE)
  await expect(before.getByRole('link')).toHaveAttribute('target', '_blank')
  await expect(after.getByRole('link')).toHaveAttribute('href', AFTER)
  await expect(card).not.toContainText(BEFORE)
  // Side by side, each at most half the card.
  const cardBox = (await card.boundingBox())!
  const b = (await before.boundingBox())!
  const a = (await after.boundingBox())!
  expect(Math.abs(a.y - b.y)).toBeLessThan(2)
  expect(a.x).toBeGreaterThan(b.x + b.width - 1)
  expect(b.width).toBeLessThanOrEqual(cardBox.width / 2)
  expect(a.width).toBeLessThanOrEqual(cardBox.width / 2)
  // The Diff link comes after the images, the end of the rows.
  expect((await card.getByTestId('diff-link').boundingBox())!.y).toBeGreaterThan(b.y + b.height - 1)

  // Not image URLs: plain labelled rows.
  const text = page.getByTestId('question-card').filter({ hasText: 'Accept other-made-up?' })
  await expect(text.getByTestId('question-images')).toHaveCount(0)
  await expect(text.locator('[data-row="Before"]')).toContainText('the old list')
  await expect(text.locator('[data-row="After"]')).toContainText('nothing yet')
})

test('an old free-form question still renders and answers as before', async ({ page, request }) => {
  const me = uniqueName()
  const slug = uniqueName('c')
  const reviewer = await joinAs(request)
  const question = await ask(
    request,
    reviewer.id,
    `#${slug}`,
    'Merge **now** or wait? See https://example.com/x inline.',
  )
  await actAs(page, me)
  await page.goto(`/panel/messages/${slug}`)

  const card = page.getByTestId('question-card')
  await expect(card.locator('strong')).toHaveText('now')
  await expect(card.getByTestId('question-row')).toHaveCount(0)
  await expect(card.getByTestId('diff-link')).toHaveCount(0)
  await expect(card.getByTestId('accept-actions')).toHaveCount(0)
  const box = card.getByRole('textbox', { name: `Answer @${reviewer.name}` })
  await box.fill('Wait for CI.')
  const sent = nextPost(page)
  await box.press('Enter')
  expect((await sent).reply_to).toBe(question.id)
  await expect(card).toHaveAttribute('data-state', 'answered')
  await expect(card.getByTestId('answer')).toContainText('Wait for CI.')
})

test('a question for a task I do not own shows no buttons', async ({ page, request }) => {
  const me = uniqueName()
  const task = await taskInReview(request, uniqueName('other'))
  const reviewer = await joinAs(request)
  await ask(request, reviewer.id, `#${me}`, shaped(task))
  await actAs(page, me)
  await page.goto(`/panel/messages/${me}`)

  const card = page.getByTestId('question-card')
  await expect(card.getByTestId('diff-link')).toBeVisible()
  await expect(card.getByTestId('accept-state').locator('[data-status="review"]')).toBeVisible()
  await expect(card.getByRole('button', { name: /^Accept/ })).toHaveCount(0)
  await expect(card.getByRole('button', { name: 'Send back…' })).toHaveCount(0)
})

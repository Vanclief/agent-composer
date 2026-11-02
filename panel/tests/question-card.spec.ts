// Owned by the messages feature: the question card in the reviewer's shape
// (title, Changes/Asks/Diff rows, View diff) and its Accept and Reject…
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

function shaped(task: string) {
  return [
    `Accept ${task}?`,
    'Lets you accept from the card.',
    'Changes: UI: a narrower question card',
    'Asks: nothing',
    `Diff: ${DIFF}`,
  ].join('\n')
}

test('a five-line question renders a title, labelled rows and View diff, at most 72ch wide', async ({
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
  const diff = card.getByRole('link', { name: 'View diff' })
  await expect(diff).toHaveAttribute('href', DIFF)
  await expect(diff).toHaveAttribute('target', '_blank')
  await expect(card).not.toContainText(DIFF)

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
  await expect(card.getByRole('button', { name: 'Reject…' })).toHaveCount(0)
  // The answer box stays for free text until answered; the command answered it.
  await expect(card).toHaveAttribute('data-state', 'answered')
})

test('Reject… asks for a reason and posts /reject <why> in #<task>', async ({ page, request }) => {
  const me = uniqueName()
  const task = await taskInReview(request, me)
  const reviewer = await joinAs(request)
  await ask(request, reviewer.id, `#${me}`, shaped(task))
  await actAs(page, me)
  await page.goto(`/panel/messages/${me}`)

  const card = page.getByTestId('question-card')
  await card.getByRole('button', { name: 'Reject…' }).click()
  await expect(card.getByRole('button', { name: 'Send: needs a reason' })).toBeDisabled()
  await card.getByRole('textbox', { name: `Why reject ${task}` }).fill('the card is too wide')
  const sent = nextPost(page)
  await card.getByRole('button', { name: 'Send', exact: true }).click()
  const post = await sent
  expect(post.body).toBe('/reject the card is too wide')
  expect(post.channel).toBe(`#${task}`)

  // Rejected: no longer in review, status verbatim from the tasks query.
  const state = card.getByTestId('accept-state')
  await expect(state.locator('[data-status="open"]')).toBeVisible()
  await expect(card.getByRole('button', { name: 'Reject…' })).toHaveCount(0)
  const rejected = await request.get(`/api/msg/read?channel=${encodeURIComponent('#' + task)}`)
  const bodies = ((await rejected.json()) as { messages: { body: string }[] }).messages.map((m) => m.body)
  expect(bodies).toContain(`Rejected by ${me}: the card is too wide`)
  // The answer box is still there for free text.
  await expect(card.getByRole('textbox', { name: `Answer @${reviewer.name}` })).toBeVisible()
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
  await expect(card.getByRole('link', { name: 'View diff' })).toHaveCount(0)
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
  await expect(card.getByRole('link', { name: 'View diff' })).toBeVisible()
  await expect(card.getByTestId('accept-state').locator('[data-status="review"]')).toBeVisible()
  await expect(card.getByRole('button', { name: /^Accept/ })).toHaveCount(0)
  await expect(card.getByRole('button', { name: 'Reject…' })).toHaveCount(0)
})

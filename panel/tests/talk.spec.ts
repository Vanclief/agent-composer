// Talk as an inbox: Asks (#franco) pinned first, unread badges from the
// read state in localStorage, and toasts only for my inbox and mentions.
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

function suffix() {
  return `${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`
}

// An agent participant that posts through the API.
async function joinAgent(request: APIRequestContext) {
  const response = await request.post('/api/msg/join', { data: { name: `bot-${suffix()}`, task: 'review-42' } })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { id: string; name: string }
}

async function post(request: APIRequestContext, participant: string, channel: string, body: string) {
  const response = await request.post('/api/msg/post', { data: { participant_id: participant, channel, body } })
  expect(response.ok()).toBe(true)
  return (await response.json()) as { id: number }
}

function nav(page: Page) {
  return page.getByRole('navigation', { name: 'Main' })
}

function talkBadge(page: Page) {
  return page.getByTestId('area-badge-talk').getByTestId('unread-badge')
}

async function openLive(page: Page, path: string) {
  await page.goto(path)
  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
}

test('Asks is pinned first with the question icon, its unread badge in the mention color', async ({ page }) => {
  await page.goto('/panel/messages')
  const rows = page.getByTestId('channel-rows').getByRole('link')
  await expect(rows.first()).toHaveAttribute('data-testid', 'inbox-row')
  const inbox = page.getByTestId('inbox-row')
  await expect(inbox).toHaveText(/^Asks/)
  await expect(inbox.locator(':scope > svg.lucide-message-circle-question-mark')).toBeVisible()
  await expect(inbox).toHaveAttribute('href', '/panel/messages/franco')
  // The open questions for me, wherever they were asked.
  await expect(inbox.getByTestId('question-badge')).toHaveText(/^\d+$/)
  const badge = inbox.getByTestId('unread-badge')
  await expect(badge).toHaveText(/^\d+$/)
  expect(Number(await badge.textContent())).toBeGreaterThanOrEqual(3)
  await expect(badge).toHaveAttribute('data-mention', 'true')
  await expect(badge).toHaveClass(/bg-pink-600/)
  await expect(inbox).toHaveAttribute('data-unread', 'true')
  await expect(talkBadge(page)).toHaveAttribute('data-mention', 'true')
})

test('opening the inbox clears its badge, and it stays read after a reload', async ({ page, request }) => {
  // An inbox of its own: other workers post to #franco while this runs.
  const me = `me-${suffix()}`
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), me)
  const agent = await joinAgent(request)
  const escalation = `@${me} decision needed ${suffix()}`
  await post(request, agent.id, `#${me}`, 'Task opened for you.')
  await post(request, agent.id, `#${me}`, escalation)

  await page.goto('/panel/messages')
  const inbox = page.getByTestId('inbox-row')
  await expect(inbox.getByTestId('unread-badge')).toHaveText('2')
  await inbox.click()
  await expect(page).toHaveURL(new RegExp(`/panel/messages/${me}$`))
  await expect(page.getByTestId('page-title')).toHaveText(`Asks #${me}`)
  await expect(page.getByTestId('inbox-header')).toHaveText('Asks: questions and escalations waiting on you')
  await expect(page.getByText(escalation)).toBeVisible()
  await expect(inbox.getByTestId('unread-badge')).toHaveCount(0)
  await expect(inbox).toHaveAttribute('data-unread', 'false')
  const read = await page.evaluate(() => JSON.parse(localStorage.getItem('agc.panel.read') ?? '{}'))
  expect(read[`#${me}`]).toBeGreaterThan(0)

  await page.goto('/panel/messages/releases')
  await expect(page.getByTestId('inbox-row')).toHaveAttribute('data-unread', 'false')
  await page.reload()
  await expect(page.getByTestId('channel-rows').getByRole('link', { name: /^#releases\b/ })).toBeVisible()
  await expect(page.getByTestId('inbox-row').getByTestId('unread-badge')).toHaveCount(0)
})

test('a message to my inbox while on /tasks toasts and bumps the Talk badge', async ({ page, request }) => {
  // Everything already posted counts as read, so the badge starts empty.
  const { channels } = (await (await request.get('/api/msg/channels')).json()) as {
    channels: { channel: string; last_id: number }[]
  }
  const read = Object.fromEntries(channels.map((c) => [c.channel, c.last_id]))
  await page.addInitScript((r) => localStorage.setItem('agc.panel.read', JSON.stringify(r)), read)
  await openLive(page, '/panel/tasks')

  const agent = await joinAgent(request)
  const body = `Need your call ${suffix()}: ` + 'merge or wait? '.repeat(20)
  const message = await post(request, agent.id, '#franco', body)

  const toast = page.locator('[data-sonner-toast]').filter({
    has: page.locator(`[data-testid="toast-message"][data-id="${message.id}"]`),
  })
  await expect(toast).toBeVisible()
  await expect(toast).toContainText(`@${agent.name}`)
  await expect(toast).toContainText('#franco')
  // 160 characters and an ellipsis.
  expect((await toast.getByTestId('toast-message-body').textContent())?.length).toBe(161)
  await expect(talkBadge(page)).toHaveText(/^\d+$/)
  expect(Number(await talkBadge(page).textContent())).toBeGreaterThanOrEqual(1)

  await toast.getByRole('button', { name: 'Open' }).dispatchEvent('click')
  await expect(page).toHaveURL(/\/panel\/messages\/franco$/)
  await expect(page.getByTestId('message-list').getByText(body.slice(0, 30))).toBeVisible()
})

test('a message to a followed channel bumps only its badge, with no toast', async ({ page, request }) => {
  const channel = `#talk-${suffix()}`
  await page.addInitScript(
    (c) => localStorage.setItem('agc.panel.follow', JSON.stringify({ [c]: { on: true, at: 0 } })),
    channel,
  )
  await openLive(page, '/panel/messages/releases')
  const agent = await joinAgent(request)
  const message = await post(request, agent.id, channel, 'Nothing for franco here.')

  const row = nav(page).getByRole('link', { name: new RegExp(`^${channel}\\b`) })
  await expect(row.getByTestId('unread-badge')).toHaveText('1')
  await expect(row.getByTestId('unread-badge')).toHaveAttribute('data-mention', 'false')
  await expect(row).toHaveAttribute('data-unread', 'true')
  await expect(
    page.getByRole('group', { name: 'Channels' }).getByRole('link', { name: new RegExp(`^${channel}\\b`) }),
  ).toBeVisible()
  await page.waitForTimeout(500)
  await expect(page.locator(`[data-testid="toast-message"][data-id="${message.id}"]`)).toHaveCount(0)

  await row.click()
  await expect(row.getByTestId('unread-badge')).toHaveCount(0)
})

test('a mention of me in another channel toasts; my own messages never do', async ({ page, request }) => {
  // Its own name, so no other test holds it or posts to its inbox.
  const me = `me-${suffix()}`
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), me)
  await openLive(page, '/panel/messages/releases')
  const channel = `#talk-${suffix()}`
  // A person's mention follows the channel (follow.ts); an agent's only toasts.
  const person = await request.post('/api/msg/join', { data: { name: `p-${suffix()}`, task: '' } })
  const agent = (await person.json()) as { id: string }
  const mention = await post(request, agent.id, channel, `@${me} can you look?`)
  const toast = page.locator(`[data-testid="toast-message"][data-id="${mention.id}"]`)
  await expect(toast).toBeVisible()
  await expect(toast).toContainText(channel)
  const row = nav(page).getByRole('link', { name: new RegExp(`^${channel}\\b`) })
  await expect(row.getByTestId('unread-badge')).toHaveAttribute('data-mention', 'true')
  // The inbox row follows the name.
  await expect(page.getByTestId('inbox-row')).toHaveAttribute('href', `/panel/messages/${me}`)

  // Mine, even with a mention, only shows in the feed.
  const box = page.getByRole('textbox', { name: 'Message #releases' })
  const text = `Note to self @${me}`
  await expect(page.getByRole('button', { name: 'Send: empty' })).toBeVisible()
  await box.fill(text)
  await box.press('Enter')
  await expect(page.getByTestId('message-list').getByText(text)).toBeVisible()
  await page.waitForTimeout(500)
  await expect(page.locator('[data-testid="toast-message"]').filter({ hasText: text })).toHaveCount(0)
})

test('Channels lists task channels first, then the rest, and ends with Browse', async ({ page }) => {
  // #review-42 is followed because franco posted there and an agent asks
  // @franco a question there (agc's opening mention alone would not);
  // #releases once opened, so it is newer yet comes after the task.
  await page.goto('/panel/messages/releases')
  const channels = page.getByRole('group', { name: 'Channels' })
  const rows = channels.getByRole('link')
  await expect(rows.first()).toHaveAttribute('data-channel', '#review-42')
  await expect(channels.locator('[data-channel="#releases"]')).toBeVisible()
  const order = await channels
    .locator('[data-channel]')
    .evaluateAll((els) => els.map((e) => e.getAttribute('data-channel')))
  expect(order.indexOf('#review-42')).toBeLessThan(order.indexOf('#releases'))
  await expect(rows.last()).toHaveAttribute('data-testid', 'browse-link')
  // Asks is its own row, and agent inboxes and DMs are in DMs.
  for (const c of ['#franco', '#designer-1', '@franco/yc-pitch']) {
    await expect(channels.locator(`[data-channel="${c}"]`)).toHaveCount(0)
  }
})

test("DMs list a followed agent's inbox as @agent with its state light, and my DMs", async ({ page }) => {
  // franco wrote to designer-1 in #designer-1, so it is followed.
  await page.goto('/panel/messages')
  const dms = page.getByRole('group', { name: 'DMs' })
  const designer = dms.getByTestId('agent-dm-row').and(dms.locator('[data-channel="#designer-1"]'))
  await expect(designer).toContainText('@designer-1')
  await expect(designer).not.toContainText('#designer-1')
  const led = designer.getByTestId('status-led')
  await expect(led).toHaveAttribute('data-status', 'awake')
  await expect(led).toHaveAttribute('aria-label', 'awake')
  await expect(designer.getByTestId('unread-badge')).toHaveText('1')
  await expect(dms.getByTestId('dm-row').and(dms.locator('[data-channel="@franco/yc-pitch"]'))).toBeVisible()
  // Newest activity first: designer-1's answer is the last seeded post.
  await expect(dms.getByRole('link').first()).toHaveAttribute('data-channel', '#designer-1')

  await designer.click()
  await expect(page).toHaveURL(/\/panel\/messages\/designer-1$/)
  await expect(page.getByTestId('message-list').getByText('Sketch is up')).toBeVisible()
  await expect(designer.getByTestId('unread-badge')).toHaveCount(0)
})

// Talk mode as an inbox: #franco pinned first, unread badges from the
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
  return page.getByRole('navigation', { name: 'Mode' }).getByTestId('unread-badge')
}

async function openLive(page: Page, path: string) {
  await page.goto(path)
  await expect(page.getByTestId('live-status')).toHaveAttribute('data-state', 'connected')
}

test('the inbox is pinned first with its unread badge in the mention color', async ({ page }) => {
  await page.goto('/panel/messages')
  const rows = page.getByTestId('channel-rows').getByRole('link')
  await expect(rows.first()).toHaveAttribute('data-testid', 'inbox-row')
  const inbox = page.getByTestId('inbox-row')
  await expect(inbox).toContainText('Inbox')
  await expect(inbox).toHaveAttribute('href', '/panel/messages/franco')
  const badge = inbox.getByTestId('unread-badge')
  await expect(badge).toHaveText(/^\d+$/)
  expect(Number(await badge.textContent())).toBeGreaterThanOrEqual(3)
  await expect(badge).toHaveAttribute('data-mention', 'true')
  await expect(badge).toHaveClass(/bg-pink-600/)
  await expect(inbox).toHaveAttribute('data-unread', 'true')
  await expect(talkBadge(page)).toHaveAttribute('data-mention', 'true')
})

test('opening the inbox clears its badge, and it stays read after a reload', async ({ page }) => {
  await page.goto('/panel/messages')
  const inbox = page.getByTestId('inbox-row')
  await expect(inbox.getByTestId('unread-badge')).toBeVisible()
  await inbox.click()
  await expect(page).toHaveURL(/\/panel\/messages\/franco$/)
  await expect(page.getByTestId('page-title')).toHaveText('Inbox #franco')
  await expect(page.getByTestId('inbox-header')).toHaveText('Your inbox: agents and tasks escalate here')
  await expect(page.getByText('decision needed on review-42')).toBeVisible()
  await expect(inbox.getByTestId('unread-badge')).toHaveCount(0)
  await expect(inbox).toHaveAttribute('data-unread', 'false')
  const read = await page.evaluate(() => JSON.parse(localStorage.getItem('agc.panel.read') ?? '{}'))
  expect(read['#franco']).toBeGreaterThan(0)

  await page.goto('/panel/messages/releases')
  await expect(page.getByTestId('inbox-row')).toHaveAttribute('data-unread', 'false')
  await page.reload()
  await expect(page.getByTestId('channel-rows').getByRole('link', { name: /^#review-42\b/ })).toBeVisible()
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
    page.getByRole('group', { name: 'Other' }).getByRole('link', { name: new RegExp(`^${channel}\\b`) }),
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
  const agent = await joinAgent(request)
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

test('the Tasks group lists task channels and Other the rest', async ({ page }) => {
  // #review-42 is followed because agc's opening message mentions @franco;
  // #releases once opened.
  await page.goto('/panel/messages/releases')
  const tasks = page.getByRole('group', { name: 'Tasks' })
  const other = page.getByRole('group', { name: 'Other' })
  await expect(tasks.getByRole('link', { name: /^#review-42\b/ })).toBeVisible()
  await expect(other.getByRole('link', { name: /^#releases\b/ })).toBeVisible()
  await expect(tasks.getByRole('link', { name: /^#releases\b/ })).toHaveCount(0)
  // The inbox is its own row, in neither group.
  await expect(tasks.getByRole('link', { name: /#franco/ })).toHaveCount(0)
  await expect(other.getByRole('link', { name: /#franco/ })).toHaveCount(0)
})

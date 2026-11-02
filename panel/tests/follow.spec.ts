// Following (src/features/messages/follow.ts): the Talk sidebar lists only
// channels I opened, posted in or was mentioned in, my inbox and my DMs;
// Browse lists every channel; notifications only for what is addressed to me.
import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test'

function suffix() {
  return `${Date.now().toString(36)}${Math.floor(Math.random() * 1000)}`
}

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

function rows(page: Page) {
  return page.getByTestId('channel-rows')
}

function row(page: Page, channel: string) {
  return rows(page).locator(`[data-channel="${channel}"]`)
}

function talkBadge(page: Page) {
  return page.getByRole('navigation', { name: 'Mode' }).getByTestId('unread-badge')
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
  // agc's opening message in #review-42 mentions @franco.
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

test('a mention in an unfollowed channel follows it and toasts', async ({ page, request }) => {
  const me = await actAs(page)
  await openLive(page, '/panel/tasks')
  const channel = `#follow-${suffix()}`
  const agent = await joinAgent(request)
  const message = await post(request, agent.id, channel, `@${me} take a look`)
  await expect(page.locator(`[data-testid="toast-message"][data-id="${message.id}"]`)).toBeVisible()
  await expect(talkBadge(page)).toHaveText('1')
  await page.getByRole('navigation', { name: 'Mode' }).getByRole('link', { name: /Talk/ }).click()
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

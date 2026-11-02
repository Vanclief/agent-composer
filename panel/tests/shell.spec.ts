// Owned by the foundation (layout, routing).
import { expect, test, type Page } from '@playwright/test'

test('/ redirects to tasks on a first visit', async ({ page }) => {
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(page.getByTestId('page-title')).toHaveText('Tasks')
  await expect(page.getByTestId('page-area')).toHaveText('Work /')

  await page.getByRole('link', { name: 'Channels', exact: true }).click()
  await expect(page).toHaveURL(/\/panel\/messages$/)
  await expect(page.getByTestId('page-title')).toHaveText('Messages')
  await expect(page.getByTestId('page-area')).toHaveText('Talk /')
})

test('unknown paths go to tasks', async ({ page }) => {
  await page.goto('/panel/no/such/page')
  await expect(page).toHaveURL(/\/panel\/tasks$/)
})

function header(page: Page, area: string) {
  return page.getByTestId(`area-header-${area}`)
}

function group(page: Page, label: string) {
  return page.getByRole('group', { name: label, exact: true })
}

for (const path of ['/panel/tasks', '/panel/think/beliefs', '/panel/messages']) {
  test(`the three group headers are always shown on ${path}`, async ({ page }) => {
    await page.goto(path)
    const nav = page.getByRole('navigation', { name: 'Main' })
    await expect(nav.locator('[data-testid^="area-header-"]')).toHaveText(['Think', /^Work(\d+)?$/, /^Talk/])
    const think = await header(page, 'think').boundingBox()
    const work = await header(page, 'work').boundingBox()
    const talk = await header(page, 'talk').boundingBox()
    expect(work!.y).toBeGreaterThan(think!.y + think!.height - 1)
    expect(talk!.y).toBeGreaterThan(work!.y + work!.height - 1)
    expect(talk!.x).toBe(work!.x)
    // Every group open by default, pages of all three listed.
    await expect(group(page, 'Think').getByRole('link')).toHaveText([
      'Beliefs',
      'Decisions',
      'Rules',
      'Actions',
      'Evidence',
    ])
    await expect(group(page, 'Work').getByRole('link')).toHaveText([/^Tasks(\d+)?$/, 'Compute', 'Agents', 'Accounts'])
    await expect(group(page, 'Talk').getByRole('link', { name: 'Channels', exact: true })).toBeVisible()
  })
}

test('the route highlights its page without hiding the other groups', async ({ page }) => {
  await page.goto('/panel/compute/review-42')
  await expect(page.getByTestId('page-title')).toHaveText('review-42')
  await expect(group(page, 'Work').getByRole('link', { name: 'Compute' })).toHaveAttribute('aria-current', 'page')
  await expect(header(page, 'work')).toHaveAttribute('data-current', 'true')
  await expect(header(page, 'think')).toHaveAttribute('data-current', 'false')
  await group(page, 'Think').getByRole('link', { name: 'Rules' }).click()
  await expect(page).toHaveURL(/\/panel\/think\/rules$/)
  await expect(page.getByTestId('page-area')).toHaveText('Think /')
  await expect(group(page, 'Work').getByRole('link')).toHaveText([/^Tasks(\d+)?$/, 'Compute', 'Agents', 'Accounts'])
})

test('collapsing Work hides its pages, keeps the header, and survives a reload', async ({ page }) => {
  await page.goto('/panel/think/beliefs')
  const work = header(page, 'work')
  await expect(work).toHaveAttribute('aria-expanded', 'true')
  await work.click()
  await expect(work).toHaveAttribute('aria-expanded', 'false')
  await expect(work).toBeVisible()
  for (const name of ['Tasks', 'Compute', 'Agents', 'Accounts']) {
    await expect(group(page, 'Work').getByRole('link', { name })).toHaveCount(0)
  }
  await expect(group(page, 'Think').getByRole('link', { name: 'Beliefs' })).toBeVisible()

  await page.reload()
  await expect(header(page, 'work')).toHaveAttribute('aria-expanded', 'false')
  await expect(group(page, 'Work').getByRole('link')).toHaveCount(0)

  await header(page, 'work').click()
  await expect(group(page, 'Work').getByRole('link')).toHaveText([/^Tasks(\d+)?$/, 'Compute', 'Agents', 'Accounts'])
})

test('navigating to a channel opens Talk when it was collapsed, and leaves the others', async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('seeded')) {
      sessionStorage.setItem('seeded', '1')
      localStorage.setItem('agc.panel.group.talk', 'closed')
      localStorage.setItem('agc.panel.group.think', 'closed')
    }
  })
  await page.goto('/panel/tasks')
  await expect(header(page, 'talk')).toHaveAttribute('aria-expanded', 'false')
  await expect(page.getByTestId('channel-rows')).toHaveCount(0)

  await page.goto('/panel/messages/review-42')
  await expect(header(page, 'talk')).toHaveAttribute('aria-expanded', 'true')
  await expect(page.getByTestId('channel-rows')).toBeVisible()
  await expect(header(page, 'think')).toHaveAttribute('aria-expanded', 'false')
  await expect(header(page, 'work')).toHaveAttribute('aria-expanded', 'true')
})

test('the Talk badge stays visible with Talk collapsed', async ({ page }) => {
  await page.goto('/panel/tasks')
  const badge = page.getByTestId('area-badge-talk').getByTestId('unread-badge')
  // Never opened in this browser: the inbox alone is unread.
  await expect(badge).toHaveText(/^\d+\+?$/)
  await header(page, 'talk').click()
  await expect(page.getByTestId('channel-rows')).toHaveCount(0)
  await expect(badge).toBeVisible()
  await expect(badge).toHaveText(/^\d+\+?$/)
})

test('/ opens the last page visited', async ({ page }) => {
  await page.goto('/panel/think/rules')
  await expect(page.getByTestId('page-title')).toHaveText('Rules')
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/think\/rules$/)

  await page.goto('/panel/messages/review-42')
  await expect(page.getByTestId('page-title')).toHaveText(/review-42/)
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/messages\/review-42$/)
})

test('the sidebar scrolls as one column with every group open', async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 420 })
  await page.goto('/panel/messages')
  const nav = page.getByRole('navigation', { name: 'Main' })
  await expect(page.getByTestId('channel-rows')).toBeVisible()
  const overflow = await nav.evaluate((el) => el.scrollHeight > el.clientHeight)
  expect(overflow).toBe(true)
  await nav.evaluate((el) => el.scrollTo(0, el.scrollHeight))
  // The headers scroll away with the rest: none sticks.
  await expect(header(page, 'think')).not.toBeInViewport()
  await expect(page.getByTestId('browse-link')).toBeInViewport()
})

test('a stored name that is not the server owner rings the chip; one click signs in as the owner', async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('seeded')) {
      sessionStorage.setItem('seeded', '1')
      localStorage.setItem('agc.panel.human', 'test')
    }
  })
  await page.goto('/panel/tasks')
  const chip = page.getByTestId('me')
  await expect(chip).toHaveText('@test')
  await expect(chip).toHaveAttribute('data-mismatch', 'true')
  await expect(chip).toHaveAttribute('title', "You are @test here, the server's owner is @franco")

  await chip.click()
  const joined = page.waitForRequest((r) => r.url().endsWith('/api/msg/join') && r.method() === 'POST')
  await page.getByTestId('sign-in-owner').click()
  expect((await joined).postDataJSON()).toEqual({ name: 'franco' })
  await expect(chip).toHaveText('@franco')
  await expect(chip).not.toHaveAttribute('data-mismatch')
  await expect(chip).toHaveAttribute('title', 'Signed in as @franco')
  expect(await page.evaluate(() => localStorage.getItem('agc.panel.human'))).toBe('franco')
})

test('with no stored name the panel is the server owner from GET /api/defaults', async ({ page, request }) => {
  const defaults = await (await request.get('/api/defaults')).json()
  expect(defaults).toEqual({ harness: 'claude_code', model: 'claude-opus-5-5', owner: 'franco', system: 'agc' })
  await page.route('**/api/defaults', (route) => route.fulfill({ json: { ...defaults, owner: 'ada' } }))
  await page.goto('/panel/tasks')
  await expect(page.getByTestId('me')).toHaveText('@ada')
  await expect(page.getByTestId('me')).not.toHaveAttribute('data-mismatch')
})

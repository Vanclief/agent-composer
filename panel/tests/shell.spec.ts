// Owned by the foundation (layout, routing, theme).
import { expect, test } from '@playwright/test'

test('/ redirects to tasks and the Talk mode opens messages', async ({ page }) => {
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(page.getByTestId('page-title')).toHaveText('Tasks')

  await page.getByRole('navigation', { name: 'Mode' }).getByRole('link', { name: 'Talk' }).click()
  await expect(page).toHaveURL(/\/panel\/messages$/)
  await expect(page.getByTestId('page-title')).toHaveText('Messages')
})

test('unknown paths go to tasks', async ({ page }) => {
  await page.goto('/panel/no/such/page')
  await expect(page).toHaveURL(/\/panel\/tasks$/)
})

test('theme defaults to dark and the toggle persists', async ({ page }) => {
  await page.goto('/panel/tasks')
  const html = page.locator('html')
  await expect(html).toHaveClass(/dark/)

  await page.getByRole('button', { name: 'Light theme' }).click()
  await expect(html).not.toHaveClass(/dark/)
  await page.reload()
  await expect(html).not.toHaveClass(/dark/)
  await expect(page.getByRole('button', { name: 'Dark theme' })).toBeVisible()
})

test('the mode switch swaps the nav between Work and Think', async ({ page }) => {
  await page.goto('/panel/tasks')
  const mode = page.getByRole('navigation', { name: 'Mode' })
  const nav = page.getByRole('navigation', { name: 'Main' })
  await expect(mode.getByRole('link', { name: 'Work' })).toHaveAttribute('aria-current', 'page')
  await expect(nav.getByRole('link')).toHaveText(['Tasks', 'Compute'])

  await mode.getByRole('link', { name: 'Think' }).click()
  await expect(page).toHaveURL(/\/panel\/think\/beliefs$/)
  await expect(mode.getByRole('link', { name: 'Think' })).toHaveAttribute('aria-current', 'page')
  await expect(nav.getByRole('link')).toHaveText(['Beliefs', 'Decisions', 'Rules', 'Actions', 'Evidence'])
  await nav.getByRole('link', { name: 'Rules' }).click()
  await expect(page).toHaveURL(/\/panel\/think\/rules$/)

  await mode.getByRole('link', { name: 'Work' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(nav.getByRole('link')).toHaveText(['Tasks', 'Compute'])
})

test('Work nav opens Compute, which stays in Work mode', async ({ page }) => {
  await page.goto('/panel/tasks')
  const mode = page.getByRole('navigation', { name: 'Mode' })
  const nav = page.getByRole('navigation', { name: 'Main' })
  await nav.getByRole('link', { name: 'Compute' }).click()
  await expect(page).toHaveURL(/\/panel\/compute$/)
  await expect(page.getByTestId('page-title')).toHaveText('Compute')
  await expect(mode.getByRole('link', { name: 'Work' })).toHaveAttribute('aria-current', 'page')
  await expect(nav.getByRole('link')).toHaveText(['Tasks', 'Compute'])

  await page.goto('/panel/compute/review-42')
  await expect(page.getByTestId('page-title')).toHaveText('review-42')
  await expect(mode.getByRole('link', { name: 'Work' })).toHaveAttribute('aria-current', 'page')
})

test('the three modes are a vertical list', async ({ page }) => {
  await page.goto('/panel/tasks')
  const mode = page.getByRole('navigation', { name: 'Mode' })
  await expect(mode.getByRole('link')).toHaveText(['Think', 'Work', 'Talk'])
  const think = await mode.getByRole('link', { name: 'Think' }).boundingBox()
  const work = await mode.getByRole('link', { name: 'Work' }).boundingBox()
  const talk = await mode.getByRole('link', { name: 'Talk' }).boundingBox()
  expect(work!.y).toBeGreaterThan(think!.y + think!.height - 1)
  expect(talk!.y).toBeGreaterThan(work!.y + work!.height - 1)
  expect(talk!.x).toBe(work!.x)
})

test('/messages is Talk mode with the Channels nav', async ({ page }) => {
  await page.goto('/panel/messages')
  const mode = page.getByRole('navigation', { name: 'Mode' })
  const nav = page.getByRole('navigation', { name: 'Main' })
  await expect(mode.getByRole('link', { name: 'Talk' })).toHaveAttribute('aria-current', 'page')
  await expect(mode.getByRole('link', { name: 'Work' })).not.toHaveAttribute('aria-current', 'page')
  await expect(nav.getByRole('link', { name: 'Channels', exact: true })).toBeVisible()
  await expect(nav.getByRole('link', { name: 'Tasks' })).toHaveCount(0)
})

test('/ opens Talk when it was the last mode', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('agc.panel.mode', 'talk'))
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/messages$/)
})

test('the Talk sidebar lists channels with their counts', async ({ page }) => {
  const res = await page.request.get('/api/msg/channels')
  const { channels } = (await res.json()) as { channels: { channel: string; count: number }[] }
  await page.goto('/panel/messages')
  const nav = page.getByRole('navigation', { name: 'Main' })
  for (const name of ['#review-42', '#releases']) {
    const info = channels.find((c) => c.channel === name)
    expect(info).toBeDefined()
    const row = nav.getByRole('link', { name: new RegExp(`^${name}\\b`) })
    await expect(row).toBeVisible()
    await expect(row.getByTestId('channel-count')).toHaveText(/^\d+$/)
    const count = Number(await row.getByTestId('channel-count').textContent())
    expect(count).toBeGreaterThanOrEqual(info!.count)
  }
})

test('clicking a channel in the sidebar opens and highlights it', async ({ page }) => {
  await page.goto('/panel/messages/review-42')
  const nav = page.getByRole('navigation', { name: 'Main' })
  const releases = nav.getByRole('link', { name: /^#releases\b/ })
  const review = nav.getByRole('link', { name: /^#review-42\b/ })
  await expect(review).toHaveAttribute('aria-current', 'page')
  await releases.click()
  await expect(page).toHaveURL(/\/panel\/messages\/releases$/)
  await expect(releases).toHaveAttribute('aria-current', 'page')
  await expect(releases).toHaveClass(/bg-pink-500\/20/)
  await expect(review).not.toHaveAttribute('aria-current', 'page')
})

test('Work mode shows no channel rows', async ({ page }) => {
  await page.goto('/panel/tasks')
  const nav = page.getByRole('navigation', { name: 'Main' })
  await expect(nav.getByRole('link')).toHaveText(['Tasks', 'Compute'])
  await expect(nav.getByTestId('channel-count')).toHaveCount(0)
})

// Owned by the foundation (StatusPill). Status pills show the backend's
// word verbatim in one of four light tiers (data-tier): lit, off,
// attention, idle. Lists sort attention, lit, idle, off.
import { expect, test, type Page } from '@playwright/test'

const TIERS = ['attention', 'lit', 'idle', 'off']

async function tiersInOrder(page: Page, selector: string) {
  const tiers = await page.locator(selector).evaluateAll((els) => els.map((el) => el.getAttribute('data-tier') ?? ''))
  return tiers.map((tier) => TIERS.indexOf(tier))
}

test('running, done, failed and open each show their own tier, verbatim', async ({ page }) => {
  await page.goto('/panel/compute')
  const running = page.getByRole('article', { name: 'review-42', exact: true }).locator('[data-status]')
  await expect(running).toHaveText('running')
  await expect(running).toHaveAttribute('data-tier', 'lit')
  const failed = page.getByRole('article', { name: 'boot-fail', exact: true }).locator('[data-status]')
  await expect(failed).toHaveText('failed')
  await expect(failed).toHaveAttribute('data-tier', 'attention')
  const stopped = page.getByRole('article', { name: 'docs-pass', exact: true }).locator('[data-status]')
  await expect(stopped).toHaveText('stopped')
  await expect(stopped).toHaveAttribute('data-tier', 'off')

  await page.goto('/panel/tasks')
  const done = page.locator('[data-task="docs-pass"]').getByTestId('task-status').locator('[data-status]')
  await expect(done).toHaveText('done')
  await expect(done).toHaveAttribute('data-tier', 'off')
  const open = page.locator('[data-task="flaky-ci"]').getByTestId('task-status').locator('[data-status]')
  await expect(open).toHaveText('open')
  await expect(open).toHaveAttribute('data-tier', 'idle')
  const review = page.locator('[data-task="review-42"]').getByTestId('task-status').locator('[data-status]')
  await expect(review).toHaveAttribute('data-tier', 'lit')
})

test('a done pill is not lit like a running one', async ({ page }) => {
  await page.goto('/panel/tasks')
  const done = page.locator('[data-task="docs-pass"]').getByTestId('task-status').locator('[data-status]')
  const running = page.locator('[data-task="review-42"]').getByTestId('sandbox-summary').locator('[data-status]')
  await expect(running).toHaveText('running')
  await expect(running).toHaveAttribute('data-tier', 'lit')
  await expect(done).toHaveAttribute('data-tier', 'off')
  // The done card recedes too.
  await expect(page.locator('[data-task="docs-pass"]')).toHaveAttribute('data-tier', 'off')
})

test('lists sort attention, then lit, then idle, then off', async ({ page }) => {
  await page.goto('/panel/compute')
  await expect(page.getByRole('region', { name: 'failed', exact: true })).toBeVisible()
  const groups = await tiersInOrder(page, 'section > h2 > [data-tier]')
  expect(groups).toEqual([...groups].sort((a, b) => a - b))
  expect(groups[0]).toBe(TIERS.indexOf('attention'))
  expect(groups.at(-1)).toBe(TIERS.indexOf('off'))

  await page.goto('/panel/tasks')
  await expect(page.locator('[data-task="docs-pass"]')).toBeVisible()
  const cards = await tiersInOrder(page, '[data-task]')
  expect(cards).toEqual([...cards].sort((a, b) => a - b))
})

test('the legend shows the four tiers, words only on hover', async ({ page }) => {
  for (const path of ['/panel/tasks', '/panel/compute']) {
    await page.goto(path)
    const legend = page.getByTestId('status-legend')
    await expect(legend).toHaveText('')
    const tiers = await legend
      .locator('[data-tier]')
      .evaluateAll((els) => els.map((el) => [el.getAttribute('data-tier'), el.getAttribute('title')]))
    expect(tiers).toEqual(TIERS.map((tier) => [tier, tier]))
  }
})

test('open is the green LED off, an unknown word the same LED with grey text', async ({ page }) => {
  // A status this panel does not know, injected into the task list.
  await page.route('**/api/tasks', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    body.tasks = [
      {
        ...body.tasks.find((t: { name: string }) => t.name === 'flaky-ci'),
        id: 'x-paused',
        name: 'paused-x',
        status: 'paused',
      },
      ...body.tasks,
    ]
    await route.fulfill({ response, json: body })
  })
  await page.goto('/panel/tasks')
  const open = page.locator('[data-task="flaky-ci"]').getByTestId('task-status').locator('[data-status]')
  await expect(open).toHaveAttribute('data-tier', 'idle')
  await expect(open.locator('[aria-hidden] > span')).toHaveClass(/bg-emerald-950/)
  await expect(open).not.toHaveClass(/zinc/)

  const unknown = page.locator('[data-task="paused-x"]').getByTestId('task-status').locator('[data-status]')
  await expect(unknown).toHaveText('paused')
  await expect(unknown).toHaveAttribute('data-tier', 'idle')
  await expect(unknown.locator('[aria-hidden] > span')).toHaveClass(/bg-emerald-950/)
  await expect(unknown).toHaveClass(/zinc/)
})

// Owned by the terminals feature (TerminalsTab).
import { expect, test } from '@playwright/test'

test('terminals tab lists the jobs of a running task', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/terminals')
  await expect(page.getByRole('heading', { name: 'TerminalsTab' })).toBeVisible()
  await expect(page.getByText('claude', { exact: true })).toBeVisible()
})

test('a stopped task says why it has no terminals', async ({ page }) => {
  await page.goto('/panel/tasks/flaky-ci/terminals')
  await expect(page.getByText('Task stopped')).toBeVisible()
})

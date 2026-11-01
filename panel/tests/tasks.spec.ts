// Owned by the tasks feature (TasksPage, TaskPage, SpecTab).
import { expect, test } from '@playwright/test'

test('tasks list shows each task with its backend status', async ({ page }) => {
  await page.goto('/panel/tasks')
  await expect(page.getByRole('heading', { name: 'TasksPage' })).toBeVisible()

  const review = page.getByRole('listitem').filter({ hasText: 'Review 42' })
  await expect(review.locator('[data-status]')).toHaveText('running')
  const docs = page.getByRole('listitem').filter({ hasText: 'docs-pass' })
  await expect(docs.locator('[data-status]')).toHaveText('unknown')
})

test('a task opens on its terminals tab', async ({ page }) => {
  await page.goto('/panel/tasks/review-42')
  await expect(page).toHaveURL(/\/panel\/tasks\/review-42\/terminals$/)
  await expect(page.getByRole('heading', { name: 'TaskPage' })).toBeVisible()
  await expect(page.getByTestId('page-title')).toHaveText('Review 42')
})

test('spec tab shows the task spec', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/spec')
  await expect(page.getByRole('heading', { name: 'SpecTab' })).toBeVisible()
  await expect(page.getByText('image: agc-agent')).toBeVisible()
})

test('an unknown task shows the backend error verbatim', async ({ page }) => {
  await page.goto('/panel/tasks/nope/spec')
  await expect(page.getByRole('alert')).toHaveText('task nope not found')
})

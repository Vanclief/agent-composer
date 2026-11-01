// Owned by the messages feature (MessagesTab, MessagesPage).
import { expect, test } from '@playwright/test'

test('a task messages tab shows its channel', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/messages')
  await expect(page.getByRole('heading', { name: 'MessagesTab' })).toBeVisible()
  await expect(page.getByText('Look at the loop node first.')).toBeVisible()
})

test('messages page lists channels', async ({ page }) => {
  await page.goto('/panel/messages')
  await expect(page.getByRole('heading', { name: 'MessagesPage' })).toBeVisible()
  await expect(page.getByRole('link', { name: /review-42/ })).toBeVisible()
})

test('a channel route shows that channel', async ({ page }) => {
  await page.goto('/panel/messages/releases')
  await expect(page.getByTestId('page-title')).toHaveText('Messages #releases')
  await expect(page.getByText('v0.4.0 tagged.')).toBeVisible()
})

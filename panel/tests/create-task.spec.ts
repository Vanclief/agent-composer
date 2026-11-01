// Owned by the create-task feature (CreateTaskDialog).
import { expect, test } from '@playwright/test'

test('New task opens the create dialog with secrets and saved specs', async ({ page }) => {
  await page.goto('/panel/tasks')
  await page.getByRole('button', { name: 'New task' }).click()

  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('CreateTaskDialog')).toBeVisible()
  await expect(dialog.getByText('backend-review')).toBeVisible()
  await expect(dialog.getByText('GITHUB_TOKEN')).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Create: needs a title' })).toBeDisabled()
})

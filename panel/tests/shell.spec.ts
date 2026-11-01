// Owned by the foundation (layout, routing, theme).
import { expect, test } from '@playwright/test'

test('/ redirects to tasks and the sidebar links both pages', async ({ page }) => {
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(page.getByTestId('page-title')).toHaveText('Tasks')

  const nav = page.getByRole('navigation', { name: 'Main' })
  await nav.getByRole('link', { name: 'Messages' }).click()
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
  await expect(nav.getByRole('link')).toHaveText(['Tasks', 'Messages'])

  await mode.getByRole('link', { name: 'Think' }).click()
  await expect(page).toHaveURL(/\/panel\/think\/beliefs$/)
  await expect(mode.getByRole('link', { name: 'Think' })).toHaveAttribute('aria-current', 'page')
  await expect(nav.getByRole('link')).toHaveText(['Beliefs', 'Decisions', 'Rules', 'Evidence'])
  await nav.getByRole('link', { name: 'Rules' }).click()
  await expect(page).toHaveURL(/\/panel\/think\/rules$/)

  await mode.getByRole('link', { name: 'Work' }).click()
  await expect(page).toHaveURL(/\/panel\/tasks$/)
  await expect(nav.getByRole('link')).toHaveText(['Tasks', 'Messages'])
})

// Owned by the terminals feature (TerminalsTab).
import { expect, test, type Page } from '@playwright/test'

const rail = (page: Page) => page.getByRole('list', { name: 'Terminals' })
const tile = (page: Page, command: string) =>
  rail(page)
    .getByRole('button')
    .filter({ has: page.getByText(command, { exact: true }) })
const visibleScreen = (page: Page) => page.locator('.xterm-rows:visible')

test('the rail shows each job with its state shape', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/terminals')
  await expect(tile(page, 'claude').locator('[data-state="running"]')).toBeVisible()
  await expect(tile(page, 'bash').locator('[data-state="exited"]').first()).toBeVisible()
  await expect(page.getByRole('group', { name: 'Start terminal' })).toBeVisible()
})

test('selecting a job attaches and echoes typed input', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/terminals')
  await tile(page, 'claude').click()
  await expect(tile(page, 'claude')).toHaveAttribute('aria-current', 'true')
  await expect(visibleScreen(page)).toContainText('mock terminal')
  await expect(page.getByRole('button', { name: 'Detach' })).toBeVisible()

  await page.locator('.xterm:visible').click()
  await page.keyboard.type('echo hi')
  await expect(visibleScreen(page)).toContainText('echo hi')

  await page.getByRole('button', { name: 'Detach' }).click()
  await expect(page.getByRole('button', { name: 'Reconnect' })).toBeVisible()
  await expect(tile(page, 'claude')).toBeVisible()
})

test('an exited job keeps its output and says exited', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/terminals')
  const exited = tile(page, 'bash')
    .filter({ has: page.locator('[data-state="exited"]') })
    .first()
  await exited.click()
  await expect(visibleScreen(page)).toContainText('[process exited]')
  await expect(exited.getByText('exited', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Reconnect' })).toHaveCount(0)
})

test('quick-start bash starts a job and selects it', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/terminals')
  await expect(tile(page, 'claude')).toBeVisible()
  await page.getByRole('group', { name: 'Start terminal' }).getByRole('button', { name: 'bash' }).click()
  const selected = rail(page).locator('[aria-current="true"]')
  await expect(selected).toContainText('bash')
  await expect(selected.locator('[data-state="running"]')).toBeVisible()
  await expect(visibleScreen(page)).toContainText('mock terminal · review-42 · bash')
})

test('a custom command starts on Enter', async ({ page }) => {
  await page.goto('/panel/tasks/review-42/terminals')
  const command = `vim notes-${Date.now()}`
  await page.getByRole('textbox', { name: 'Command' }).fill(command)
  await page.getByRole('textbox', { name: 'Command' }).press('Enter')
  await expect(rail(page).locator('[aria-current="true"]')).toContainText(command)
})

test('a stopped task says why and offers no start buttons', async ({ page }) => {
  await page.goto('/panel/tasks/flaky-ci/terminals')
  await expect(page.getByText('Task stopped', { exact: true })).toBeVisible()
  await expect(page.getByRole('group', { name: 'Start terminal' })).toHaveCount(0)
  await expect(rail(page)).toHaveCount(0)
})

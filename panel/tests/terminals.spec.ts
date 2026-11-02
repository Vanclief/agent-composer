// Owned by the terminals feature (TerminalsTab, TerminalPane), mounted on
// the compute feature's SandboxPage at /compute/:name.
import { expect, test, type Page } from '@playwright/test'

const rail = (page: Page) => page.getByRole('list', { name: 'Terminals' })
const tiles = (page: Page) => rail(page).getByRole('button')
const tile = (page: Page, command: string) => tiles(page).filter({ has: page.getByText(command, { exact: true }) })
const visibleScreen = (page: Page) => page.locator('.xterm-rows:visible')

test('the agent job is listed first with the task icon', async ({ page }) => {
  await page.goto('/panel/compute/review-42')
  const first = tiles(page).first()
  await expect(first).toHaveAttribute('title', 'agent')
  await expect(first).toContainText('claude')
  await expect(first.locator('[data-icon="task"]')).toBeVisible()
  await expect(first.locator('[data-state="running"]')).toBeVisible()
  await expect(tile(page, 'bash').first().locator('[data-icon="terminal"]')).toBeVisible()
  // The agent is shown until another job is picked.
  await expect(first).toHaveAttribute('aria-current', 'true')
  await expect(visibleScreen(page)).toContainText('mock terminal · review-42 · claude')
})

test('a new bash terminal starts, is selected and echoes input', async ({ page }) => {
  await page.goto('/panel/compute/review-42')
  await expect(tiles(page).first()).toHaveAttribute('title', 'agent')
  await page.getByRole('group', { name: 'Start terminal' }).getByRole('button', { name: 'bash' }).click()
  const selected = rail(page).locator('[aria-current="true"]')
  await expect(selected).toContainText('bash')
  await expect(selected.locator('[data-state="running"]')).toBeVisible()
  await expect(selected.locator('[data-icon="terminal"]')).toBeVisible()
  await expect(visibleScreen(page)).toContainText('mock terminal · review-42 · bash')

  await page.locator('.xterm:visible').click()
  await page.keyboard.type('echo hi')
  await expect(visibleScreen(page)).toContainText('echo hi')

  await page.getByRole('button', { name: 'Detach' }).click()
  await expect(page.getByRole('button', { name: 'Reconnect' })).toBeVisible()
})

test('a custom command starts on Enter', async ({ page }) => {
  await page.goto('/panel/compute/review-42')
  const command = `vim notes-${Date.now()}`
  await page.getByRole('textbox', { name: 'Command' }).fill(command)
  await page.getByRole('textbox', { name: 'Command' }).press('Enter')
  await expect(rail(page).locator('[aria-current="true"]')).toContainText(command)
})

test('resize frames are sent and accepted', async ({ page }) => {
  const sent: { rows: number; cols: number }[] = []
  page.on('websocket', (ws) => {
    ws.on('framesent', ({ payload }) => {
      if (typeof payload === 'string' && payload.includes('"resize"')) {
        sent.push(JSON.parse(payload) as { rows: number; cols: number })
      }
    })
  })
  await page.goto('/panel/compute/review-42')
  await expect(visibleScreen(page)).toContainText('[resized to ')
  await expect.poll(() => sent.length).toBeGreaterThan(0)

  const before = sent.length
  const size = page.viewportSize() ?? { width: 1280, height: 720 }
  await page.setViewportSize({ width: size.width - 200, height: size.height - 100 })
  await expect.poll(() => sent.length).toBeGreaterThan(before)
  const last = sent[sent.length - 1]
  await expect(visibleScreen(page)).toContainText(`[resized to ${last.rows}x${last.cols}]`)
})

test('an exited job keeps its output and says exited', async ({ page }) => {
  await page.goto('/panel/compute/review-42')
  const exited = tile(page, 'bash')
    .filter({ has: page.locator('[data-state="exited"]') })
    .first()
  await exited.click()
  await expect(visibleScreen(page)).toContainText('[process exited]')
  await expect(exited.getByText('exited', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Reconnect' })).toHaveCount(0)
})

test('a stopped sandbox says why and offers no start buttons', async ({ page }) => {
  await page.goto('/panel/compute/docs-pass')
  await expect(page.getByText('Sandbox stopped', { exact: true })).toBeVisible()
  await expect(page.getByRole('group', { name: 'Start terminal' })).toHaveCount(0)
  await expect(rail(page)).toHaveCount(0)
})

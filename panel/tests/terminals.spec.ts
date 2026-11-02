// Owned by the terminals feature (TerminalsTab, TerminalPane), mounted on
// the compute feature's SandboxPage.
// TODO(ADR 0008): smoke tests left by the foundation pass; rewrite them once
// SandboxPage renders the terminals at /compute/:name.
import { expect, test } from '@playwright/test'

test('a sandbox page opens under Compute', async ({ page }) => {
  await page.goto('/panel/compute/review-42')
  await expect(page.getByRole('heading', { name: 'SandboxPage' })).toBeVisible()
})

test('terminals live under sandboxes', async ({ request }) => {
  const running = await request.get('/api/sandboxes/review-42/terminals')
  expect(running.ok()).toBe(true)
  const { terminals } = (await running.json()) as { terminals: { id: string; command: string[]; running: boolean }[] }
  expect(terminals.map((t) => [t.command.join(' '), t.running])).toEqual([
    ['claude', true],
    ['bash', false],
  ])
  expect(terminals[0].id).toBe('agent')

  const stopped = await request.get('/api/sandboxes/review-42-2/terminals')
  expect(stopped.status()).toBe(409)
  expect((await stopped.json()).error.message).toBe('sandbox review-42-2 is stopped; start it to use terminals')

  const failed = await request.get('/api/sandboxes/boot-fail')
  expect(await failed.json()).toMatchObject({ status: 'failed', status_error: 'clone failed: repository not found' })
})

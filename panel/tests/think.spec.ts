// Owned by the think feature (src/features/think/): the read-only beliefs,
// decisions, rules and evidence pages over the mock's knowledge seed.
import { expect, test, type Page } from '@playwright/test'

function card(page: Page, id: string) {
  return page.locator(`[data-testid="think-card"][data-id="${id}"]`)
}

test('/ opens the last used mode', async ({ page }) => {
  await page.goto('/panel/think/rules')
  await expect(page.getByTestId('page-title')).toHaveText('Think · Rules')
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/think\/beliefs$/)

  await page.goto('/panel/messages')
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/tasks$/)
})

test('/ honors a stored think mode and /think goes to beliefs', async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('seeded')) {
      localStorage.setItem('agc.panel.mode', 'think')
      sessionStorage.setItem('seeded', '1')
    }
  })
  await page.goto('/panel/')
  await expect(page).toHaveURL(/\/panel\/think\/beliefs$/)
  await page.goto('/panel/think')
  await expect(page).toHaveURL(/\/panel\/think\/beliefs$/)
})

test('beliefs page lists the seeded beliefs with their marks', async ({ page }) => {
  await page.goto('/panel/think/beliefs')
  await expect(page.getByTestId('page-title')).toHaveText('Think · Beliefs')
  await expect(page.getByTestId('think-card')).toHaveCount(8)

  await expect(card(page, 'go-has-one-formatting').locator('[data-label="grounded"]')).toHaveText('grounded')
  await expect(card(page, 'workflows-will-take-large-media').locator('[data-label="opinion"]')).toHaveText('opinion')
  await expect(card(page, 'microsandbox-spike-passed').locator('[data-label="in_doubt"]')).toHaveText('in_doubt')
  await expect(card(page, 'agents-write-go-well')).toContainText(
    'Go is better than Rust in general for writing code with agents.',
  )
  await expect(card(page, 'agents-write-go-well')).toContainText('beliefs/repo.yaml')
})

test('opening a belief shows its why tree and what rests on it', async ({ page }) => {
  await page.goto('/panel/think/beliefs')
  await card(page, 'agents-write-go-well').click()
  await expect(page).toHaveURL(/\/panel\/think\/beliefs\/agents-write-go-well$/)
  await expect(card(page, 'agents-write-go-well').getByRole('link')).toHaveAttribute('aria-current', 'true')

  const pane = page.getByRole('complementary', { name: 'Detail' })
  const tree = pane.getByTestId('why-tree')
  await expect(tree.getByTestId('why-node')).toHaveCount(2)
  const formatting = tree.locator('[data-testid="why-node"][data-id="go-has-one-formatting"]')
  await expect(formatting.locator('[data-label="grounded"]')).toBeVisible()
  await expect(formatting.getByTestId('evidence-row')).toContainText('franco · 2026-11-01')

  const wrong = pane.getByRole('region', { name: 'If this is wrong' })
  await expect(wrong.getByRole('region', { name: 'Beliefs' })).toContainText(
    'A strict, uniform Go style makes code written by agents quick to review.',
  )
  await expect(wrong.getByRole('region', { name: 'Decisions' })).toContainText('0')
  await expect(wrong.getByRole('region', { name: 'Rules' })).toContainText(
    'Never write an explicit `;` in Go outside a 3-clause `for` header.',
  )

  await pane.getByRole('link', { name: 'Close' }).click()
  await expect(page).toHaveURL(/\/panel\/think\/beliefs$/)
  await expect(pane).toHaveCount(0)
})

test('a belief in doubt shows its retracted evidence struck through', async ({ page }) => {
  await page.goto('/panel/think/beliefs/microsandbox-spike-passed')
  const pane = page.getByRole('complementary', { name: 'Detail' })
  const row = pane.getByTestId('why-tree').getByTestId('evidence-row')
  await expect(row).toHaveAttribute('data-retracted', 'true')
  await expect(row).toHaveCSS('text-decoration-line', 'line-through')
  await expect(row).toHaveAttribute('title', /The rig moved to microsandbox 0\.8/)
  await expect(pane.getByRole('region', { name: 'Decisions' })).toContainText(
    'A Task is one microsandbox VM, and agc reads its state live from the runtime.',
  )
})

test('decisions show what they rest on and say when nothing rests on them', async ({ page }) => {
  await page.goto('/panel/think/decisions')
  await expect(page.getByTestId('think-card')).toHaveCount(3)
  await expect(card(page, 'binary-artifacts-stored-by-file-class').getByTestId('rests-on-opinion')).toHaveText(
    'rests on opinion: 1',
  )
  await expect(card(page, 'messaging-over-per-task-vsock').getByTestId('rests-on-doubt')).toHaveText(
    'rests on doubt: 1',
  )

  await card(page, 'messaging-over-per-task-vsock').click()
  const pane = page.getByRole('complementary', { name: 'Detail' })
  // The tree goes through the decision it rests on down to its beliefs.
  await expect(pane.locator('[data-testid="why-node"][data-id="microsandbox-spike-passed"]')).toBeVisible()
  await expect(pane.getByText('Nothing rests on this')).toBeVisible()
})

test('evidence page strikes through the retracted piece', async ({ page }) => {
  await page.goto('/panel/think/evidence')
  await expect(page.getByTestId('page-title')).toHaveText('Think · Evidence')
  await expect(page.getByTestId('think-card')).toHaveCount(6)

  const retracted = card(page, 'microsandbox-spike-on-the-rig')
  await expect(retracted.getByTestId('card-text')).toHaveCSS('text-decoration-line', 'line-through')
  await expect(retracted.getByTestId('retracted')).toHaveText('retracted')
  await expect(retracted.getByTestId('cited-by')).toHaveText('1')

  const live = card(page, 'gofmt-is-the-only-go-formatter')
  await expect(live.getByTestId('card-text')).not.toHaveCSS('text-decoration-line', 'line-through')
  await expect(live.getByTestId('retracted')).toHaveCount(0)
  await expect(card(page, 'agc-stores-everything-in-local-sqlite').getByTestId('cited-by')).toHaveText('0')

  await retracted.click()
  const pane = page.getByRole('complementary', { name: 'Detail' })
  await expect(pane.getByRole('region', { name: 'Cited by' })).toContainText('On the rig, a microsandbox VM')
  await expect(pane.getByRole('region', { name: 'Decisions' })).toContainText('A Task is one microsandbox VM')
})

test('rules are grouped by subject with a check chip on mechanical ones', async ({ page }) => {
  await page.goto('/panel/think/rules')
  const groups = page.getByTestId('rule-group')
  await expect(groups).toHaveCount(2)
  const repo = page.getByRole('region', { name: 'agent-composer code and pull requests' })
  const rig = page.getByRole('region', { name: 'the Linux rig' })
  await expect(repo.getByTestId('think-card')).toHaveCount(5)
  await expect(rig.getByTestId('think-card')).toHaveCount(1)

  await expect(page.getByTestId('check-chip')).toHaveCount(2)
  await expect(card(page, 'go-no-explicit-semicolons').getByTestId('check-chip')).toHaveText(
    'scripts/check-semicolons.sh',
  )
  await expect(card(page, 'go-err-naming').getByTestId('check-chip')).toHaveCount(0)

  await card(page, 'go-gofmt-clean').click()
  const pane = page.getByRole('complementary', { name: 'Detail' })
  await expect(pane.locator('[data-testid="why-node"][data-id="go-has-one-formatting"]')).toBeVisible()
  await expect(pane.getByRole('region', { name: 'If this is wrong' })).toHaveCount(0)
})

test('no knowledge files says so in place', async ({ page }) => {
  await page.route('**/api/knowledge', (route) =>
    route.fulfill({ json: { dir: '/srv/empty', evidence: [], beliefs: [], decisions: [], rules: [] } }),
  )
  await page.goto('/panel/think/beliefs')
  await expect(page.getByText('No knowledge files in /srv/empty')).toBeVisible()
  await expect(page.getByTestId('think-card')).toHaveCount(0)
})

test('a failed load shows the server error verbatim', async ({ page }) => {
  await page.route('**/api/knowledge', (route) =>
    route.fulfill({
      status: 400,
      json: { error: { code: 'EINVALID', message: 'beliefs/repo.yaml: belief x: because y not found' } },
    }),
  )
  await page.goto('/panel/think/decisions')
  await expect(page.getByRole('alert')).toHaveText('beliefs/repo.yaml: belief x: because y not found')
})

test('an unknown id says so in the pane', async ({ page }) => {
  await page.goto('/panel/think/beliefs/no-such-belief')
  await expect(page.getByRole('complementary', { name: 'Detail' })).toContainText(
    'No belief no-such-belief in /home/franco/agent-composer',
  )
})

// Owned by the think feature (src/features/think/): the read-only beliefs,
// decisions, rules, actions and evidence pages over the mock's knowledge
// seed.
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
  await expect(page.getByTestId('think-card')).toHaveCount(14)

  await expect(card(page, 'go-has-one-formatting').locator('[data-label="grounded"]')).toHaveText('grounded')
  await expect(card(page, 'workflows-will-take-large-media').locator('[data-label="opinion"]')).toHaveText('opinion')
  await expect(card(page, 'microsandbox-spike-passed').locator('[data-label="in_doubt"]')).toHaveText('in_doubt')
  await expect(card(page, 'agents-write-go-well').getByTestId('card-text')).toHaveText(
    'I believe Go is better than Rust in general for writing code with agents.',
  )
  await expect(card(page, 'agents-write-go-well').getByTestId('card-text')).toHaveAttribute(
    'title',
    'Go is better than Rust in general for writing code with agents.',
  )
  await expect(card(page, 'workflows-will-take-large-media').getByTestId('card-text')).toHaveText(
    'I think workflows will take large media such as raw video and photo sets as input, but have no evidence yet.',
  )
  await expect(card(page, 'microsandbox-spike-passed').getByTestId('card-text')).toHaveText(
    'I believed on the rig, a microsandbox VM with Claude Code and Codex boots in under two seconds and keeps a detached terminal across disconnects, but its evidence was retracted.',
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
  await expect(page.getByTestId('think-card')).toHaveCount(5)
  await expect(card(page, 'tasks-run-in-microsandbox-vms').getByTestId('card-text')).toHaveText(
    'I decided that a Task is one microsandbox VM, and agc reads its state live from the runtime.',
  )
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
  await expect(page.getByTestId('think-card')).toHaveCount(11)
  await expect(card(page, 'rig-lacks-kvm').getByTestId('card-text')).toHaveText(
    'ana saw that the rig has no KVM on 2026-11-02, written up by codex-cli.',
  )

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
    route.fulfill({
      json: { dir: '/srv/empty', without: [], evidence: [], beliefs: [], decisions: [], rules: [], actions: [] },
    }),
  )
  await page.route('**/api/knowledge/conflicts', (route) => route.fulfill({ json: { conflicts: [] } }))
  await page.goto('/panel/think/beliefs')
  await expect(page.getByText('No knowledge files in /srv/empty')).toBeVisible()
  await expect(page.getByTestId('think-card')).toHaveCount(0)
  await expect(page.getByTestId('conflicts')).toHaveCount(0)
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

test('actions page lists open actions first, then done ones, with checkbox marks', async ({ page }) => {
  await page.goto('/panel/think/actions')
  await expect(page.getByTestId('page-title')).toHaveText('Think · Actions')
  await expect(page.getByRole('navigation', { name: 'Main' }).getByRole('link')).toHaveText([
    'Beliefs',
    'Decisions',
    'Rules',
    'Actions',
    'Evidence',
  ])

  const groups = page.getByTestId('action-group')
  await expect(groups).toHaveCount(2)
  await expect(groups.nth(0)).toHaveAttribute('aria-label', 'To do')
  await expect(groups.nth(1)).toHaveAttribute('aria-label', 'Done')
  const todo = page.getByRole('region', { name: 'To do' })
  const done = page.getByRole('region', { name: 'Done' })
  await expect(todo.getByTestId('think-card')).toHaveCount(4)
  await expect(done.getByTestId('think-card')).toHaveCount(1)
  await expect(todo.locator('[data-testid="think-card"] [data-testid="done-mark"][data-done="false"]')).toHaveCount(4)
  await expect(done.locator('[data-testid="think-card"] [data-testid="done-mark"][data-done="true"]')).toHaveCount(1)

  const cgo = card(page, 'release-builds-with-cgo')
  await expect(cgo.getByTestId('card-text')).toHaveText(
    'I will build the release binaries with cgo so Tasks can run sandboxes from a released agc.',
  )
  await expect(cgo.getByTestId('serves')).toHaveText('tasks-run-in-microsandbox-vms')
  await expect(cgo.getByTestId('rests-on-doubt')).toHaveText('rests on doubt: 1')
  await expect(card(page, 'gitignore-the-local-data-dir').getByTestId('card-text')).toHaveText(
    'I did exclude .agent_composer/ in .gitignore so a project-local secrets.env never enters git.',
  )

  await cgo.click()
  await expect(page).toHaveURL(/\/panel\/think\/actions\/release-builds-with-cgo$/)
  const pane = page.getByRole('complementary', { name: 'Detail' })
  await expect(pane.locator('[data-testid="why-node"][data-id="tasks-run-in-microsandbox-vms"]')).toBeVisible()
  await expect(pane.locator('[data-testid="why-node"][data-id="microsandbox-spike-passed"]')).toBeVisible()
})

test('a contested belief shows its mark and links to what contests it', async ({ page }) => {
  await page.goto('/panel/think/beliefs')
  const c = card(page, 'claim-c')
  await expect(c.locator('[data-label="contested"]')).toHaveText('contested')
  await expect(c.getByTestId('card-text')).toHaveText('I think C is true, though it is contested.')
  await expect(card(page, 'claim-a').getByTestId('card-text')).toHaveText(
    'I believe the rig can run sandboxes, though it is contested.',
  )
  await expect(card(page, 'claim-b').getByTestId('contested-by')).toHaveCount(0)

  await c.getByTestId('contested-by').click()
  await expect(page).toHaveURL(/\/panel\/think\/beliefs\/claim-b$/)

  await page.goto('/panel/think/beliefs/claim-a')
  const pane = page.getByRole('complementary', { name: 'Detail' })
  await pane.getByTestId('contested-by').click()
  await expect(page).toHaveURL(/\/panel\/think\/evidence\/rig-lacks-kvm$/)
})

test('conflicts sit on top of the beliefs page with both sides', async ({ page }) => {
  await page.goto('/panel/think/beliefs')
  const conflicts = page.getByTestId('conflict')
  await expect(conflicts).toHaveCount(2)

  const kvm = conflicts.nth(0)
  await expect(kvm.getByTestId('conflict-text')).toHaveText(
    'ana saw that the rig has no KVM on 2026-11-02, written up by codex-cli, but franco saw that the rig has KVM on 2026-11-01.',
  )
  await expect(kvm).toContainText('Pick one to retract in the files')

  const bc = page.locator('[data-testid="conflict"][data-a="claim-b"]')
  await expect(bc.getByTestId('conflict-text')).toHaveText('I believe B is true, but I think C is true.')
  const sides = bc.getByTestId('conflict-side')
  await expect(sides).toHaveCount(2)
  await expect(sides.nth(0).getByTestId('conflict-leaf')).toHaveText('seen-b')
  await expect(sides.nth(0).getByTestId('conflict-leaf')).toHaveAttribute('data-kind', 'evidence')
  await expect(sides.nth(1).getByTestId('conflict-leaf')).toHaveText('hunch-c')
  await expect(sides.nth(1).getByTestId('conflict-leaf')).toHaveAttribute('data-kind', 'belief')
})

test('a belief with two independent reasons shows the alternative under Or', async ({ page }) => {
  await page.goto('/panel/think/beliefs/either')
  const tree = page.getByRole('complementary', { name: 'Detail' }).getByTestId('why-tree')
  await expect(tree.locator('[data-testid="why-node"][data-id="by-x"]')).toBeVisible()
  const or = tree.getByTestId('why-or')
  await expect(or).toHaveCount(1)
  await expect(or.getByRole('heading')).toHaveText('Or')
  await expect(or.getByTestId('evidence-row')).toContainText('ana · 2026-11-02')
  await expect(or.getByTestId('why-node')).toHaveCount(0)
})

test('what-if relabels the page without the item, then restores it', async ({ page }) => {
  await page.goto('/panel/think/beliefs/go-has-one-formatting')
  const agents = card(page, 'agents-write-go-well')
  await expect(agents.locator('[data-label="grounded"]')).toBeVisible()
  await expect(page.getByTestId('what-if-banner')).toHaveCount(0)

  const pane = page.getByRole('complementary', { name: 'Detail' })
  const toggle = pane.getByRole('switch', { name: 'What if this were gone?' })
  await toggle.click()
  await expect(page).toHaveURL(/\?without=go-has-one-formatting$/)
  await expect(page.getByTestId('what-if-banner')).toHaveText('Showing labels without go-has-one-formatting')
  await expect(page.getByTestId('page-title')).toHaveText('Think · Beliefs · without go-has-one-formatting')
  await expect(agents.locator('[data-label="in_doubt"]')).toBeVisible()
  await expect(agents.getByTestId('card-text')).toHaveText(
    'I would doubt that Go is better than Rust in general for writing code with agents, without go-has-one-formatting.',
  )
  // Untouched beliefs keep their labels.
  await expect(card(page, 'go-syntax-is-small').locator('[data-label="grounded"]')).toBeVisible()
  // What rests on it in the pane is relabeled from the same answer.
  const resting = pane.getByRole('region', { name: 'If this is wrong' }).getByRole('region', { name: 'Beliefs' })
  await expect(resting.locator('[data-label="in_doubt"]')).toHaveCount(2)

  await toggle.click()
  await expect(page).toHaveURL(/\/panel\/think\/beliefs\/go-has-one-formatting$/)
  await expect(page.getByTestId('what-if-banner')).toHaveCount(0)
  await expect(page.getByTestId('page-title')).toHaveText('Think · Beliefs')
  await expect(agents.locator('[data-label="grounded"]')).toBeVisible()
})

test('what-if on evidence relabels the beliefs citing it', async ({ page }) => {
  await page.goto('/panel/think/evidence/gofmt-is-the-only-go-formatter')
  await page.getByRole('switch', { name: 'What if this were gone?' }).click()
  await expect(page.getByTestId('what-if-banner')).toHaveText('Showing labels without gofmt-is-the-only-go-formatter')
  const cited = page.getByRole('region', { name: 'Cited by' })
  await expect(cited.locator('[data-label="in_doubt"]')).toHaveCount(1)
})

test('a what-if on an unknown id shows the server error verbatim', async ({ page }) => {
  await page.goto('/panel/think/beliefs?without=no-such-id')
  await expect(page.getByRole('alert')).toHaveText('without: no-such-id is not a loaded belief or evidence')
})

test('doubt lists open actions and what still stands on other support', async ({ page }) => {
  await page.goto('/panel/think/beliefs/microsandbox-spike-passed')
  const wrong = page.getByRole('complementary', { name: 'Detail' }).getByRole('region', { name: 'If this is wrong' })
  await expect(wrong.getByRole('region', { name: 'Actions' })).toContainText(
    'Build the release binaries with cgo so Tasks can run sandboxes from a released agc.',
  )
  await expect(wrong.getByRole('region', { name: 'Actions' }).getByRole('link')).toHaveCount(4)
  await expect(wrong.getByRole('region', { name: 'Still standing (other support)' })).toHaveCount(0)

  await page.goto('/panel/think/beliefs/by-x')
  const byX = page.getByRole('complementary', { name: 'Detail' }).getByRole('region', { name: 'If this is wrong' })
  const standing = byX.getByRole('region', { name: 'Still standing (other support)' })
  await expect(standing).toContainText('W holds.')
  await expect(standing).toHaveCSS('opacity', '0.6')
  await expect(byX.getByRole('region', { name: 'Beliefs' })).toContainText('0')
})

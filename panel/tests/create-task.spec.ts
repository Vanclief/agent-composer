// Owned by the create-task feature (CreateTaskDialog).
import { expect, test, type Page } from '@playwright/test'

async function openDialog(page: Page) {
  await page.goto('/panel/tasks')
  await page.getByRole('button', { name: 'New task' }).click()
  const dialog = page.getByRole('dialog', { name: 'New task' })
  await expect(dialog).toBeVisible()
  return dialog
}

test('the title derives the name live, the name is editable, and Create says why it is off', async ({ page }) => {
  const dialog = await openDialog(page)
  const create = dialog.locator('[data-create]')
  await expect(create).toHaveText('Create: needs a title')
  await expect(create).toBeDisabled()

  const title = dialog.getByRole('textbox', { name: 'Title' })
  await expect(title).toBeFocused()
  await title.pressSequentially('Review facebook ads')
  const name = dialog.locator('[data-name]')
  await expect(name).toHaveValue('review-facebook-ads')
  await expect(dialog.locator('[data-own-channel]')).toContainText('review-facebook-ads')
  await expect(create).toHaveText('Create task')
  await expect(create).toBeEnabled()

  await title.fill('42 things')
  await expect(name).toHaveValue('task-42-things')

  await title.fill('!!!')
  await expect(name).toHaveValue('')
  await expect(create).toHaveText('Create: needs a letter or digit')

  // An edited name stops following the title until reset.
  await name.fill('my-name')
  await title.fill('Something else')
  await expect(name).toHaveValue('my-name')
  await expect(dialog.locator('[data-own-channel]')).toContainText('my-name')
  await name.fill('')
  await expect(create).toHaveText('Create: needs a name')
  await dialog.locator('[data-name-reset]').click()
  await expect(name).toHaveValue('something-else')

  // Without a sandbox the brief is all there is, so it is required.
  await dialog.locator('[data-compute]').click()
  await expect(create).toHaveText('Create: needs a brief')
  await expect(create).toBeDisabled()
  await dialog.locator('[data-brief]').fill('Do it.')
  await expect(create).toHaveText('Create task')
})

test('a harness shows the server defaults as values', async ({ page }) => {
  const dialog = await openDialog(page)
  await expect(dialog.locator('[data-assignee="claude_code"]')).toHaveAttribute('data-state', 'on')
  await expect(dialog.locator('[data-model]')).toHaveValue('claude-opus-5-5')
  await expect(dialog.locator('[data-manager]')).toHaveValue('franco')
  await expect(dialog.locator('[data-owner]')).toHaveValue('franco')
  await dialog.getByRole('textbox', { name: 'Title' }).fill('Hint check')
  await expect(dialog.locator('[data-agent-hint]')).toHaveText(
    'The manager answers the agent in #hint-check and raises to franco what only the owner can decide.',
  )
  await expect(dialog.locator('[data-compute]')).toHaveAttribute('aria-checked', 'true')
  await expect(dialog.locator('[data-brief]')).toHaveAttribute('placeholder', 'What to do, and what done looks like')
  await expect(dialog.locator('[data-command]')).toHaveAttribute('placeholder', 'empty: agc runs the agent for you')
  const summary = dialog.locator('[data-summary]')
  await expect(summary).toHaveText('claude_code (claude-opus-5-5) · manager franco · owner franco · 2 cpus 4 GB')

  await dialog.locator('[data-repo]').fill('github.com/org/repo')
  await expect(summary).toHaveText(
    'claude_code (claude-opus-5-5) · manager franco · owner franco · 2 cpus 4 GB · repo github.com/org/repo',
  )

  // Another harness has no default model here: the field is empty, not hidden.
  await dialog.locator('[data-assignee="codex_cli"]').click()
  await expect(dialog.locator('[data-model]')).toHaveValue('')
  await expect(summary).toHaveText('codex_cli · manager franco · owner franco · 2 cpus 4 GB · repo github.com/org/repo')
})

test('a person hides the model and turns compute off', async ({ page }) => {
  const dialog = await openDialog(page)
  const create = dialog.locator('[data-create]')
  await dialog.getByRole('textbox', { name: 'Title' }).fill('Person check')

  await dialog.locator('[data-assignee="person"]').click()
  await expect(dialog.locator('[data-model]')).toHaveCount(0)
  await expect(dialog.locator('[data-compute]')).toHaveAttribute('aria-checked', 'false')
  await expect(dialog.locator('[data-card="compute"]')).toContainText('No sandbox: a person does this task')
  await expect(dialog.locator('[data-repo]')).toHaveCount(0)
  await expect(dialog.locator('[data-person]')).toBeFocused()
  await expect(create).toHaveText("Create: needs the person's name")
  await dialog.locator('[data-person]').fill('alice')
  await expect(create).toHaveText('Create: needs a brief')
  await expect(dialog.locator('[data-summary]')).toHaveText('alice · manager franco · owner franco · no sandbox')

  // Back to a harness: its model and the sandbox return.
  await dialog.locator('[data-assignee="claude_code"]').click()
  await expect(dialog.locator('[data-model]')).toHaveValue('claude-opus-5-5')
  await expect(dialog.locator('[data-compute]')).toHaveAttribute('aria-checked', 'true')
  await expect(create).toHaveText('Create task')
})

test('the for picker lists open actions first, then decisions', async ({ page }) => {
  const dialog = await openDialog(page)
  await dialog.locator('[data-for]').click()
  const options = dialog.locator('[data-for-option]')
  await expect(options.first()).toHaveAttribute('data-for-option', 'attach-rulesets-to-task-specs')
  await expect(dialog.locator('[data-for-picker]')).toContainText('Decisions')

  await dialog.getByPlaceholder('action or decision id').fill('cgo')
  await expect(options).toHaveCount(1)
  await options.first().click()
  await expect(dialog.locator('[data-for-picker]')).toHaveCount(0)
  await expect(dialog.locator('[data-for="release-builds-with-cgo"]')).toContainText('Build the release binaries')

  await dialog.locator('[data-for-clear]').click()
  await expect(dialog.locator('[data-for=""]')).toBeVisible()
})

test('chips toggle and secrets show their hosts input', async ({ page }) => {
  const dialog = await openDialog(page)

  const cpus = dialog.locator('[data-cpus]')
  await expect(cpus.getByRole('radio', { name: '2', exact: true })).toHaveAttribute('data-state', 'on')
  await cpus.getByRole('radio', { name: '4', exact: true }).click()
  await expect(cpus.getByRole('radio', { name: '4', exact: true })).toHaveAttribute('data-state', 'on')
  await expect(cpus.getByRole('radio', { name: '2', exact: true })).toHaveAttribute('data-state', 'off')

  const github = dialog.locator('[data-secret="GITHUB_TOKEN"]')
  await expect(github).toHaveAttribute('aria-pressed', 'false')
  await github.click()
  await expect(github).toHaveAttribute('aria-pressed', 'true')
  await expect(dialog.locator('[data-secret-hosts="GITHUB_TOKEN"]')).toHaveCount(0)
  await github.click()
  await expect(github).toHaveAttribute('aria-pressed', 'false')

  // CUSTOM has no known hosts, so selecting it asks for them.
  await dialog.locator('[data-secret="CUSTOM"]').click()
  await expect(dialog.locator('[data-secret-hosts="CUSTOM"]')).toBeFocused()

  // The repo dir chip follows the repo, hidden while it is empty.
  await expect(dialog.locator('[data-repo-dir]')).toHaveCount(0)
  await dialog.locator('[data-repo]').fill('https://github.com/org/widgets.git')
  await expect(dialog.locator('[data-repo-dir]')).toHaveText('/home/agent/widgets')

  // Setup is collapsed until clicked, then counts its lines.
  await expect(dialog.locator('[data-setup]')).toHaveCount(0)
  await dialog.locator('[data-setup-toggle]').click()
  await dialog.locator('[data-setup]').fill('go mod download\nmake build\n')
  await expect(dialog.locator('[data-setup-lines]')).toHaveText('2')
})

test('the YAML toggle round-trips a memory change both ways', async ({ page }) => {
  const dialog = await openDialog(page)
  const memory = dialog.locator('[data-memory]')
  await memory.getByRole('radio', { name: '8 GB' }).click()

  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  const raw = dialog.locator('[data-raw]')
  await expect(raw).toHaveValue(/^memory: 8192$/m)

  await raw.fill((await raw.inputValue()).replace('memory: 8192', 'memory: 16384'))
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  await expect(memory.getByRole('radio', { name: '16 GB' })).toHaveAttribute('data-state', 'on')
  await expect(memory.getByRole('radio', { name: '8 GB' })).toHaveAttribute('data-state', 'off')

  // A free value that is not a preset shows as an extra lit chip.
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  await raw.fill('memory: 3072\n')
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  await expect(memory.getByRole('radio', { name: '3 GB' })).toHaveAttribute('data-state', 'on')

  // Broken YAML: the parser's error in place, Create says why it is off.
  await dialog.getByRole('textbox', { name: 'Title' }).fill('Raw check')
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  await raw.fill('memory: [8\n')
  await expect(dialog.getByRole('alert')).toBeVisible()
  await expect(dialog.locator('[data-create]')).toHaveText('Create: fix YAML')
  await expect(dialog.locator('[data-create]')).toBeDisabled()
  // The toggle keeps the YAML open until it parses.
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  await expect(raw).toBeVisible()
})

test('Start from loads a saved spec into the cards', async ({ page }) => {
  const dialog = await openDialog(page)
  await dialog.locator('[data-tile="backend-review"]').click()
  await expect(dialog.locator('[data-tile="backend-review"]')).toHaveAttribute('aria-pressed', 'true')
  await expect(dialog.locator('[data-repo]')).toHaveValue('github.com/vanclief/agent-composer')
  await expect(dialog.locator('[data-ref]')).toHaveValue('master')
  await expect(dialog.locator('[data-repo-dir]')).toHaveText('/home/agent/agent-composer')
  await expect(dialog.locator('[data-memory]').getByRole('radio', { name: '4 GB' })).toHaveAttribute('data-state', 'on')

  await dialog.locator('[data-tile=""]').click()
  await expect(dialog.locator('[data-repo]')).toHaveValue('')
})

test('Save spec saves the form and shows it as a tile', async ({ page }) => {
  const dialog = await openDialog(page)
  const name = `Saved ${Date.now()}`
  const slug = name.toLowerCase().replace(' ', '-')
  await dialog.locator('[data-repo]').fill('github.com/org/saved')

  await dialog.locator('[data-save-spec]').click()
  const input = dialog.locator('[data-spec-name]')
  await expect(input).toBeFocused()
  await expect(dialog.getByRole('button', { name: 'Save: needs a name' })).toBeDisabled()
  await input.fill(name)
  await dialog.getByRole('button', { name: 'Save', exact: true }).click()

  const tile = dialog.locator(`[data-tile="${slug}"]`)
  await expect(tile).toBeVisible()
  await expect(tile).toHaveAttribute('aria-pressed', 'true')
  const saved = await page.request.get(`/api/specs/${slug}`)
  expect((await saved.json()).yaml).toContain('repo: github.com/org/saved')
})

test('Create sends the spec and opens the new task', async ({ page }) => {
  const dialog = await openDialog(page)
  const stamp = Date.now()
  const title = `Create check ${stamp}`
  const name = `create-check-${stamp}`

  await dialog.getByRole('textbox', { name: 'Title' }).fill(title)
  await dialog.locator('[data-cpus]').getByRole('radio', { name: '4', exact: true }).click()
  await dialog.locator('[data-memory]').getByRole('radio', { name: '8 GB' }).click()
  await dialog.locator('[data-repo]').fill('github.com/org/repo')
  await dialog.locator('[data-ref]').fill('main')
  await dialog.locator('[data-secret="GITHUB_TOKEN"]').click()
  await dialog.locator('[data-secret="CUSTOM"]').click()
  await dialog.locator('[data-secret-hosts="CUSTOM"]').fill('api.example.com, example.com')

  await dialog.getByRole('button', { name: 'Add variable' }).click()
  await dialog.locator('[data-env-key]').last().fill('FOO')
  await dialog.locator('[data-env-value]').last().fill('bar')
  // A row without a key is dropped.
  await dialog.getByRole('button', { name: 'Add variable' }).click()
  await dialog.locator('[data-env-value]').last().fill('orphan')

  await dialog.getByRole('button', { name: 'Add volume' }).click()
  const volume = dialog.locator('[data-volume]').last()
  await volume.locator('[data-volume-source]').fill('data')
  await volume.locator('[data-volume-dest]').fill('/home/agent/data')
  await volume.locator('[data-volume-rw]').click()

  const draft = dialog.locator('[data-channel-draft]')
  await draft.fill('releases')
  await draft.press('Enter')
  await draft.fill('#alerts')
  await draft.press('Enter')
  await dialog.locator('[data-channel="#alerts"] [data-channel-read]').click()
  await expect(dialog.locator('[data-card="channels"] [data-count]')).toHaveText('3')

  await dialog.locator('[data-brief]').fill('Ship it.')
  await dialog.locator('[data-for]').click()
  await dialog.locator('[data-for-option="release-builds-with-cgo"]').click()
  await dialog.locator('[data-command]').fill('bash -l')

  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/tasks'))
  const answered = page.waitForResponse((r) => r.request().method() === 'POST' && r.url().endsWith('/api/tasks'))
  await dialog.locator('[data-create]').click()
  const spec = {
    image: 'agc-agent',
    cpus: 4,
    memory: 8192,
    repo: 'github.com/org/repo',
    ref: 'main',
    secrets: ['GITHUB_TOKEN', 'CUSTOM@api.example.com,example.com'],
    env: { FOO: 'bar' },
    volumes: ['data:/home/agent/data:rw'],
    channels: ['#releases', '#alerts:read'],
  }
  expect((await posted).postData()).toBe(
    JSON.stringify({
      title,
      brief: 'Ship it.',
      assignee: 'claude_code',
      model: 'claude-opus-5-5',
      manager: 'franco',
      owner: 'franco',
      for: 'release-builds-with-cgo',
      notify: true,
      accept: [],
      sandbox: { ...spec, command: 'bash -l' },
    }),
  )
  // Returns at once: the task is open and its sandbox starting.
  const created = await (await answered).json()
  expect(created.status).toBe('open')
  expect(created.sandboxes[0].status).toBe('starting')

  await expect(page).toHaveURL(new RegExp(`/panel/tasks/${name}$`))
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByTestId('task-header').locator('[data-status]')).toHaveText('open')
  const task = await (await page.request.get(`/api/tasks/${name}`)).json()
  expect(task.title).toBe(title)
  expect(task.for).toBe('release-builds-with-cgo')
  expect(task.sandboxes[0].spec).toEqual(spec)
  expect(task.sandboxes[0].command).toBe('bash -l')
})

test('a person task is created with no sandbox', async ({ page }) => {
  const dialog = await openDialog(page)
  const stamp = Date.now()
  const name = `person-check-${stamp}`
  await dialog.getByRole('textbox', { name: 'Title' }).fill(`Person check ${stamp}`)
  await dialog.locator('[data-brief]').fill('Call the bank.')
  await dialog.locator('[data-assignee="person"]').click()
  await dialog.locator('[data-person]').fill('alice')
  await expect(dialog.locator('[data-summary]')).toContainText('no sandbox')

  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/tasks'))
  await dialog.locator('[data-create]').click()
  const body = JSON.parse((await posted).postData() ?? '{}')
  expect(body.assignee).toBe('alice')
  expect(body.model).toBe('')
  expect(body.manager).toBe('franco')
  expect(body.owner).toBe('franco')
  expect(body.sandbox).toBeUndefined()

  await expect(page).toHaveURL(new RegExp(`/panel/tasks/${name}$`))
  const task = await (await page.request.get(`/api/tasks/${name}`)).json()
  expect(task.assignee).toBe('alice')
  expect(task.status).toBe('open')
  expect(task.sandboxes).toEqual([])
})

test('a backend error shows verbatim and keeps the dialog open', async ({ page }) => {
  const dialog = await openDialog(page)
  // review-42 exists in the mock.
  await dialog.getByRole('textbox', { name: 'Title' }).fill('Review 42')
  await dialog.locator('[data-create]').click()
  await expect(dialog.getByRole('alert')).toHaveText('task review-42 already exists')
  await expect(dialog).toBeVisible()
})

// Screenshots for review, not a check:
//   CREATE_TASK_SHOT=/tmp/agc-panel npx playwright test create-task -g screenshot
test('screenshot', async ({ page }) => {
  const dir = process.env.CREATE_TASK_SHOT
  test.skip(!dir, 'set CREATE_TASK_SHOT to a directory')
  await page.setViewportSize({ width: 1440, height: 900 })
  // Mouse off the controls, transitions and the open animation done.
  const shoot = async (path: string) => {
    await page.mouse.move(0, 0)
    await page.waitForTimeout(400)
    await page.screenshot({ path })
  }
  for (const theme of ['dark']) {
    const dialog = await openDialog(page)
    await shoot(`${dir}/newtask-blank-${theme}.png`)

    await dialog.getByRole('textbox', { name: 'Title' }).fill('Review facebook ads')
    await dialog
      .locator('[data-brief]')
      .fill('Find the ads that lose money and pause them.\nDone: a list in the channel.')
    await dialog.locator('[data-for]').click()
    await dialog.locator('[data-for-option="attach-rulesets-to-task-specs"]').click()
    await dialog.locator('[data-tile="backend-review"]').click()
    await dialog.locator('[data-cpus]').getByRole('radio', { name: '4', exact: true }).click()
    await dialog.locator('[data-memory]').getByRole('radio', { name: '8 GB' }).click()
    await dialog.locator('[data-setup-toggle]').click()
    await dialog.locator('[data-setup]').fill('go mod download\nmake build')
    await dialog.locator('[data-setup-toggle]').click()
    await dialog.locator('[data-secret="GITHUB_TOKEN"]').click()
    await dialog.locator('[data-secret="CUSTOM"]').click()
    await dialog.locator('[data-secret-hosts="CUSTOM"]').fill('api.example.com')
    await dialog.getByRole('button', { name: 'Add variable' }).click()
    await dialog.locator('[data-env-key]').last().fill('FOO')
    await dialog.locator('[data-env-value]').last().fill('bar')
    await dialog.getByRole('button', { name: 'Add variable' }).click()
    await dialog.getByRole('button', { name: 'Add volume' }).click()
    await dialog.locator('[data-volume-source]').last().fill('data')
    await dialog.locator('[data-volume-dest]').last().fill('/home/agent/data')
    await dialog.locator('[data-volume-rw]').last().click()
    await dialog.getByRole('button', { name: 'Add volume' }).click()
    await dialog.locator('[data-kind="host"]').last().click()
    const draft = dialog.locator('[data-channel-draft]')
    await draft.fill('alerts:read')
    await draft.press('Enter')
    await draft.fill('releases')
    await draft.press('Enter')
    await dialog.getByRole('textbox', { name: 'Title' }).focus()
    await shoot(`${dir}/newtask-${theme}.png`)

    await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
    await shoot(`${dir}/newtask-raw-${theme}.png`)

    await dialog.locator('[data-assignee="person"]').click()
    await dialog.locator('[data-person]').fill('alice')
    await shoot(`${dir}/newtask-person-${theme}.png`)
  }
})

test('"Accepted when" sends one criterion per line, empty lines dropped', async ({ page }) => {
  const dialog = await openDialog(page)
  const stamp = Date.now()
  const name = `criteria-check-${stamp}`
  await dialog.getByRole('textbox', { name: 'Title' }).fill(`Criteria check ${stamp}`)
  await dialog.locator('[data-compute]').click()
  await dialog.locator('[data-brief]').fill('Polish the stop icon.')
  const accept = dialog.getByRole('textbox', { name: 'Accepted when' })
  await expect(accept).toHaveAttribute('placeholder', '`npm run check` passes')
  await accept.fill('`npm run check` passes\n\n  the Stop icon is filled  \n\n')

  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/tasks'))
  await dialog.locator('[data-create]').click()
  const body = JSON.parse((await posted).postData() ?? '{}')
  expect(body.accept).toEqual(['`npm run check` passes', 'the Stop icon is filled'])

  await expect(page).toHaveURL(new RegExp(`/panel/tasks/${name}$`))
  await expect(page.getByRole('region', { name: 'Accepted when' }).getByTestId('criterion')).toHaveText([
    '`npm run check` passes',
    'the Stop icon is filled',
  ])
})

test('NAME, NAME@hosts and NAME as ENV@hosts round-trip through the form unchanged', async ({ page }) => {
  const dialog = await openDialog(page)
  const entries = [
    'GITHUB_TOKEN',
    'CUSTOM@api.example.com,example.com',
    'ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN@api.anthropic.com',
  ]
  const yaml = 'image: agc-agent\nsecrets:\n' + entries.map((e) => `  - ${e}\n`).join('')

  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  const raw = dialog.locator('[data-raw]')
  await raw.fill(yaml)
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()

  // Read into the form: all three picked, the alias lit and on the chip.
  for (const name of ['GITHUB_TOKEN', 'CUSTOM', 'ANTHROPIC_WORK']) {
    await expect(dialog.locator(`[data-secret="${name}"]`)).toHaveAttribute('aria-pressed', 'true')
  }
  await expect(dialog.locator('[data-secret-hosts="CUSTOM"]')).toHaveValue('api.example.com, example.com')
  await expect(dialog.locator('[data-secret="ANTHROPIC_WORK"]')).toHaveText('ANTHROPIC_WORK → CLAUDE_CODE_OAUTH_TOKEN')
  await expect(
    dialog.locator('[data-secret-as="ANTHROPIC_WORK"] [data-env="CLAUDE_CODE_OAUTH_TOKEN"]'),
  ).toHaveAttribute('data-state', 'on')

  // Written back unchanged.
  await dialog.getByRole('button', { name: 'YAML', exact: true }).click()
  await expect(raw).toHaveValue(yaml)
})

test('a picked secret no harness reads offers the harness variables, and one becomes NAME as ENV', async ({ page }) => {
  const dialog = await openDialog(page)
  const stamp = Date.now()
  await dialog.getByRole('textbox', { name: 'Title' }).fill(`Account check ${stamp}`)

  // Read directly by a harness, or bound to a host that is no provider's
  // API: no choice.
  await dialog.locator('[data-secret="CLAUDE_CODE_OAUTH_TOKEN"]').click()
  await dialog.locator('[data-secret="GITHUB_TOKEN"]').click()
  await expect(dialog.locator('[data-secret-as="CLAUDE_CODE_OAUTH_TOKEN"]')).toHaveCount(0)
  await expect(dialog.locator('[data-secret-as="GITHUB_TOKEN"]')).toHaveCount(0)
  await dialog.locator('[data-secret="CLAUDE_CODE_OAUTH_TOKEN"]').click()

  // Not picked: no choice yet. Picked: claude_code's variables, none lit.
  await expect(dialog.locator('[data-secret-as]')).toHaveCount(0)
  const work = dialog.locator('[data-secret="ANTHROPIC_WORK"]')
  await work.click()
  const as = dialog.locator('[data-secret-as="ANTHROPIC_WORK"] [data-env]')
  await expect(as).toHaveText(['CLAUDE_CODE_OAUTH_TOKEN', 'ANTHROPIC_API_KEY'])
  await expect(as.and(page.locator('[data-state="on"]'))).toHaveCount(0)
  await expect(work).toHaveText('ANTHROPIC_WORK')

  // Another harness offers its own variables.
  await dialog.locator('[data-card="agent"]').getByRole('radio', { name: 'codex_cli' }).click()
  await expect(as).toHaveText(['CODEX_API_KEY'])
  await dialog.locator('[data-card="agent"]').getByRole('radio', { name: 'claude_code' }).click()

  await dialog.locator('[data-secret-as="ANTHROPIC_WORK"] [data-env="CLAUDE_CODE_OAUTH_TOKEN"]').click()
  await expect(dialog.locator('[data-env="CLAUDE_CODE_OAUTH_TOKEN"]')).toHaveAttribute('data-state', 'on')
  await expect(work).toHaveText('ANTHROPIC_WORK → CLAUDE_CODE_OAUTH_TOKEN')

  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/tasks'))
  await dialog.locator('[data-create]').click()
  const body = JSON.parse((await posted).postData() ?? '{}')
  expect(body.sandbox.secrets).toEqual(['GITHUB_TOKEN', 'ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN'])
})

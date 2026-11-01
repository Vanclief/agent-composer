// Owned by the create-task feature (CreateTaskDialog).
import { expect, test, type Page } from '@playwright/test'

async function openDialog(page: Page) {
  await page.goto('/panel/tasks')
  await page.getByRole('button', { name: 'New task' }).click()
  const dialog = page.getByRole('dialog', { name: 'New task' })
  await expect(dialog).toBeVisible()
  return dialog
}

test('the title derives the slug live and gates Create', async ({ page }) => {
  const dialog = await openDialog(page)
  const create = dialog.locator('[data-create]')
  await expect(create).toHaveText('Create: needs a title')
  await expect(create).toBeDisabled()

  const title = dialog.getByRole('textbox', { name: 'Title' })
  await expect(title).toBeFocused()
  await title.pressSequentially('Review facebook ads')
  await expect(dialog.locator('[data-slug]')).toHaveText('review-facebook-ads')
  await expect(dialog.locator('[data-own-channel]')).toContainText('review-facebook-ads')
  await expect(create).toHaveText('Create task')
  await expect(create).toBeEnabled()

  await title.fill('42 things')
  await expect(dialog.locator('[data-slug]')).toHaveText('task-42-things')

  await title.fill('!!!')
  await expect(dialog.locator('[data-slug]')).toHaveCount(0)
  await expect(create).toHaveText('Create: needs a letter or digit')
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

  const posted = page.waitForRequest((r) => r.method() === 'POST' && r.url().endsWith('/api/tasks'))
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
  expect((await posted).postData()).toBe(JSON.stringify({ title, spec }))

  await expect(page).toHaveURL(new RegExp(`/panel/tasks/${name}/terminals$`))
  await expect(page.getByRole('dialog')).toHaveCount(0)
  const task = await (await page.request.get(`/api/tasks/${name}`)).json()
  expect(task.title).toBe(title)
  expect(task.spec).toEqual(spec)
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
  for (const theme of ['dark', 'light']) {
    await page.addInitScript((value) => localStorage.setItem('agc.panel.theme', value), theme)
    const dialog = await openDialog(page)
    await shoot(`${dir}/newtask-blank-${theme}.png`)

    await dialog.getByRole('textbox', { name: 'Title' }).fill('Review facebook ads')
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
  }
})

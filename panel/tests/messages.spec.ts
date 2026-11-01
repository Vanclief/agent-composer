// Owned by the messages feature (MessagesTab, MessagesPage).
import { expect, test, type Page } from '@playwright/test'

// Every test shares one mock, and a joined name stays held, so each test
// posts under its own name unless it means to collide.
function uniqueName(prefix = 'h') {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`
}

async function actAs(page: Page, name: string) {
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), name)
}

test('a task messages tab shows its channel with human and agent authors', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/tasks/review-42/messages')
  const list = page.getByTestId('message-list')
  for (const body of [
    'Cloned the repo and ran the tests. 3 failures in workflow/.',
    'Look at the loop node first.',
    'Reproduced 2 of 3 locally. The third looks flaky.',
    'Fixed the loop node. Pushing to review-42.',
  ]) {
    await expect(list.getByText(body)).toBeVisible()
  }
  await expect(list.locator('[data-kind="human"]', { hasText: '@franco' })).toBeVisible()
  await expect(list.locator('[data-kind="agent"]', { hasText: '@reviewer' }).first()).toBeVisible()
  await expect(list.locator('[data-kind="agent"]', { hasText: '@tester' })).toBeVisible()
})

test('joining and posting shows the message in the list', async ({ page }) => {
  const name = uniqueName()
  const slug = uniqueName('c')
  await actAs(page, name)
  await page.goto(`/panel/messages/${slug}`)
  await expect(page.getByText('No messages')).toBeVisible()
  await expect(page.getByRole('button', { name: `Identity @${name}` })).toBeVisible()

  const send = page.getByRole('button', { name: 'Send: empty' })
  await expect(send).toBeDisabled()

  const box = page.getByRole('textbox', { name: `Message #${slug}` })
  await box.fill('first line')
  await box.press('Shift+Enter')
  await box.pressSequentially('second line')
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled()
  await box.press('Enter')

  const list = page.getByTestId('message-list')
  await expect(list.getByText('first line\nsecond line')).toBeVisible()
  await expect(list.locator('[data-kind="human"]', { hasText: `@${name}` })).toBeVisible()
  await expect(box).toHaveValue('')

  // The participant is kept, so a reload does not trip over its own name.
  await page.reload()
  await expect(list.getByText('first line\nsecond line')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Send: empty' })).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('a taken name shows the 409 verbatim and Join as… picks another', async ({ page }) => {
  // @reviewer is an active agent in the mock.
  await actAs(page, 'reviewer')
  await page.goto('/panel/tasks/review-42/messages')
  await expect(page.getByRole('alert')).toHaveText('name @reviewer is already in use')
  await expect(page.getByRole('button', { name: 'Send: join first' })).toBeDisabled()

  const name = uniqueName()
  await page.getByRole('button', { name: 'Join as…' }).click()
  await page.getByRole('textbox', { name: 'Name' }).fill(name)
  await page.getByRole('button', { name: 'Join', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.getByRole('button', { name: `Identity @${name}` })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Send: empty' })).toBeVisible()
})

test('messages page rail lists channels and navigates', async ({ page }) => {
  await actAs(page, uniqueName())
  await page.goto('/panel/messages')
  await expect(page.getByTestId('page-title')).toHaveText('Messages')
  await expect(page.getByText('Pick a channel')).toBeVisible()

  const rail = page.getByRole('navigation', { name: 'Channels' })
  await expect(rail.getByRole('link', { name: /review-42/ })).toBeVisible()
  await rail.getByRole('link', { name: /releases/ }).click()
  await expect(page).toHaveURL(/\/panel\/messages\/releases$/)
  await expect(page.getByTestId('page-title')).toHaveText('Messages #releases')
  await expect(page.getByText('v0.4.0 tagged.')).toBeVisible()
  await expect(rail.getByRole('link', { name: /releases/ })).toHaveAttribute('aria-current', 'page')
})

test('the live feed shows a message another client posts', async ({ page, request }) => {
  const slug = uniqueName('c')
  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  await expect(page.getByText('No messages')).toBeVisible()
  // The read already answered, so only the wait loop can deliver it.

  const joined = await request.post('/api/msg/join', { data: { name: uniqueName('o') } })
  expect(joined.ok()).toBeTruthy()
  const { id } = await joined.json()
  const posted = await request.post('/api/msg/post', {
    data: { participant_id: id, channel: `#${slug}`, body: 'hello from elsewhere' },
  })
  expect(posted.ok()).toBeTruthy()

  await expect(page.getByTestId('message-list').getByText('hello from elsewhere')).toBeVisible()
})

test('a long channel opens at the bottom and new messages above the fold show a pill', async ({ page, request }) => {
  const slug = uniqueName('c')
  const joined = await request.post('/api/msg/join', { data: { name: uniqueName('o') } })
  const { id } = await joined.json()
  const postAs = (body: string) =>
    request.post('/api/msg/post', { data: { participant_id: id, channel: `#${slug}`, body } })
  for (let i = 1; i <= 40; i++) {
    await postAs(`line ${i}`)
  }

  await actAs(page, uniqueName())
  await page.goto(`/panel/messages/${slug}`)
  const list = page.getByTestId('message-list')
  await expect(list.getByText('line 40', { exact: true })).toBeInViewport()

  await list.evaluate((el) => el.scrollTo({ top: 0 }))
  await expect(list.getByText('line 1', { exact: true })).toBeInViewport()
  await postAs('line 41')
  const pill = page.getByRole('button', { name: 'New messages' })
  await expect(pill).toBeVisible()
  await pill.click()
  await expect(list.getByText('line 41', { exact: true })).toBeInViewport()
  await expect(pill).toHaveCount(0)
})

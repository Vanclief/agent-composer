// Avatars and presence: src/components/avatar.tsx and
// src/features/messages/presence.ts, as applied in the channel header,
// the Agents list and the top bar. Every test posts in its own channel.
import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test'

function unique(prefix: string) {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`
}

async function actAs(page: Page, name: string) {
  await page.addInitScript((n) => localStorage.setItem('agc.panel.human', n), name)
}

// Joins as the agent of task and posts once in channel.
async function postAsAgent(request: APIRequestContext, task: string, channel: string, body: string) {
  const name = unique('bot-')
  const joined = await request.post('/api/msg/join', { data: { name, task } })
  expect(joined.ok()).toBe(true)
  const { id } = (await joined.json()) as { id: string }
  const posted = await request.post('/api/msg/post', { data: { participant_id: id, channel, body } })
  expect(posted.ok()).toBe(true)
  return name
}

async function art(avatar: Locator) {
  const svg = avatar.locator('svg')
  return {
    seed: await svg.getAttribute('data-seed'),
    shapes: await svg.locator('[data-shapes]').innerHTML(),
  }
}

test('the same name draws the same picture everywhere, two names differ', async ({ page }) => {
  // Default identity franco: the top bar and the #review-42 header both
  // draw @franco.
  await page.goto('/panel/messages/review-42')
  const strip = page.getByRole('group', { name: 'Members' })
  const inStrip = strip.locator('[data-avatar="franco"]')
  const inTopBar = page.getByTestId('me').locator('[data-avatar="franco"]')
  await expect(inStrip).toBeVisible()
  await expect(inTopBar).toBeVisible()
  const a = await art(inStrip)
  const b = await art(inTopBar)
  expect(a.seed).toMatch(/^[0-9a-f]{8}$/)
  expect(b).toEqual(a)

  const reviewer = await art(strip.locator('[data-avatar="reviewer"]'))
  const tester = await art(strip.locator('[data-avatar="tester"]'))
  expect(reviewer.seed).not.toEqual(tester.seed)
  expect(reviewer.shapes).not.toEqual(tester.shapes)

  // Rings: human pink, agent violet; agc gets its grey mark.
  await expect(inStrip).toHaveAttribute('data-kind', 'human')
  await expect(strip.locator('[data-avatar="reviewer"]')).toHaveAttribute('data-kind', 'agent')
  await expect(strip.locator('[data-avatar="agc"]')).toHaveAttribute('data-kind', 'agc')
})

test('the channel header shows who is online and who was last seen', async ({ page, request }) => {
  const channel = unique('presence-')
  // review-42 has a running sandbox: its agent is online. flaky-ci has
  // none: its agent is offline.
  const on = await postAsAgent(request, 'review-42', `#${channel}`, 'here')
  const off = await postAsAgent(request, 'flaky-ci', `#${channel}`, 'was here')
  await actAs(page, unique('h'))
  await page.goto(`/panel/messages/${channel}`)

  const strip = page.getByRole('group', { name: 'Members' })
  const onMember = strip.locator(`[data-member="${on}"]`)
  const offMember = strip.locator(`[data-member="${off}"]`)
  await expect(onMember.getByTestId('presence')).toHaveAttribute('data-online', 'true')
  await expect(offMember.getByTestId('presence')).toHaveAttribute('data-online', 'false')
  await expect(onMember).toHaveAttribute('title', `@${on} · online (Agent in task review-42)`)
  await expect(offMember).toHaveAttribute('title', `@${off} · last seen just now (Agent in task flaky-ci)`)
})

test('the participants route carries online and last_seen_at', async ({ request }) => {
  const response = await request.get('/api/msg/participants?channel=%23review-42')
  const { participants } = (await response.json()) as {
    participants: { name: string; online: boolean; last_seen_at: string | null }[]
  }
  const byName = new Map(participants.map((p) => [p.name, p]))
  expect(byName.get('reviewer')?.online).toBe(true)
  expect(byName.get('franco')?.online).toBe(true)
  expect(typeof byName.get('tester')?.last_seen_at).toBe('string')
})

test('the Agents list shows an avatar per agent, the dot lit only when awake', async ({ page }) => {
  await page.goto('/panel/agents')
  const list = page.getByRole('list', { name: 'Agents' })
  const awake = list.locator('li[data-agent="orchestrator-1"]')
  const asleep = list.locator('li[data-agent="ada"]')
  await expect(awake.locator('[data-avatar="orchestrator-1"] svg')).toBeVisible()
  await expect(awake.getByTestId('presence')).toHaveAttribute('data-online', 'true')
  await expect(asleep.getByTestId('presence')).toHaveAttribute('data-online', 'false')
  // The word stays verbatim in its pill.
  await expect(awake.getByTestId('agent-state')).toHaveText('awake')
  const rows = await list.locator('li[data-agent]').count()
  await expect(list.locator('li[data-agent] [data-avatar]')).toHaveCount(rows)
})

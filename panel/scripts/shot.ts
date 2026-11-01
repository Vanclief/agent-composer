// Screenshot one panel route against a fresh mock, at 1440x900.
//
//   npm run shot -- /tasks
//   npm run shot -- /tasks/review-42/messages --theme light
//   npm run shot -- /messages --out /tmp/messages.png --wait 2000
//
// Starts its own mock and Vite dev server on free ports (no build, nothing
// shared with other checkouts), writes /tmp/agc-panel/<route>.png unless
// --out is given, and prints the path.
import { mkdirSync } from 'node:fs'
import type { AddressInfo } from 'node:net'
import { dirname } from 'node:path'

import { chromium } from '@playwright/test'
import { createServer } from 'vite'

import { startMock } from '../mock/server.ts'

function flag(name: string) {
  const i = process.argv.indexOf(name)
  if (i < 0) {
    return undefined
  }
  return process.argv[i + 1]
}

const flagsWithValues = new Set(['--out', '--theme', '--wait'])
const positional = process.argv
  .slice(2)
  .filter((arg, i, all) => !arg.startsWith('--') && !flagsWithValues.has(all[i - 1]))
const route = '/' + (positional[0] ?? '/tasks').replace(/^\/+/, '')
const theme = flag('--theme') ?? 'dark'
const settleMs = Number(flag('--wait') ?? 1000)
const slug = route.replace(/^\/+|\/+$/g, '').replace(/[^a-zA-Z0-9_-]+/g, '-') || 'root'
const out = flag('--out') ?? `/tmp/agc-panel/${slug}.png`

const mock = await startMock(0)
process.env.VITE_API_TARGET = mock.url
const vite = await createServer({
  logLevel: 'silent',
  server: { host: '127.0.0.1', port: 0 },
})
await vite.listen()
const port = (vite.httpServer?.address() as AddressInfo).port

const browser = await chromium.launch()
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  await context.addInitScript((value) => localStorage.setItem('agc.panel.theme', value), theme)
  const page = await context.newPage()
  page.on('pageerror', (error) => console.error('page error:', error.message))
  // Not networkidle: the tasks poll and the messaging long poll never idle.
  await page.goto(`http://127.0.0.1:${port}/panel${route}`, { waitUntil: 'load' })
  await page.waitForTimeout(settleMs)
  mkdirSync(dirname(out), { recursive: true })
  await page.screenshot({ path: out })
  console.log(out)
} finally {
  await browser.close()
  await vite.close()
  await mock.close()
}

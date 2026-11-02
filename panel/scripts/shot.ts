// Screenshot one panel route against a fresh mock, at 1440x900 unless
// --width / --height say otherwise.
//
//   npm run shot -- /tasks
//   npm run shot -- /tasks/review-42/messages --theme light
//   npm run shot -- /messages --out /tmp/messages.png --wait 2000
//   npm run shot -- /tasks/review-42 --width 1000
//   npm run shot -- /think/beliefs --storage agc.panel.group.work=closed
//   npm run shot -- /tasks --click '[data-testid=area-header-work]'
//
// Starts its own mock and Vite dev server on free ports (no build, nothing
// shared with other checkouts), writes /tmp/agc-panel/<route>.png unless
// --out is given, and prints the path. --storage key=value (repeatable)
// seeds localStorage before the page loads; --click <selector> clicks it
// once the page has settled.
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

const flagsWithValues = new Set(['--out', '--theme', '--wait', '--width', '--height', '--storage', '--click'])
const positional = process.argv
  .slice(2)
  .filter((arg, i, all) => !arg.startsWith('--') && !flagsWithValues.has(all[i - 1]))
const route = '/' + (positional[0] ?? '/tasks').replace(/^\/+/, '')
const theme = flag('--theme') ?? 'dark'
const settleMs = Number(flag('--wait') ?? 1000)
const width = Number(flag('--width') ?? 1440)
const height = Number(flag('--height') ?? 900)
const slug = route.replace(/^\/+|\/+$/g, '').replace(/[^a-zA-Z0-9_-]+/g, '-') || 'root'
const storage = process.argv
  .flatMap((arg, i, all) => (arg === '--storage' && all[i + 1] ? [all[i + 1]] : []))
  .map((pair) => [pair.slice(0, pair.indexOf('=')), pair.slice(pair.indexOf('=') + 1)])
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
  const context = await browser.newContext({ viewport: { width, height } })
  await context.addInitScript((value) => localStorage.setItem('agc.panel.theme', value), theme)
  await context.addInitScript((pairs) => pairs.forEach(([key, value]) => localStorage.setItem(key, value)), storage)
  const page = await context.newPage()
  page.on('pageerror', (error) => console.error('page error:', error.message))
  // Not networkidle: the tasks poll and the messaging long poll never idle.
  await page.goto(`http://127.0.0.1:${port}/panel${route}`, { waitUntil: 'load' })
  await page.waitForTimeout(settleMs)
  const click = flag('--click')
  if (click) {
    await page.click(click)
    await page.waitForTimeout(200)
  }
  mkdirSync(dirname(out), { recursive: true })
  await page.screenshot({ path: out })
  console.log(out)
} finally {
  await browser.close()
  await vite.close()
  await mock.close()
}

import { createHash } from 'node:crypto'

import { defineConfig, devices } from '@playwright/test'

// Ports come from this checkout's path, so test runs in parallel worktrees
// never share (or reuse) each other's servers. PANEL_TEST_PORT overrides.
function basePort() {
  if (process.env.PANEL_TEST_PORT) {
    return Number(process.env.PANEL_TEST_PORT)
  }
  const hash = parseInt(createHash('sha1').update(process.cwd()).digest('hex').slice(0, 6), 16)
  return 20000 + (hash % 20000) * 2
}

const mockPort = basePort()
const webPort = mockPort + 1

// Tests open full paths: page.goto('/panel/tasks').
export default defineConfig({
  testDir: 'tests',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${webPort}`,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: `tsx mock/server.ts --port ${mockPort}`,
      url: `http://127.0.0.1:${mockPort}/api/tasks`,
      reuseExistingServer: false,
    },
    {
      // The built panel, as agc would serve it, proxying /api to the mock.
      command: `vite build --logLevel warn && vite preview --host 127.0.0.1 --port ${webPort} --strictPort`,
      url: `http://127.0.0.1:${webPort}/panel/`,
      env: { VITE_API_TARGET: `http://127.0.0.1:${mockPort}` },
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
})

import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config for the dashboard e2e suite.
 *
 * `global-setup.ts` spawns the pre-built `secret-server` binary into a
 * fresh per-run tmp dir and captures the auto-bootstrap admin token from
 * stdout. The token lands in `process.env.DASHBOARD_TOKEN` so the auth
 * spec can read it. `global-teardown.ts` kills the server.
 *
 * Prerequisite: `make build` (or equivalent) must have produced
 * `bin/secret-server` at the worktree root.
 * One-time browser install: `pnpm exec playwright install chromium`.
 */
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:9090',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  globalSetup: './tests/e2e/global-setup.ts',
  globalTeardown: './tests/e2e/global-teardown.ts',
});

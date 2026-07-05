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
 * One-time browser install: `npx playwright install chromium`.
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
    // DESIGN.md seals visual decisions at 1920×1080; the dashboard targets
    // operator desktops, not the 1280×720 default. Pinning the viewport
    // here makes the seal config-driven instead of test-by-test.
    viewport: { width: 1920, height: 1080 },
  },
  projects: [
    {
      name: 'chromium',
      // The project spread overrides the top-level `use`, so we re-pin the
      // viewport here to keep the seal at 1920×1080 even if Playwright
      // changes the Desktop Chrome default in a future release.
      use: { ...devices['Desktop Chrome'], viewport: { width: 1920, height: 1080 } },
    },
  ],
  globalSetup: './tests/e2e/global-setup.ts',
  globalTeardown: './tests/e2e/global-teardown.ts',
});

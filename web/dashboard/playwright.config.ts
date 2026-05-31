import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config for the dashboard e2e suite.
 *
 * The default `testDir` is `tests/e2e/` so Vitest (which scans
 * src/**) and Playwright don't fight over the same files. Browsers
 * must be installed once before the first run:
 *
 *   pnpm exec playwright install --with-deps chromium
 *
 * Server orchestration:
 *   `webServer` is intentionally NOT set. The auth e2e needs a fresh
 *   secret-server with a known service token, which is easier to
 *   manage from the make target / CI workflow (Task 13/14) than from
 *   the Playwright config — the config would need to bootstrap the
 *   server, issue a token, and feed it into the test. We surface that
 *   via PLAYWRIGHT_BASE_URL / DASHBOARD_TOKEN env vars instead.
 */
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:8080',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
});

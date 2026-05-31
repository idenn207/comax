import { expect, test } from '@playwright/test';

/**
 * End-to-end auth flow against a live secret-server.
 *
 * Prerequisites:
 *   - secret-server running at PLAYWRIGHT_BASE_URL (default http://localhost:8080)
 *   - DASHBOARD_TOKEN env var set to a valid service token. Generate one via
 *     POST /api/v1/bootstrap on a fresh DB, or `secret token issue` on an
 *     existing one. See docs/quickstart.md.
 *
 * Skip-with-reason when the token isn't present so the suite stays green in
 * environments that don't run the integration server. CI Task 13/14 will
 * provide both.
 */

const token = process.env.DASHBOARD_TOKEN;

test.describe('dashboard auth', () => {
  test.skip(!token, 'DASHBOARD_TOKEN not set — see tests/e2e/auth.spec.ts header');

  test('login → refresh stays authed → logout → refresh redirects to /login', async ({ page }) => {
    await page.goto('/login');

    // ── Login ─────────────────────────────────────────────────────────
    await page.getByLabel('서비스 토큰').fill(token!);
    await page.getByRole('button', { name: '로그인' }).click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole('heading', { name: 'Comax Secrets' })).toBeVisible();

    // ── Refresh persists session ─────────────────────────────────────
    await page.reload();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole('heading', { name: 'Comax Secrets' })).toBeVisible();

    // ── Logout flushes cookie + CSRF ─────────────────────────────────
    await page.getByRole('button', { name: '로그아웃' }).click();
    await expect(page).toHaveURL(/\/login$/);

    // ── Refresh after logout still hits /login (no stale auth) ───────
    await page.reload();
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole('heading', { name: '로그인' })).toBeVisible();
  });

  test('invalid token shows an inline error and stays on /login', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('서비스 토큰').fill('definitely-not-a-real-token');
    await page.getByRole('button', { name: '로그인' }).click();
    await expect(page.getByRole('alert')).toContainText('토큰이 올바르지 않습니다');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('protected route bounces unauthenticated users to /login', async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/');
    await expect(page).toHaveURL(/\/login$/);
  });
});

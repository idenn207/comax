import { expect, test } from '@playwright/test';

/**
 * End-to-end auth flow against a live secret-server.
 *
 * `playwright.config.ts` launches the binary (webServer) into a fresh
 * tmp DB and `tests/e2e/global-setup.ts` calls POST /bootstrap to obtain
 * the admin token, which is injected here via DASHBOARD_TOKEN.
 */

test.describe('dashboard auth', () => {
  test('login → refresh stays authed → logout → refresh redirects to /login', async ({ page }) => {
    const token = process.env.DASHBOARD_TOKEN;
    if (!token) throw new Error('DASHBOARD_TOKEN missing — global-setup did not run');

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
    await expect(page.getByRole('alert')).toContainText('Invalid token');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('protected route bounces unauthenticated users to /login', async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/');
    await expect(page).toHaveURL(/\/login$/);
  });
});

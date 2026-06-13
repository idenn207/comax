import { expect, test } from '@playwright/test';

import { checkA11y, loginWithBootstrap } from './helpers/axe';

/**
 * /settings/sessions: list + revoke + axe gate.
 *
 * Setup follows a11y.spec.ts. A second browser context POSTs
 * /api/v1/dashboard/session against the same bearer so the actor's
 * list shows two rows — the cookie-current one and the second-context
 * one. Revoking the second row through the UI exercises the full code
 * path: ConfirmDialog → DELETE → invalidate → re-render.
 */

test.describe('dashboard sessions', () => {
  test('list shows current + second session, revoke + axe', async ({ browser, page }) => {
    await loginWithBootstrap(page);

    const token = process.env.DASHBOARD_TOKEN;
    if (!token) throw new Error('DASHBOARD_TOKEN missing — global-setup did not run');

    const secondaryCtx = await browser.newContext();
    const secondaryResp = await secondaryCtx.request.post('/api/v1/dashboard/session', {
      data: { token },
    });
    expect(secondaryResp.status()).toBe(201);
    await secondaryCtx.close();

    await page.goto('/settings/sessions');
    await expect(page.getByRole('heading', { name: '활성 세션' })).toBeVisible();

    const rows = page.locator('.session-row');
    await expect(rows).toHaveCount(2);
    await expect(page.getByText('현재 세션')).toHaveCount(1);

    const currentRow = page.locator('.session-row--current');
    await expect(
      currentRow.getByRole('button', { name: /현재 세션은 회수할 수 없습니다/ }),
    ).toBeDisabled();

    const otherRow = page.locator('.session-row:not(.session-row--current)');
    await otherRow.getByRole('button', { name: /회수/ }).click();

    await expect(page.getByText(/cookie가 이미 탈취된 상태였다면/)).toBeVisible();

    await page.getByRole('button', { name: '회수', exact: true }).click();

    await expect(rows).toHaveCount(1);
    await expect(page.locator('.session-row--current')).toHaveCount(1);

    await checkA11y(page);
  });
});

import { expect, test } from '@playwright/test';

import { checkA11y, loginWithBootstrap } from './helpers/axe';

/**
 * Per-route axe-core WCAG 2.2 AA gate.
 *
 * Each test navigates to one route, waits for its primary heading or
 * landmark to confirm hydration, then runs axe across the rendered DOM.
 *
 * The bootstrap token from global-setup.ts logs us into the dashboard
 * before exercising the authenticated routes; /login is checked
 * separately while signed out.
 */

test.describe('dashboard a11y (WCAG 2.2 AA)', () => {
  test('/login passes axe while signed out', async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/login');
    await expect(page.getByRole('heading', { name: '로그인' })).toBeVisible();
    await checkA11y(page);
  });

  test('/ (projects grid home) passes axe', async ({ page }) => {
    await loginWithBootstrap(page);
    await page.goto('/');
    await expect(page.getByRole('heading', { name: '프로젝트', exact: true })).toBeVisible();
    await checkA11y(page);
  });

  test('/audit (empty feed) passes axe', async ({ page }) => {
    await loginWithBootstrap(page);
    await page.goto('/audit');
    await expect(page.getByRole('heading', { name: '감사 로그' })).toBeVisible();
    await checkA11y(page);
  });

  // The bootstrap token is admin, so /settings/tokens renders the full
  // table (M3). A non-admin session would show the "admin only" notice
  // instead; both paths must pass axe.
  test('/settings/tokens (M3) passes axe', async ({ page }) => {
    await loginWithBootstrap(page);
    await page.goto('/settings/tokens');
    await expect(page.getByRole('heading', { name: '서비스 토큰' })).toBeVisible();
    await checkA11y(page);
  });

  test('/integrations/github-actions (M3) passes axe', async ({ page }) => {
    await loginWithBootstrap(page);
    await page.goto('/integrations/github-actions');
    await expect(page.getByRole('heading', { name: 'GitHub Actions', exact: true })).toBeVisible();
    await checkA11y(page);
  });

  // The bootstrap token is admin, so /integrations/webhooks renders the full
  // table (M4). A non-admin session shows the "admin only" notice instead;
  // both paths must pass axe.
  test('/integrations/webhooks (M4) passes axe', async ({ page }) => {
    await loginWithBootstrap(page);
    await page.goto('/integrations/webhooks');
    await expect(page.getByRole('heading', { name: '웹훅', exact: true })).toBeVisible();
    await checkA11y(page);
  });

  // The env-vs-env diff page renders a "select an env" empty state when
  // there is nothing to compare against — that's the path we check from
  // the bootstrap, since the audit/empty-state path is the most a11y-
  // fragile (callouts, headings, select).
  test('project + env screens pass axe through the create flow', async ({ page }) => {
    await loginWithBootstrap(page);
    const projectName = `a11y-${Date.now()}`;
    const envName = 'prod';

    // Create a project so the grid home has both empty-state and
    // populated-state coverage in the same run.
    await page.goto('/');
    await page.getByRole('button', { name: '새 프로젝트' }).click();
    await page.getByLabel('프로젝트 이름').fill(projectName);
    await page.getByRole('button', { name: '생성' }).click();
    await expect(page.getByRole('link', { name: `프로젝트 ${projectName} 열기` })).toBeVisible();
    await checkA11y(page);

    await page.getByRole('link', { name: `프로젝트 ${projectName} 열기` }).click();
    await expect(page.getByRole('heading', { name: projectName })).toBeVisible();
    await checkA11y(page);

    await page.getByRole('button', { name: '새 환경' }).click();
    await page.getByLabel('환경 이름').fill(envName);
    await page.getByRole('button', { name: '생성' }).click();
    await expect(page.getByRole('link', { name: `환경 ${envName} 열기` })).toBeVisible();
    await checkA11y(page);

    await page.getByRole('link', { name: `환경 ${envName} 열기` }).click();
    await expect(page.getByRole('heading', { name: envName })).toBeVisible();
    await checkA11y(page);
  });
});

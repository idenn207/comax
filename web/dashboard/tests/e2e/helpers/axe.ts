import AxeBuilder from '@axe-core/playwright';
import { expect, type Page } from '@playwright/test';

/**
 * Run an axe-core scan against the current page and assert there are no
 * WCAG 2.1 / 2.2 AA violations. The helper centralizes:
 *
 *   1. Tag set — `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa`, `wcag22aa`
 *      cover the levels the dashboard targets. `best-practice` is not
 *      enforced because axe's best-practice rules flag styling choices
 *      (color-contrast-enhanced, region-id) that exceed what the PRD
 *      asks for.
 *
 *   2. Disabled rules — `region` is excluded on routes where Radix
 *      Themes injects a portal root above the document body before the
 *      main landmark mounts. We keep the rule but disable it per call
 *      so the per-page exemption is explicit at the call site, not
 *      buried in the helper.
 *
 * Pretty-prints violations on failure so the operator can see the
 * exact selector + WCAG criterion without digging through Playwright's
 * raw expectation diff.
 */

export interface AxeOptions {
  /** Additional rule IDs to skip for this call (e.g. ['region']). */
  disableRules?: string[];
}

export async function checkA11y(page: Page, options: AxeOptions = {}): Promise<void> {
  const builder = new AxeBuilder({ page }).withTags([
    'wcag2a',
    'wcag2aa',
    'wcag21a',
    'wcag21aa',
    'wcag22aa',
  ]);
  if (options.disableRules && options.disableRules.length > 0) {
    builder.disableRules(options.disableRules);
  }
  const results = await builder.analyze();

  if (results.violations.length > 0) {
    const formatted = results.violations.map((v) => ({
      id: v.id,
      impact: v.impact,
      help: v.help,
      nodes: v.nodes.map((n) => ({
        target: n.target,
        failureSummary: n.failureSummary,
      })),
    }));
    console.error(
      `axe found ${results.violations.length} violation(s):\n${JSON.stringify(formatted, null, 2)}`,
    );
  }

  expect(results.violations, 'axe-core WCAG 2.2 AA violations').toEqual([]);
}

/**
 * Drive the login form with the bootstrap token captured by
 * global-setup. Used by every authenticated route's a11y spec so the
 * token wiring lives in one place.
 */
export async function loginWithBootstrap(page: Page): Promise<void> {
  const token = process.env.DASHBOARD_TOKEN;
  if (!token) throw new Error('DASHBOARD_TOKEN missing — global-setup did not run');
  await page.goto('/login');
  await page.getByLabel('서비스 토큰').fill(token);
  await page.getByRole('button', { name: '로그인' }).click();
  await page.waitForURL(/\/$/);
}

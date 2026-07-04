/**
 * Site-wide constants. `url` is parameterized by SITE_URL so sitemap/robots/
 * canonical/OG render against the real deploy host (Codex F1). The placeholder
 * fallback is intentionally obvious so check-site-url.mjs can fail-closed if a
 * production build ships without SITE_URL set (Codex impl-F4).
 */

const FALLBACK_URL = 'http://localhost:3000';

/**
 * Resolve the canonical site origin. Precedence:
 *   1. SITE_URL (explicit operator override).
 *   2. VERCEL_PROJECT_PRODUCTION_URL (Vercel's stable production domain) — so a
 *      Vercel production build gets the real host with no manual SITE_URL.
 *   3. VERCEL_URL (per-deployment host) — covers preview deploys.
 *   4. localhost fallback (local dev; check-site-url.mjs fails closed on this in
 *      strict/production contexts so it never ships as canonical).
 */
export function resolveSiteUrl(): string {
  const explicit = process.env.SITE_URL;
  if (explicit) return explicit.replace(/\/$/, '');
  const vercelProd = process.env.VERCEL_PROJECT_PRODUCTION_URL;
  if (vercelProd) return `https://${vercelProd}`.replace(/\/$/, '');
  const vercelUrl = process.env.VERCEL_URL;
  if (vercelUrl) return `https://${vercelUrl}`.replace(/\/$/, '');
  return FALLBACK_URL;
}

export const siteUrl: string = resolveSiteUrl();

export const siteConfig = {
  name: 'Comax Secrets',
  tagline: '가벼운 self-host 시크릿 매니저',
  description:
    '개인 개발자와 소규모 팀을 위한 가벼운 self-host 시크릿 매니저. SQLite 하나로 부팅하고, worktree와 multi-service 환경을 1급으로 다루며, CLI 한 줄과 GitHub Action 한 줄로 시크릿을 주입한다.',
  url: siteUrl,
  repo: 'https://github.com/idenn207/comax',
  license: 'MIT',
} as const;

export type NavItem = {
  title: string;
  href: string;
  external?: boolean;
};

export const mainNav: NavItem[] = [
  { title: '문서', href: '/docs' },
  { title: 'Quickstart', href: '/docs/quickstart' },
  { title: 'CLI', href: '/docs/cli' },
  { title: 'SDK', href: '/docs/sdk' },
  { title: 'GitHub', href: siteConfig.repo, external: true },
];

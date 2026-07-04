#!/usr/bin/env node
// Codex impl-F4 — SITE_URL fail-closed guard.
//
// metadataBase, canonical URLs, sitemap, robots, and OG all resolve against the
// site origin at build time. If a production build bakes the localhost fallback,
// every canonical/sitemap/OG URL is wrong. This gate fails closed in strict mode
// so it cannot ship.
//
// It resolves the origin with the SAME precedence as lib/site.ts (SITE_URL →
// VERCEL_PROJECT_PRODUCTION_URL → VERCEL_URL → localhost) and is wired into the
// Vercel build command (vercel.json), so the real production deploy path runs it
// too — not just CI. Strict mode fires on a production deploy (VERCEL_ENV=
// production) or an explicit SITE_URL_REQUIRED=1.

function resolveSiteUrl() {
  const explicit = process.env.SITE_URL;
  if (explicit) return explicit.replace(/\/$/, '');
  if (process.env.VERCEL_PROJECT_PRODUCTION_URL)
    return `https://${process.env.VERCEL_PROJECT_PRODUCTION_URL}`.replace(/\/$/, '');
  if (process.env.VERCEL_URL) return `https://${process.env.VERCEL_URL}`.replace(/\/$/, '');
  return '';
}

const url = resolveSiteUrl().trim();
// Strict for ANY Vercel build (VERCEL=1 is injected in preview AND production),
// since vercel.json's buildCommand runs this and any deploy bakes canonical/OG
// URLs that must be a real host. Also strict on explicit SITE_URL_REQUIRED=1.
// Local dev / CI (VERCEL unset) stay non-strict so the localhost fallback warns
// instead of failing.
const strict = process.env.SITE_URL_REQUIRED === '1' || process.env.VERCEL === '1';

const placeholder = /localhost|127\.0\.0\.1|example\.(com|test|org)|placeholder|your-|\$\{/i;
const isBad = !url || !/^https?:\/\//.test(url) || placeholder.test(url);

if (isBad) {
  const detail = `resolved site origin is not a real production host: '${url || '(unset)'}'`;
  if (strict) {
    const why = process.env.VERCEL === '1' ? `Vercel build (VERCEL_ENV=${process.env.VERCEL_ENV ?? 'unknown'})` : 'SITE_URL_REQUIRED=1';
    console.error(
      `check-site-url FAIL — ${detail} (strict: ${why}). Set SITE_URL, or deploy where Vercel injects VERCEL_PROJECT_PRODUCTION_URL.`,
    );
    process.exit(1);
  }
  console.warn(
    `check-site-url WARN — ${detail}; local/CI fallback in use. Any Vercel build (VERCEL=1) fails closed.`,
  );
  process.exit(0);
}

console.log(`check-site-url OK — SITE_URL='${url}'.`);

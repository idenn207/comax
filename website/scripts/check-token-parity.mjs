#!/usr/bin/env node
// Codex F4 — cross-app token drift guard.
//
// The website mirrors web/dashboard neutral + semantic tokens by copy (no shared
// package). This asserts that every --color-* token present in BOTH files has an
// identical value in the light and dark blocks. The website-only --color-brand*
// family (M6 D10) is the single allowed exception.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const websiteRoot = path.resolve(fileURLToPath(import.meta.url), '../..');
const repoRoot = path.resolve(websiteRoot, '..');

const dashboardTokens = path.join(repoRoot, 'web', 'dashboard', 'src', 'styles', 'tokens.css');
const websiteGlobals = path.join(websiteRoot, 'app', 'globals.css');

const ALLOW_WEBSITE_ONLY = /^color-brand/;

function parseColorTokens(css) {
  const darkIdx = css.indexOf("[data-theme='dark']");
  const lightPart = darkIdx >= 0 ? css.slice(0, darkIdx) : css;
  const darkPart = darkIdx >= 0 ? css.slice(darkIdx) : '';
  const extract = (s) => {
    const map = {};
    const re = /--(color-[\w-]+):\s*([^;]+);/g;
    let m;
    while ((m = re.exec(s))) map[m[1]] = m[2].trim();
    return map;
  };
  return { light: extract(lightPart), dark: extract(darkPart) };
}

function normalize(value) {
  // Collapse whitespace, then canonicalize numbers so formatting-only diffs
  // (0.010 vs 0.01, 0.20 vs 0.2) do not read as drift.
  return value
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/[0-9]*\.?[0-9]+/g, (n) => String(Number(n)));
}

const dashboard = parseColorTokens(readFileSync(dashboardTokens, 'utf8'));
const website = parseColorTokens(readFileSync(websiteGlobals, 'utf8'));

const mismatches = [];
for (const mode of ['light', 'dark']) {
  const d = dashboard[mode];
  const w = website[mode];
  for (const name of Object.keys(w)) {
    if (ALLOW_WEBSITE_ONLY.test(name)) continue;
    if (!(name in d)) continue; // website may omit some dashboard tokens; only check the intersection
    if (normalize(d[name]) !== normalize(w[name])) {
      mismatches.push(`[${mode}] --${name}: dashboard='${d[name]}' website='${w[name]}'`);
    }
  }
}

if (mismatches.length > 0) {
  console.error('token-parity FAIL — neutral/semantic tokens diverged from the dashboard:');
  for (const m of mismatches) console.error('  ' + m);
  console.error('\nFix: sync the value, or (if intentionally website-only) name it --color-brand*.');
  process.exit(1);
}

console.log('token-parity OK — shared --color-* tokens match the dashboard (brand exempt).');

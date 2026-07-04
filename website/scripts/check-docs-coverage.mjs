#!/usr/bin/env node
// Codex impl-F3 — hand-built docs shell coverage guard.
//
// Because nav/search/prev-next are hand-built (not a docs framework), a doc can
// exist on disk yet be unreachable from the nav (and thus the search index /
// prev-next / sitemap). This asserts a bijection between content/docs/*.mdx and
// the docsNav slugs in lib/docs.ts, and that every doc has frontmatter the
// search index needs (title + description).

import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const websiteRoot = path.resolve(fileURLToPath(import.meta.url), '../..');
const docsDir = path.join(websiteRoot, 'content', 'docs');
const libDocs = path.join(websiteRoot, 'lib', 'docs-nav.ts');

function navSlugs() {
  const src = readFileSync(libDocs, 'utf8');
  const start = src.indexOf('export const docsNav');
  const end = src.indexOf('\n];', start);
  if (start === -1 || end === -1) {
    console.error('coverage FAIL — could not locate docsNav block in lib/docs.ts');
    process.exit(1);
  }
  const block = src.slice(start, end);
  const slugs = [];
  const re = /slug:\s*'([^']*)'/g;
  let m;
  while ((m = re.exec(block))) slugs.push(m[1]);
  return slugs;
}

function fileSlugs() {
  return readdirSync(docsDir)
    .filter((f) => f.endsWith('.mdx'))
    .map((f) => (f === 'index.mdx' ? '' : f.replace(/\.mdx$/, '')));
}

const errors = [];
const navList = navSlugs();
const nav = new Set(navList);
const files = new Set(fileSlugs());

// A duplicate slug in docsNav collapses in the Set-based bijection below but
// makes prev/next (docSiblings) non-deterministic — catch it explicitly.
if (navList.length !== nav.size) {
  const seen = new Set();
  for (const slug of navList) {
    if (seen.has(slug)) errors.push(`docsNav has duplicate slug '${slug || '(index)'}' (breaks prev/next)`);
    seen.add(slug);
  }
}

for (const slug of files) {
  if (!nav.has(slug)) errors.push(`file '${slug || 'index'}.mdx' has no docsNav entry (unreachable)`);
}
for (const slug of nav) {
  if (!files.has(slug)) errors.push(`docsNav slug '${slug || '(index)'}' has no content/docs file`);
}

// Frontmatter completeness (search index needs title + description).
for (const slug of files) {
  const file = path.join(docsDir, slug === '' ? 'index.mdx' : `${slug}.mdx`);
  const src = readFileSync(file, 'utf8');
  const fm = /^---\r?\n([\s\S]*?)\r?\n---/.exec(src);
  if (!fm) {
    errors.push(`'${slug || 'index'}.mdx' missing frontmatter block`);
    continue;
  }
  if (!/\btitle:\s*\S/.test(fm[1])) errors.push(`'${slug || 'index'}.mdx' missing frontmatter title`);
  if (!/\bdescription:\s*\S/.test(fm[1]))
    errors.push(`'${slug || 'index'}.mdx' missing frontmatter description`);
}

if (errors.length > 0) {
  console.error('docs-coverage FAIL:');
  for (const e of errors) console.error('  ' + e);
  process.exit(1);
}

console.log(`docs-coverage OK — ${files.size} docs, all reachable from docsNav with frontmatter.`);

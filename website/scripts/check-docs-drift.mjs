#!/usr/bin/env node
// Codex impl-F3 — source-contract drift guard.
//
// After the repo's user-facing docs are reduced to stubs, the website MDX is the
// canonical reference. This asserts the MDX still matches the ACTUAL source
// contracts (not the stubs): CLI subcommands (cmd/cli/main.go), GitHub Action
// inputs (action.yml), and SDK exports (sdk/src/index.ts). If a command/flag/
// input/export is added or renamed at the source, the drift check fails until
// the MDX is updated. It also verifies internal /docs links resolve.

import { existsSync, readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const websiteRoot = path.resolve(fileURLToPath(import.meta.url), '../..');
const repoRoot = path.resolve(websiteRoot, '..');
const docsDir = path.join(websiteRoot, 'content', 'docs');

const errors = [];

function read(file) {
  return readFileSync(file, 'utf8');
}
function readDoc(name) {
  const p = path.join(docsDir, name);
  if (!existsSync(p)) {
    errors.push(`missing doc: content/docs/${name}`);
    return '';
  }
  return read(p);
}

// ── CLI subcommands (cmd/cli/main.go) → cli.mdx ──
const mainGo = read(path.join(repoRoot, 'cmd', 'cli', 'main.go'));
const cliCommands = [...mainGo.matchAll(/new(\w+)Cmd\(st\)/g)].map((m) => m[1].toLowerCase());
const cliMdx = readDoc('cli.mdx');
for (const cmd of cliCommands) {
  if (!cliMdx.includes(`secret ${cmd}`)) {
    errors.push(`cli.mdx missing CLI command: 'secret ${cmd}' (source: cmd/cli/main.go)`);
  }
}

// ── GitHub Action inputs (action.yml) → github-actions.mdx ──
const actionYml = read(path.join(repoRoot, 'action.yml'));
const inputsStart = actionYml.indexOf('\ninputs:');
const runsStart = actionYml.indexOf('\nruns:');
const inputsBlock =
  inputsStart >= 0 ? actionYml.slice(inputsStart, runsStart >= 0 ? runsStart : undefined) : '';
const actionInputs = [...inputsBlock.matchAll(/^ {2}([a-z][\w-]*):/gm)].map((m) => m[1]);
const actionMdx = readDoc('github-actions.mdx');
for (const input of actionInputs) {
  if (!actionMdx.includes(input)) {
    errors.push(`github-actions.mdx missing action input: '${input}' (source: action.yml)`);
  }
}

// ── SDK exports (sdk/src/index.ts) → sdk.mdx ──
const indexTs = read(path.join(repoRoot, 'sdk', 'src', 'index.ts'));
const OPTIONAL_EXPORTS = new Set(['Http']);
const valueExports = new Set();
for (const m of indexTs.matchAll(/export function (\w+)/g)) valueExports.add(m[1]);
for (const block of indexTs.matchAll(/export \{([^}]*)\}/g)) {
  for (const raw of block[1].split(',')) {
    const entry = raw.trim();
    if (!entry || entry.startsWith('type ')) continue;
    const name = entry.split(/\s+as\s+/)[0].trim();
    if (/^[A-Za-z_]\w*$/.test(name)) valueExports.add(name);
  }
}
const sdkMdx = readDoc('sdk.mdx');
for (const name of valueExports) {
  if (OPTIONAL_EXPORTS.has(name)) continue;
  if (!sdkMdx.includes(name)) {
    errors.push(`sdk.mdx missing SDK export: '${name}' (source: sdk/src/index.ts)`);
  }
}
for (const method of ['getAll', 'reload']) {
  if (sdkMdx && !sdkMdx.includes(method)) {
    errors.push(`sdk.mdx missing SecretsClient method: '${method}'`);
  }
}

// ── Internal /docs link resolution (all mdx) ──
const validSlugs = new Set(
  readdirSync(docsDir)
    .filter((f) => f.endsWith('.mdx'))
    .map((f) => (f === 'index.mdx' ? '' : f.replace(/\.mdx$/, ''))),
);
for (const file of readdirSync(docsDir).filter((f) => f.endsWith('.mdx'))) {
  const src = read(path.join(docsDir, file));
  for (const m of src.matchAll(/\/docs\/([a-z0-9-]*)/g)) {
    const slug = m[1];
    if (!validSlugs.has(slug)) {
      errors.push(`${file}: internal link '/docs/${slug}' points to a non-existent doc`);
    }
  }
}

if (errors.length > 0) {
  console.error('docs-drift FAIL:');
  for (const e of errors) console.error('  ' + e);
  process.exit(1);
}

console.log(
  `docs-drift OK — ${cliCommands.length} CLI commands, ${actionInputs.length} action inputs, ${valueExports.size} SDK exports all present; internal links resolve.`,
);

// Pure docs navigation data + types. NO node:fs / node:path imports so this
// module is safe to import from client components (the sidebar, toc, pager).
// Filesystem-backed helpers live in lib/docs.ts (server-only).

export type DocNavItem = {
  /** URL segment after /docs. Empty string = the docs index (/docs). */
  slug: string;
  title: string;
};

export type DocNavGroup = {
  group: string;
  items: DocNavItem[];
};

/**
 * The docs table of contents — single source of truth for ordering, grouping,
 * prev/next, and coverage. check-docs-coverage.mjs asserts every .mdx under
 * content/docs is reachable from here (Codex impl-F3).
 */
export const docsNav: DocNavGroup[] = [
  {
    group: '시작하기',
    items: [
      { slug: '', title: '개요' },
      { slug: 'quickstart', title: 'Quickstart' },
      { slug: 'self-host', title: 'Self-host 배포' },
    ],
  },
  {
    group: '레퍼런스',
    items: [
      { slug: 'cli', title: 'CLI (secret)' },
      { slug: 'sdk', title: 'Node/TS SDK' },
      { slug: 'github-actions', title: 'GitHub Actions' },
      { slug: 'webhooks', title: 'Webhooks' },
    ],
  },
  {
    group: '운영',
    items: [{ slug: 'security', title: '보안 모델' }],
  },
];

export const flatDocs: DocNavItem[] = docsNav.flatMap((g) => g.items);

export function docHref(slug: string): string {
  return slug === '' ? '/docs' : `/docs/${slug}`;
}

export function docSiblings(slug: string): { prev: DocNavItem | null; next: DocNavItem | null } {
  const i = flatDocs.findIndex((d) => d.slug === slug);
  if (i === -1) return { prev: null, next: null };
  return {
    prev: i > 0 ? (flatDocs[i - 1] ?? null) : null,
    next: i < flatDocs.length - 1 ? (flatDocs[i + 1] ?? null) : null,
  };
}

export type TocItem = { depth: 2 | 3; text: string; id: string };

import fs from 'node:fs';
import path from 'node:path';
import GithubSlugger from 'github-slugger';
import { flatDocs, docHref, type TocItem } from '@/lib/docs-nav';

// Server-only: this module reads the filesystem. Client components must import
// nav data from lib/docs-nav instead (no fs).
export * from '@/lib/docs-nav';

export const DOCS_DIR = path.join(process.cwd(), 'content', 'docs');

export function docFilePath(slug: string): string {
  const base = slug === '' ? 'index' : slug;
  return path.join(DOCS_DIR, `${base}.mdx`);
}

export function docExists(slug: string): boolean {
  return fs.existsSync(docFilePath(slug));
}

export function readDocSource(slug: string): string {
  return fs.readFileSync(docFilePath(slug), 'utf8');
}

/** All slug arrays for generateStaticParams (index = []). */
export function allDocParams(): { slug: string[] }[] {
  return flatDocs.map((d) => ({ slug: d.slug === '' ? [] : d.slug.split('/') }));
}

/**
 * Extract ## / ### headings for the on-page TOC. Slugs are produced with
 * github-slugger so they match the ids rehype-slug adds during MDX render.
 * Code fences and the frontmatter block are skipped.
 */
export function extractToc(source: string): TocItem[] {
  const body = source.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n/, '');
  const slugger = new GithubSlugger();
  const toc: TocItem[] = [];
  let inFence = false;
  for (const line of body.split(/\r?\n/)) {
    if (/^\s*```/.test(line)) {
      inFence = !inFence;
      continue;
    }
    if (inFence) continue;
    const m = /^(#{2,3})\s+(.+?)\s*$/.exec(line);
    if (m && m[1] && m[2]) {
      const depth = m[1].length as 2 | 3;
      const text = m[2].replace(/[`*_]/g, '').trim();
      toc.push({ depth, text, id: slugger.slug(text) });
    }
  }
  return toc;
}

export type DocFrontmatter = { title: string; description: string };

/** Minimal frontmatter reader for the search index (no MDX compile). */
export function readDocFrontmatter(slug: string): DocFrontmatter {
  const src = readDocSource(slug);
  const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(src);
  const fm: Record<string, string> = {};
  if (m && m[1]) {
    for (const line of m[1].split(/\r?\n/)) {
      const kv = /^(\w+):\s*(.*)$/.exec(line);
      if (kv && kv[1] && kv[2] !== undefined) {
        fm[kv[1]] = kv[2].replace(/^["']|["']$/g, '').trim();
      }
    }
  }
  return { title: fm.title ?? slug, description: fm.description ?? '' };
}

export type SearchDoc = {
  slug: string;
  href: string;
  title: string;
  description: string;
  headings: { text: string; id: string }[];
};

/** Build the client-side search index. Covers every doc in flatDocs. */
export function buildSearchIndex(): SearchDoc[] {
  return flatDocs.map((d) => {
    const fm = readDocFrontmatter(d.slug);
    const headings = extractToc(readDocSource(d.slug)).map((h) => ({ text: h.text, id: h.id }));
    return {
      slug: d.slug,
      href: docHref(d.slug),
      title: fm.title || d.title,
      description: fm.description,
      headings,
    };
  });
}

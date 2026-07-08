import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import {
  allDocParams,
  docExists,
  docHref,
  docSiblings,
  extractToc,
  readDocSource,
} from '@/lib/docs';
import { renderDoc } from '@/lib/mdx';
import { Toc } from '@/components/docs/toc';
import { DocPager } from '@/components/docs/pager';
import { pageMetadata } from '@/lib/metadata';

type Params = { slug?: string[] };

// Every doc is pre-rendered; unknown paths 404 instead of dynamic-rendering
// (Codex impl-F1).
export const dynamicParams = false;

export function generateStaticParams() {
  return allDocParams();
}

function toSlug(params: Params): string {
  return (params.slug ?? []).join('/');
}

export async function generateMetadata({
  params,
}: {
  params: Promise<Params>;
}): Promise<Metadata> {
  const slug = toSlug(await params);
  if (!docExists(slug)) return {};
  const { frontmatter } = await renderDoc(readDocSource(slug));
  return pageMetadata({
    title: frontmatter.title,
    description: frontmatter.description,
    path: docHref(slug),
  });
}

export default async function DocPage({ params }: { params: Promise<Params> }) {
  const slug = toSlug(await params);
  if (!docExists(slug)) notFound();

  const source = readDocSource(slug);
  const { content, frontmatter } = await renderDoc(source);
  const toc = extractToc(source);
  const { prev, next } = docSiblings(slug);

  return (
    <div className="xl:grid xl:grid-cols-[minmax(0,1fr)_12rem] xl:gap-10">
      <article className="min-w-0">
        <header className="mb-7">
          {frontmatter.eyebrow && <span className="doc-eyebrow">{frontmatter.eyebrow}</span>}
          <h1 className="text-3xl font-semibold tracking-tight text-text">{frontmatter.title}</h1>
          {frontmatter.description && (
            <p className="mt-3 max-w-[46rem] text-md leading-relaxed text-text-subtle">
              {frontmatter.description}
            </p>
          )}
        </header>
        <div className="prose">{content}</div>
        <DocPager prev={prev} next={next} />
      </article>
      <aside className="hidden xl:block">
        <div className="sticky top-[var(--header-height)] py-8">
          <Toc items={toc} />
        </div>
      </aside>
    </div>
  );
}

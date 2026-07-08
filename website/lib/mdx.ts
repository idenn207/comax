import { compileMDX } from 'next-mdx-remote/rsc';
import rehypeSlug from 'rehype-slug';
import rehypeAutolinkHeadings from 'rehype-autolink-headings';
import rehypePrettyCode, { type Options as PrettyCodeOptions } from 'rehype-pretty-code';
import remarkGfm from 'remark-gfm';
import remarkHeadingId from '@/lib/remark-heading-id';
import { mdxComponents } from '@/mdx-components';
import type { DocFrontmatter } from '@/lib/docs';

// Dual theme: rehype-pretty-code emits --shiki-light / --shiki-dark CSS
// variables per token; globals.css picks one based on [data-theme].
// keepBackground:false so the code surface uses our --color-code-bg token.
const prettyCodeOptions: PrettyCodeOptions = {
  theme: { light: 'github-light', dark: 'github-dark' },
  keepBackground: false,
};

// An explicit heading id is authored as `## 제목 {#slug}`. MDX would read the
// bare `{…}` as a JS expression and fail, so escape the braces (`\{ … \}`) on
// heading lines before compile; remarkHeadingId then lifts the slug into the
// heading id. Scoped to a trailing marker on an ATX heading line, so code
// fences and inline `{expr}` elsewhere are untouched.
function escapeHeadingIdMarkers(source: string): string {
  return source.replace(
    /^(#{1,6}[^\n]*?)\{#([a-z0-9]+(?:-[a-z0-9]+)*)\}([ \t]*)$/gm,
    '$1\\{#$2\\}$3',
  );
}

/** Compile an MDX document source (frontmatter parsed) into RSC content. */
export async function renderDoc(source: string) {
  return compileMDX<DocFrontmatter>({
    source: escapeHeadingIdMarkers(source),
    components: mdxComponents,
    options: {
      parseFrontmatter: true,
      mdxOptions: {
        remarkPlugins: [remarkGfm, remarkHeadingId],
        rehypePlugins: [
          rehypeSlug,
          [rehypePrettyCode, prettyCodeOptions],
          [
            rehypeAutolinkHeadings,
            {
              // Modern docs pattern: headings stay plain text; a small "#"
              // permalink is appended and revealed on hover (styled in
              // globals.css), instead of wrapping the whole heading in a link.
              behavior: 'append',
              properties: { className: ['heading-anchor'], ariaHidden: 'true', tabIndex: -1 },
              content: { type: 'text', value: '#' },
            },
          ],
        ],
      },
    },
  });
}

import { compileMDX } from 'next-mdx-remote/rsc';
import rehypeSlug from 'rehype-slug';
import rehypeAutolinkHeadings from 'rehype-autolink-headings';
import rehypePrettyCode, { type Options as PrettyCodeOptions } from 'rehype-pretty-code';
import remarkGfm from 'remark-gfm';
import { mdxComponents } from '@/mdx-components';
import type { DocFrontmatter } from '@/lib/docs';

// Dual theme: rehype-pretty-code emits --shiki-light / --shiki-dark CSS
// variables per token; globals.css picks one based on [data-theme].
// keepBackground:false so the code surface uses our --color-code-bg token.
const prettyCodeOptions: PrettyCodeOptions = {
  theme: { light: 'github-light', dark: 'github-dark' },
  keepBackground: false,
};

/** Compile an MDX document source (frontmatter parsed) into RSC content. */
export async function renderDoc(source: string) {
  return compileMDX<DocFrontmatter>({
    source,
    components: mdxComponents,
    options: {
      parseFrontmatter: true,
      mdxOptions: {
        remarkPlugins: [remarkGfm],
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

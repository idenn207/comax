import { visit } from 'unist-util-visit';
import type { Root } from 'mdast';

/** Trailing `{#slug}` marker; slug is ASCII lowercase, hyphen-separated. */
const HEADING_ID = /\s*\{#([a-z0-9]+(?:-[a-z0-9]+)*)\}\s*$/;

/**
 * Pin a heading's anchor id to the explicit `{#slug}` the author wrote, so
 * Korean headings still get ASCII lowercase-hyphen ids (project ID rule):
 *
 *   ## 빠른 시작 {#quick-start}   →   <h2 id="quick-start">빠른 시작</h2>
 *
 * The marker reaches mdast as literal text because `renderDoc` escapes the
 * braces (`\{ … \}`) before compile — MDX otherwise reads `{…}` as a JS
 * expression and fails. This runs before rehype-slug, which only slugs headings
 * that lack an id, so unmarked (already-English) headings keep their
 * github-slugger auto slug. Only the trailing marker is stripped; the visible
 * Korean copy stays.
 */
export default function remarkHeadingId() {
  return (tree: Root): void => {
    visit(tree, 'heading', (heading) => {
      const last = heading.children.at(-1);
      if (last?.type !== 'text') return;
      const match = HEADING_ID.exec(last.value);
      if (!match?.[1]) return;
      last.value = last.value.slice(0, match.index).replace(/\s+$/, '');
      heading.data = {
        ...heading.data,
        hProperties: { ...heading.data?.hProperties, id: match[1] },
      };
    });
  };
}

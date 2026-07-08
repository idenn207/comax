import Link from 'next/link';
import type { MDXComponents } from 'mdx/types';
import type { ReactNode } from 'react';
import { Info, TriangleAlert, ShieldAlert } from 'lucide-react';
import { cn } from '@/lib/cn';

// `note`/`warn`/`danger` are the original names; `info`/`warning` are aliases so
// .mdx can read semantically. All resolve to one of three tinted variants.
type CalloutType = 'note' | 'info' | 'warn' | 'warning' | 'danger';

type Variant = 'info' | 'warning' | 'danger';
const variantOf: Record<CalloutType, Variant> = {
  note: 'info',
  info: 'info',
  warn: 'warning',
  warning: 'warning',
  danger: 'danger',
};
const variantIcon: Record<Variant, typeof Info> = {
  info: Info,
  warning: TriangleAlert,
  danger: ShieldAlert,
};
// The icon is aria-hidden, so severity must reach screen readers as text —
// color/shape alone never carries meaning (DESIGN "색만으로 상태 전달 금지").
const variantLabel: Record<Variant, string> = {
  info: '정보',
  warning: '경고',
  danger: '위험',
};

// Tinted ground + tinted hairline + semantic icon (styles in globals.css
// `.doc-alert`). Meaning is carried by icon + tint, never a side-stripe.
function Callout({ type = 'note', children }: { type?: CalloutType; children: ReactNode }) {
  const variant = variantOf[type] ?? 'info';
  const Icon = variantIcon[variant];
  return (
    <div className={cn('doc-alert', `doc-alert--${variant}`)} role="note">
      <span className="doc-alert-icon" aria-hidden>
        <Icon className="h-4 w-4" />
      </span>
      <span className="sr-only">{variantLabel[variant]}: </span>
      <div className="doc-alert-body">{children}</div>
    </div>
  );
}

// Inline badge for concept prose (e.g. an "inherited" or "ref" marker), matching
// the design system's neutral/info/success chips. Color never stands alone —
// the label text carries the meaning.
function Badge({
  tone = 'neutral',
  children,
}: {
  tone?: 'neutral' | 'info' | 'success';
  children: ReactNode;
}) {
  return <span className={cn('doc-badge', `doc-badge--${tone}`)}>{children}</span>;
}

/**
 * MDX component map. Internal links go through next/link for client nav;
 * external links open safely. Callout is available directly in .mdx sources.
 * Prose text styling (headings, lists, tables) comes from the `prose` wrapper
 * in the docs page; code blocks are styled by rehype-pretty-code + globals.css.
 */
export const mdxComponents: MDXComponents = {
  // Link appearance (text-color + underline, brand on hover) is owned by the
  // `.prose a` rule in globals.css so docs links read as text, not blue links.
  a: ({ href = '', children, ...props }) => {
    const isInternal = href.startsWith('/') || href.startsWith('#');
    if (isInternal) {
      return <Link href={href}>{children}</Link>;
    }
    return (
      <a href={href} target="_blank" rel="noreferrer noopener" {...props}>
        {children}
      </a>
    );
  },
  // Wrap GFM tables in a scroll container so a long nowrap command cell
  // (e.g. `secret export --format github-env`) scrolls inside the block instead
  // of pushing the whole page into horizontal scroll on narrow viewports
  // (WCAG 1.4.10 Reflow). Uses plain global classes (`.doc-table`,
  // `.scrollbar-thin`) since this file is outside Tailwind's content globs.
  table: ({ children, ...props }) => (
    <div className="doc-table scrollbar-thin" role="region" aria-label="표" tabIndex={0}>
      <table {...props}>{children}</table>
    </div>
  ),
  Callout,
  Badge,
};

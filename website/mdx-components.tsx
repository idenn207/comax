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
  Callout,
  Badge,
};

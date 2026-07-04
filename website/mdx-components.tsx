import Link from 'next/link';
import type { MDXComponents } from 'mdx/types';
import type { ReactNode } from 'react';
import { Info, TriangleAlert, ShieldAlert } from 'lucide-react';
import { cn } from '@/lib/cn';

type CalloutType = 'note' | 'warn' | 'danger';

// Full border + soft background tint + colored icon carry the meaning. No
// side-stripe border (impeccable Absolute ban).
const calloutStyles: Record<CalloutType, { bg: string; iconColor: string; icon: typeof Info }> = {
  note: { bg: 'bg-info-soft', iconColor: 'text-info', icon: Info },
  warn: { bg: 'bg-warning-soft', iconColor: 'text-warning-strong', icon: TriangleAlert },
  danger: { bg: 'bg-danger-soft', iconColor: 'text-danger-strong', icon: ShieldAlert },
};

function Callout({ type = 'note', children }: { type?: CalloutType; children: ReactNode }) {
  const { bg, iconColor, icon: Icon } = calloutStyles[type];
  return (
    <div className={cn('my-5 flex gap-3 rounded-md border border-border px-4 py-3', bg)}>
      <Icon className={cn('mt-0.5 h-4 w-4 shrink-0', iconColor)} aria-hidden />
      <div className="text-sm text-text-subtle [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
        {children}
      </div>
    </div>
  );
}

/**
 * MDX component map. Internal links go through next/link for client nav;
 * external links open safely. Callout is available directly in .mdx sources.
 * Prose text styling (headings, lists, tables) comes from the `prose` wrapper
 * in the docs page; code blocks are styled by rehype-pretty-code + globals.css.
 */
export const mdxComponents: MDXComponents = {
  a: ({ href = '', children, ...props }) => {
    const isInternal = href.startsWith('/') || href.startsWith('#');
    if (isInternal) {
      return (
        <Link href={href} className="font-medium text-brand underline-offset-2 hover:underline">
          {children}
        </Link>
      );
    }
    return (
      <a
        href={href}
        target="_blank"
        rel="noreferrer noopener"
        className="font-medium text-brand underline-offset-2 hover:underline"
        {...props}
      >
        {children}
      </a>
    );
  },
  Callout,
};

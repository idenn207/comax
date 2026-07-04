import Link from 'next/link';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import type { DocNavItem } from '@/lib/docs-nav';
import { docHref } from '@/lib/docs-nav';

export function DocPager({ prev, next }: { prev: DocNavItem | null; next: DocNavItem | null }) {
  if (!prev && !next) return null;
  return (
    <nav
      aria-label="문서 이동"
      className="mt-14 grid gap-3 border-t border-border pt-6 sm:grid-cols-2"
    >
      {prev ? (
        <Link
          href={docHref(prev.slug)}
          className="group flex flex-col gap-1 rounded-lg border border-border p-4 transition-colors hover:border-border-strong"
        >
          <span className="flex items-center gap-1 text-xs text-muted">
            <ArrowLeft className="h-3.5 w-3.5" aria-hidden /> 이전
          </span>
          <span className="text-sm font-medium text-text">{prev.title}</span>
        </Link>
      ) : (
        <span />
      )}
      {next ? (
        <Link
          href={docHref(next.slug)}
          className="group flex flex-col items-end gap-1 rounded-lg border border-border p-4 text-right transition-colors hover:border-border-strong"
        >
          <span className="flex items-center gap-1 text-xs text-muted">
            다음 <ArrowRight className="h-3.5 w-3.5" aria-hidden />
          </span>
          <span className="text-sm font-medium text-text">{next.title}</span>
        </Link>
      ) : (
        <span />
      )}
    </nav>
  );
}

'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { docsNav, docHref } from '@/lib/docs-nav';
import { cn } from '@/lib/cn';

export function DocsSidebar() {
  const pathname = usePathname();
  return (
    <nav aria-label="문서 목차" className="flex flex-col gap-5">
      {docsNav.map((group) => (
        <div key={group.group}>
          {/* Group label is not a heading: it precedes the page <h1> in DOM
              order, so making it an <h2> breaks the document outline. The <ul>
              carries the group name programmatically via aria-label instead. */}
          <p
            aria-hidden
            className="mb-2 pl-3 text-xs font-semibold uppercase tracking-wider text-text-faint"
          >
            {group.group}
          </p>
          <ul aria-label={group.group} className="flex flex-col">
            {group.items.map((item) => {
              const href = docHref(item.slug);
              const active = pathname === href;
              return (
                <li key={href}>
                  <Link
                    href={href}
                    aria-current={active ? 'page' : undefined}
                    className={cn(
                      'block border-l py-1.5 pl-3 text-sm leading-snug transition-colors duration-fast',
                      active
                        ? 'border-text font-semibold text-text'
                        : 'border-border text-text-subtle hover:border-border-strong hover:text-text',
                    )}
                  >
                    {item.title}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </nav>
  );
}

'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { docsNav, docHref } from '@/lib/docs-nav';
import { cn } from '@/lib/cn';

export function DocsSidebar() {
  const pathname = usePathname();
  return (
    <nav aria-label="문서 목차" className="flex flex-col gap-6">
      {docsNav.map((group) => (
        <div key={group.group}>
          <h2 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wide text-muted">
            {group.group}
          </h2>
          <ul className="flex flex-col gap-0.5">
            {group.items.map((item) => {
              const href = docHref(item.slug);
              const active = pathname === href;
              return (
                <li key={href}>
                  <Link
                    href={href}
                    aria-current={active ? 'page' : undefined}
                    className={cn(
                      'block rounded-md px-3 py-1.5 text-sm transition-colors duration-fast',
                      active
                        ? 'bg-surface-active font-medium text-text'
                        : 'text-text-subtle hover:bg-surface-hover hover:text-text',
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

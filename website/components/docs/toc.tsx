'use client';

import { useEffect, useState } from 'react';
import type { TocItem } from '@/lib/docs-nav';
import { cn } from '@/lib/cn';

export function Toc({ items }: { items: TocItem[] }) {
  const [activeId, setActiveId] = useState<string>('');

  useEffect(() => {
    if (items.length === 0) return;
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        }
      },
      { rootMargin: '-80px 0px -70% 0px', threshold: 1 },
    );
    for (const item of items) {
      const el = document.getElementById(item.id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [items]);

  if (items.length === 0) return null;

  return (
    <nav aria-label="이 페이지 목차">
      <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted">이 페이지</p>
      <ul className="border-l border-border">
        {items.map((item) => (
          <li key={item.id} className={cn(item.depth === 3 && 'ml-3')}>
            <a
              href={`#${item.id}`}
              className={cn(
                '-ml-px block border-l py-1 pl-3 text-sm transition-colors duration-fast',
                activeId === item.id
                  ? 'border-text font-medium text-text'
                  : 'border-transparent text-text-faint hover:text-text-subtle',
              )}
            >
              {item.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}

'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import * as Dialog from '@radix-ui/react-dialog';
import { Search } from 'lucide-react';
import type { SearchDoc } from '@/lib/docs';

type Hit = { doc: SearchDoc; headingHits: { text: string; id: string }[] };

export function DocSearch({ index }: { index: SearchDoc[] }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  // Server renders the cross-platform "Ctrl K"; Mac clients correct to ⌘K after
  // mount (matches the handler, which accepts both metaKey and ctrlKey).
  const [isMac, setIsMac] = useState(false);
  const router = useRouter();

  useEffect(() => {
    const nav = typeof navigator !== 'undefined' ? navigator : undefined;
    setIsMac(/mac|iphone|ipad|ipod/i.test(nav?.platform || nav?.userAgent || ''));
  }, []);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen((o) => !o);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const results = useMemo<Hit[]>(() => {
    const term = query.trim().toLowerCase();
    if (!term) return index.map((doc) => ({ doc, headingHits: [] }));
    const hits: Hit[] = [];
    for (const doc of index) {
      const headingHits = doc.headings.filter((h) => h.text.toLowerCase().includes(term));
      const metaHit =
        doc.title.toLowerCase().includes(term) || doc.description.toLowerCase().includes(term);
      if (metaHit || headingHits.length > 0) hits.push({ doc, headingHits });
    }
    return hits;
  }, [query, index]);

  function go(href: string) {
    setOpen(false);
    setQuery('');
    router.push(href);
  }

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <button
          type="button"
          aria-keyshortcuts="Control+K Meta+K"
          className="flex w-full items-center gap-2 rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm text-text-faint transition-colors hover:border-border-strong"
        >
          <Search className="h-4 w-4" aria-hidden />
          <span className="flex-1 text-left">문서 검색</span>
          <kbd
            aria-hidden
            className="rounded border border-border px-1.5 py-0.5 font-mono text-xs text-muted"
          >
            {isMac ? '⌘K' : 'Ctrl K'}
          </kbd>
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-[var(--color-overlay)]" />
        <Dialog.Content
          className="fixed left-1/2 top-[12vh] z-50 w-[min(36rem,92vw)] -translate-x-1/2 overflow-hidden rounded-xl border border-border bg-surface-elevated shadow-lg focus:outline-none"
          aria-describedby={undefined}
        >
          <Dialog.Title className="sr-only">문서 검색</Dialog.Title>
          <div className="flex items-center gap-2 border-b border-border px-4">
            <Search className="h-4 w-4 text-muted" aria-hidden />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="문서 검색..."
              className="w-full bg-transparent py-3 text-md text-text outline-none placeholder:text-text-faint"
            />
          </div>
          <ul className="scrollbar-thin max-h-[60vh] overflow-y-auto p-2">
            {results.length === 0 && (
              <li className="px-3 py-6 text-center text-sm text-text-faint">결과가 없습니다.</li>
            )}
            {results.map(({ doc, headingHits }) => (
              <li key={doc.slug || 'index'} className="mb-1">
                <button
                  type="button"
                  onClick={() => go(doc.href)}
                  className="flex w-full flex-col rounded-md px-3 py-2 text-left transition-colors hover:bg-surface-hover"
                >
                  <span className="text-sm font-medium text-text">{doc.title}</span>
                  {doc.description && (
                    <span className="line-clamp-1 text-xs text-text-faint">{doc.description}</span>
                  )}
                </button>
                {headingHits.map((h) => (
                  <button
                    key={h.id}
                    type="button"
                    onClick={() => go(`${doc.href}#${h.id}`)}
                    className="block w-full rounded-md px-3 py-1.5 pl-6 text-left text-sm text-text-subtle transition-colors hover:bg-surface-hover"
                  >
                    {h.text}
                  </button>
                ))}
              </li>
            ))}
          </ul>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

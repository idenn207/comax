import { ChevronDown } from 'lucide-react';
import { DocsSidebar } from '@/components/docs/sidebar';
import { DocSearch } from '@/components/docs/search';
import { DocsEnhance } from '@/components/docs/enhance';
import { buildSearchIndex } from '@/lib/docs';

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  const searchIndex = buildSearchIndex();
  return (
    <div className="mx-auto max-w-content px-4 sm:px-6">
      <DocsEnhance />
      <div className="lg:grid lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-10">
        <aside className="hidden lg:block">
          <div className="scrollbar-thin sticky top-[var(--header-height)] max-h-[calc(100dvh-var(--header-height))] overflow-y-auto py-8 pr-2">
            <div className="mb-6">
              <DocSearch index={searchIndex} />
            </div>
            <DocsSidebar />
          </div>
        </aside>
        <div className="min-w-0 py-8">
          {/* Mobile/tablet (<lg): the desktop sidebar is display:none, so docs
              must stay browsable here — search plus a collapsible nav tree. */}
          <div className="mb-8 flex flex-col gap-3 lg:hidden">
            <DocSearch index={searchIndex} />
            <details className="group rounded-md border border-border bg-surface-elevated">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2.5 text-sm font-medium text-text-subtle transition-colors hover:text-text [&::-webkit-details-marker]:hidden">
                <span>문서 목차</span>
                <ChevronDown
                  className="h-4 w-4 transition-transform duration-fast group-open:rotate-180"
                  aria-hidden
                />
              </summary>
              <div className="border-t border-border px-3 py-4">
                <DocsSidebar />
              </div>
            </details>
          </div>
          {children}
        </div>
      </div>
    </div>
  );
}

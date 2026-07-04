import { DocsSidebar } from '@/components/docs/sidebar';
import { DocSearch } from '@/components/docs/search';
import { buildSearchIndex } from '@/lib/docs';

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  const searchIndex = buildSearchIndex();
  return (
    <div className="mx-auto max-w-content px-4 sm:px-6">
      <div className="lg:grid lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-10">
        <aside className="hidden lg:block">
          <div className="sticky top-[var(--header-height)] max-h-[calc(100dvh-var(--header-height))] overflow-y-auto py-8 pr-2">
            <div className="mb-6">
              <DocSearch index={searchIndex} />
            </div>
            <DocsSidebar />
          </div>
        </aside>
        <div className="min-w-0 py-8">
          <div className="mb-6 lg:hidden">
            <DocSearch index={searchIndex} />
          </div>
          {children}
        </div>
      </div>
    </div>
  );
}

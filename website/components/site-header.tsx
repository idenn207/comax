import Link from 'next/link';
import { Logo } from '@/components/logo';
import { ThemeToggle } from '@/components/theme-toggle';
import { MobileNav } from '@/components/mobile-nav';
import { ButtonLink } from '@/components/ui/button-link';
import { mainNav } from '@/lib/site';

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-30 border-b border-border bg-[oklch(98.5%_0.002_260/0.72)] backdrop-blur-md backdrop-saturate-150 [[data-theme='dark']_&]:bg-[oklch(15%_0.006_260/0.72)]">
      <div className="mx-auto flex h-[var(--header-height)] max-w-content items-center gap-6 px-4 sm:px-6">
        <Link href="/" className="shrink-0 rounded-sm" aria-label="Comax Secrets 홈">
          <Logo />
        </Link>

        <nav className="hidden flex-1 items-center gap-0.5 md:flex" aria-label="주요">
          {mainNav.map((item) =>
            item.external ? (
              <a
                key={item.href}
                href={item.href}
                target="_blank"
                rel="noreferrer noopener"
                className="rounded-md px-3 py-1.5 text-sm text-text-subtle transition-colors duration-fast hover:bg-surface-hover hover:text-text"
              >
                {item.title}
              </a>
            ) : (
              <Link
                key={item.href}
                href={item.href}
                className="rounded-md px-3 py-1.5 text-sm text-text-subtle transition-colors duration-fast hover:bg-surface-hover hover:text-text"
              >
                {item.title}
              </Link>
            ),
          )}
        </nav>

        <div className="ml-auto flex items-center gap-2 md:ml-0">
          <ThemeToggle />
          <ButtonLink href="/docs" size="sm" className="hidden sm:inline-flex">
            무료로 시작
          </ButtonLink>
          <MobileNav />
        </div>
      </div>
    </header>
  );
}

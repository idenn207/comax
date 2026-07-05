import Link from 'next/link';
import { Logo } from '@/components/logo';
import { siteConfig } from '@/lib/site';

const footerNav: { title: string; links: { label: string; href: string; external?: boolean }[] }[] =
  [
    {
      title: '문서',
      links: [
        { label: 'Quickstart', href: '/docs/quickstart' },
        { label: 'Self-host', href: '/docs/self-host' },
        { label: 'CLI 레퍼런스', href: '/docs/cli' },
        { label: 'SDK', href: '/docs/sdk' },
      ],
    },
    {
      title: '통합',
      links: [
        { label: 'GitHub Actions', href: '/docs/github-actions' },
        { label: 'Webhooks', href: '/docs/webhooks' },
        { label: '보안 모델', href: '/docs/security' },
      ],
    },
    {
      title: '프로젝트',
      links: [
        { label: 'GitHub', href: siteConfig.repo, external: true },
        { label: 'npm (@comax-secrets/sdk)', href: 'https://www.npmjs.com/package/@comax-secrets/sdk', external: true },
      ],
    },
  ];

export function SiteFooter() {
  return (
    <footer className="border-t border-border">
      <div className="mx-auto grid max-w-content grid-cols-2 gap-8 px-4 py-12 sm:px-6 md:grid-cols-[1.4fr_1fr_1fr_1fr]">
        <div className="col-span-2 md:col-span-1">
          <Logo />
          <p className="mt-3 max-w-xs text-sm text-text-faint">{siteConfig.tagline}. {siteConfig.license} 라이선스.</p>
        </div>
        {footerNav.map((col) => (
          <div key={col.title}>
            <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted">
              {col.title}
            </h2>
            <ul className="flex flex-col gap-2">
              {col.links.map((link) => (
                <li key={link.href}>
                  {link.external ? (
                    <a
                      href={link.href}
                      target="_blank"
                      rel="noreferrer noopener"
                      className="text-sm text-text-subtle transition-colors hover:text-brand"
                    >
                      {link.label}
                    </a>
                  ) : (
                    <Link
                      href={link.href}
                      className="text-sm text-text-subtle transition-colors hover:text-brand"
                    >
                      {link.label}
                    </Link>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="border-t border-border">
        <div className="mx-auto max-w-content px-4 py-4 text-xs text-text-faint sm:px-6">
          © {siteConfig.name}. Self-hosted secret management.
        </div>
      </div>
    </footer>
  );
}

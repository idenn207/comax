import Link from 'next/link';
import { Logo } from '@/components/logo';
import { siteConfig } from '@/lib/site';

const footerNav: { title: string; links: { label: string; href: string; external?: boolean }[] }[] =
  [
    {
      title: '시작하기',
      links: [
        { label: '설치 가이드', href: '/docs' },
        { label: '동작 방식', href: '/#how' },
        { label: '셀프호스팅 가이드', href: '/docs/self-host' },
      ],
    },
    {
      title: '더 알아보기',
      links: [
        { label: 'CLI 문서', href: '/docs/cli' },
        { label: 'SDK', href: '/docs/sdk' },
        { label: '대시보드 미리보기', href: '/#demo' },
      ],
    },
    {
      title: '프로젝트',
      links: [
        { label: 'GitHub', href: siteConfig.repo, external: true },
        {
          label: 'npm (@comax-secrets/sdk)',
          href: 'https://www.npmjs.com/package/@comax-secrets/sdk',
          external: true,
        },
      ],
    },
  ];

export function SiteFooter() {
  return (
    <footer className="border-t border-border">
      <div className="mx-auto grid max-w-content grid-cols-2 gap-8 px-4 py-12 sm:px-6 md:grid-cols-[1.7fr_1fr_1fr_1fr]">
        <div className="col-span-2 md:col-span-1">
          <Logo />
          <p className="mt-3 max-w-xs text-sm text-text-faint break-keep">
            흩어진 시크릿을 한곳에서 안전하게. 오픈소스 · {siteConfig.license} 라이선스.
          </p>
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
                      className="text-sm text-text-subtle transition-colors hover:text-text"
                    >
                      {link.label}
                    </a>
                  ) : (
                    <Link
                      href={link.href}
                      className="text-sm text-text-subtle transition-colors hover:text-text"
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
          © {siteConfig.name}. 내 인프라 안의 시크릿 매니저.
        </div>
      </div>
    </footer>
  );
}

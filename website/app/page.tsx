import type { Metadata } from 'next';
import { ArrowRight } from 'lucide-react';
import { ButtonLink } from '@/components/ui/button-link';
import { CommandBlock } from '@/components/command-block';
import { HeroGraphic } from '@/components/hero-graphic';
import { FeatureTabs } from '@/components/feature-tabs';
import { pageMetadata } from '@/lib/metadata';
import { siteConfig } from '@/lib/site';

export const metadata: Metadata = pageMetadata({ path: '/' });

const quickstartLines = [
  { kind: 'command' as const, text: 'docker compose up -d' },
  { kind: 'command' as const, text: 'secret push --env dev .env' },
  { kind: 'command' as const, text: 'secret run --env dev -- npm run dev' },
];

// A minimal drift matrix: which env holds which key. The whole product exists
// to make the "누락" cells visible before a deploy does.
const driftRows: { key: string; cells: ('ok' | 'miss')[] }[] = [
  { key: 'DATABASE_URL', cells: ['ok', 'ok', 'ok'] },
  { key: 'REDIS_URL', cells: ['ok', 'ok', 'ok'] },
  { key: 'STRIPE_KEY', cells: ['ok', 'ok', 'miss'] },
  { key: 'SENTRY_DSN', cells: ['ok', 'miss', 'ok'] },
];

const jsonLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: siteConfig.name,
  applicationCategory: 'DeveloperApplication',
  operatingSystem: 'Linux, macOS, Windows (Docker)',
  offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
  description: siteConfig.description,
  license: 'https://opensource.org/licenses/MIT',
  url: siteConfig.url,
};

export default function HomePage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />

      {/* ── Hero: teal flow graphic ── */}
      <section className="relative overflow-hidden border-b border-border">
        <div className="mx-auto grid max-w-content items-center gap-10 px-4 py-20 sm:px-6 lg:grid-cols-[1fr_1.08fr] lg:py-24">
          <div>
            <h1 className="text-display font-bold leading-[1.12] text-text">
              시크릿 하나의 출처,
              <br />
              <span className="text-brand">모든 환경</span>이 같은 상태
            </h1>
            <p className="mt-5 max-w-xl text-lg leading-relaxed text-text-subtle">
              worktree, GitHub Actions, prod까지 <span className="font-mono">.env</span> 사본을 손으로
              맞추는 일을 멈춘다. 한 서버가 진실의 출처가 되고, 환경마다 무엇이 빠졌는지 곧바로 보인다.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <ButtonLink href="/docs/quickstart">
                5분 Quickstart
                <ArrowRight className="h-4 w-4" aria-hidden />
              </ButtonLink>
              <ButtonLink href="/docs" variant="secondary">
                문서 읽기
              </ButtonLink>
            </div>
            <p className="mt-5 text-sm text-muted">
              <span className="font-mono text-text">SQLite</span> 한 개로 부팅 · 외부 의존성{' '}
              <span className="font-mono text-text">0</span> · self-host · MIT
            </p>
          </div>
          <div className="mx-auto w-full max-w-xl">
            <HeroGraphic />
          </div>
        </div>
      </section>

      {/* ── Problem: empathy + drift matrix ── */}
      <section className="border-b border-border">
        <div className="mx-auto grid max-w-content items-center gap-14 px-4 py-16 sm:px-6 lg:grid-cols-[0.9fr_1.1fr] lg:py-20">
          <div>
            <h2 className="text-2xl font-semibold text-text">
              서비스가 늘수록,
              <br />
              .env는 조용히 어긋난다
            </h2>
            <p className="mt-4 max-w-md text-md text-text-subtle">
              혼자 여러 프로젝트를 굴리면 서비스 × 환경마다 <span className="font-mono">.env</span>{' '}
              사본이 쌓인다. prod에만 빠진 키 하나는 배포가 터지기 전까지 보이지 않는다.
            </p>
            <p className="mt-5 border-t border-border pt-4 text-sm text-muted">
              Comax Secrets는 이 어긋남을 <b className="font-semibold text-text">기본 화면</b>으로
              끌어올린다. “없음”이 아니라 “있어야 하는데 없음”을 표시한다.
            </p>
          </div>

          <div className="overflow-hidden rounded-xl border border-border bg-surface-elevated">
            <div className="grid grid-cols-[1.4fr_1fr_1fr_1fr] border-b border-border bg-panel font-mono text-xs text-muted">
              <div className="px-3 py-2.5">key</div>
              <div className="px-3 py-2.5">dev</div>
              <div className="px-3 py-2.5">ci</div>
              <div className="px-3 py-2.5">prod</div>
            </div>
            {driftRows.map((row) => (
              <div
                key={row.key}
                className="grid grid-cols-[1.4fr_1fr_1fr_1fr] border-b border-border font-mono text-[0.8rem] last:border-b-0"
              >
                <div className="px-3 py-2.5 text-text">{row.key}</div>
                {row.cells.map((cell, i) => (
                  <div key={i} className="flex items-center px-3 py-2.5">
                    {cell === 'ok' ? (
                      <span className="h-1.5 w-1.5 rounded-full bg-success" aria-label="있음" />
                    ) : (
                      <span className="rounded bg-warning-soft px-2 py-0.5 text-[0.72rem] font-semibold text-[var(--color-warning-strong)]">
                        누락
                      </span>
                    )}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Features: one interactive stage, four acts ── */}
      <section className="border-b border-border">
        <div className="mx-auto max-w-content px-4 py-16 sm:px-6 lg:py-20">
          <div className="mb-9 max-w-2xl">
            <p className="font-mono text-xs font-semibold text-brand-strong">how it works</p>
            <h2 className="mt-2.5 text-2xl font-semibold text-text">한 서버가 하는 네 가지 일</h2>
            <p className="mt-3 text-md text-text-subtle">
              같은 출처에서 런타임·CI·컨테이너로 시크릿을 내보낸다. 탭을 눌러 각각이 실제로 어떻게
              도는지 본다.
            </p>
          </div>
          <FeatureTabs />
        </div>
      </section>

      {/* ── Quickstart: the one Committed teal band ── */}
      <section className="bg-brand text-brand-text">
        <div className="mx-auto grid max-w-content items-center gap-11 px-4 py-16 sm:px-6 lg:grid-cols-[1fr_1.05fr] lg:py-20">
          <div>
            <p className="font-mono text-xs opacity-80">get started</p>
            <h2 className="mt-2.5 text-2xl font-semibold">부팅에서 주입까지, 세 단계</h2>
            <p className="mt-3 max-w-md opacity-90">
              클린 VM에서 2분. GitHub Secret 등록도, 환경별 <span className="font-mono">.env</span>{' '}
              복사도 없다.
            </p>
            <div className="mt-6 flex flex-wrap gap-3">
              <ButtonLink href="/docs/self-host" variant="oncolor">
                Self-host 가이드
                <ArrowRight className="h-4 w-4" aria-hidden />
              </ButtonLink>
              <ButtonLink href="/docs" variant="ghost">
                전체 문서
              </ButtonLink>
            </div>
          </div>
          <CommandBlock label="setup — 3 steps" lines={quickstartLines} />
        </div>
      </section>
    </>
  );
}

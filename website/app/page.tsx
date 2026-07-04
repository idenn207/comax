import type { Metadata } from 'next';
import { ArrowRight, GitBranch, HardDrive, GitPullRequestArrow, Diff } from 'lucide-react';
import { ButtonLink } from '@/components/ui/button-link';
import { CommandBlock } from '@/components/command-block';
import { pageMetadata } from '@/lib/metadata';
import { siteConfig } from '@/lib/site';

export const metadata: Metadata = pageMetadata({ path: '/' });

const axes = [
  {
    icon: HardDrive,
    title: 'NAS·홈랩에서 그냥 뜬다',
    body: 'SQLite 파일 하나를 마운트하고 docker compose up 한 줄. 외부 Postgres나 Redis를 요구하지 않는다. 저사양 self-host 환경을 1순위로 설계했다.',
    featured: true,
  },
  {
    icon: GitBranch,
    title: 'Worktree를 1급으로',
    body: '.secretrc와 git branch 매핑으로 worktree마다 올바른 환경이 자동으로 붙는다. secret run -- <cmd> 한 줄이면 그 프로세스에 시크릿이 주입된다.',
  },
  {
    icon: GitPullRequestArrow,
    title: 'GitHub Secret 등록이 사라진다',
    body: 'load-action 한 줄이 지정한 environment의 시크릿을 step env로 주입한다. 저장소마다 Secret을 다시 등록하는 절차 자체가 없어진다.',
  },
  {
    icon: Diff,
    title: '환경 간 누락이 보인다',
    body: 'local에는 있는데 prod에는 없는 키를 대시보드가 전용 신호로 표시한다. 참조와 오버라이드로 공통 값을 한 곳에서 관리한다.',
  },
];

const heroLines = [
  { kind: 'comment' as const, text: '# 어느 worktree에서든, 환경이 자동으로 붙는다' },
  { kind: 'command' as const, text: 'secret run -- npm run dev' },
  { kind: 'output' as const, text: '→ web/dev 환경 시크릿 14개 주입, 자식 프로세스로 전달' },
  { kind: 'command' as const, text: 'secret diff local prod' },
  { kind: 'output' as const, text: '  + STRIPE_KEY   (local 에만 있음)' },
  { kind: 'output' as const, text: '  ~ DB_HOST      (값 다름)' },
];

const quickstartLines = [
  { kind: 'comment' as const, text: '# 1. 서버 부팅 (SQLite 한 개, 외부 의존성 없음)' },
  { kind: 'command' as const, text: 'docker compose up -d' },
  { kind: 'comment' as const, text: '# 2. 로그인하고 환경을 만든 뒤 .env를 올린다' },
  { kind: 'command' as const, text: 'secret login && secret init web' },
  { kind: 'command' as const, text: 'secret push --env dev .env' },
  { kind: 'comment' as const, text: '# 3. 어디서든 주입해서 실행' },
  { kind: 'command' as const, text: 'secret run --env dev -- npm run dev' },
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

      {/* ── Hero ── */}
      <section className="border-b border-border">
        <div className="mx-auto grid max-w-content gap-12 px-4 py-16 sm:px-6 lg:grid-cols-[1.05fr_1fr] lg:items-center lg:py-24">
          <div>
            <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-border bg-surface-elevated px-3 py-1 text-xs font-medium text-text-subtle">
              <span className="h-1.5 w-1.5 rounded-full bg-success" aria-hidden />
              self-host · MIT · zero-dep SDK
            </p>
            <h1 className="text-hero font-semibold leading-[var(--line-tight)] text-text">
              시크릿을 <span className="text-brand">내 인프라</span>에 두고,
              <br />
              환경 동기화를 손에서 뗀다
            </h1>
            <p className="mt-6 max-w-xl text-lg leading-[var(--line-normal)] text-text-subtle">
              multi-service × multi-environment의 .env 사본을 손으로 맞추는 일을 멈춘다. Comax
              Secrets는 SQLite 하나로 부팅하고, worktree와 GitHub Actions까지 같은 소스에서 시크릿을
              주입하는 가벼운 self-host 도구다.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <ButtonLink href="/docs/quickstart">
                5분 Quickstart
                <ArrowRight className="h-4 w-4" aria-hidden />
              </ButtonLink>
              <ButtonLink href={siteConfig.repo} variant="secondary" external>
                GitHub
              </ButtonLink>
            </div>
          </div>
          <CommandBlock label="terminal — secret" lines={heroLines} />
        </div>
      </section>

      {/* ── 4-axis USP (asymmetric bento) ── */}
      <section className="border-b border-border">
        <div className="mx-auto max-w-content px-4 py-16 sm:px-6 lg:py-20">
          <h2 className="max-w-2xl text-3xl font-semibold text-text">
            Infisical·Doppler가 무거운 자리에서, 네 가지가 다르다
          </h2>
          <p className="mt-3 max-w-2xl text-md text-text-subtle">
            SaaS 종속도, 무거운 self-host 운영 부담도 없이 개인 개발자의 실제 페인에 맞춘다.
          </p>
          <div className="mt-10 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {axes.map((axis) => {
              const Icon = axis.icon;
              return (
                <article
                  key={axis.title}
                  className={cnFeatured(axis.featured)}
                >
                  <Icon className="h-5 w-5 text-text-subtle" aria-hidden />
                  <h3 className="mt-4 text-lg font-semibold text-text">{axis.title}</h3>
                  <p className="mt-2 text-sm leading-[var(--line-normal)] text-text-subtle">
                    {axis.body}
                  </p>
                </article>
              );
            })}
          </div>
        </div>
      </section>

      {/* ── Quickstart teaser ── */}
      <section className="border-b border-border">
        <div className="mx-auto grid max-w-content gap-12 px-4 py-16 sm:px-6 lg:grid-cols-[1fr_1.1fr] lg:items-center lg:py-20">
          <div>
            <h2 className="text-3xl font-semibold text-text">부팅에서 주입까지, 세 단계</h2>
            <p className="mt-4 max-w-md text-md text-text-subtle">
              클린 VM에서 docker compose up 이후 2분 안에 첫 시크릿을 주입한다. GitHub Secret 등록도,
              환경별 .env 복사도 없다.
            </p>
            <div className="mt-8">
              <ButtonLink href="/docs/quickstart" variant="secondary">
                전체 Quickstart 보기
                <ArrowRight className="h-4 w-4" aria-hidden />
              </ButtonLink>
            </div>
          </div>
          <CommandBlock label="setup — 3 steps" lines={quickstartLines} />
        </div>
      </section>

      {/* ── Final CTA ── */}
      <section>
        <div className="mx-auto max-w-content px-4 py-20 text-center sm:px-6">
          <h2 className="mx-auto max-w-2xl text-3xl font-semibold text-text">
            12개의 .env를 손으로 맞추는 일을 오늘 끝낸다
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-md text-text-subtle">
            self-host 가이드를 따라 서버를 올리고, SDK와 Action으로 런타임·CI까지 같은 소스에 연결한다.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <ButtonLink href="/docs/quickstart">시작하기</ButtonLink>
            <ButtonLink href="/docs/self-host" variant="secondary">
              Self-host 가이드
            </ButtonLink>
          </div>
        </div>
      </section>
    </>
  );
}

// Featured axis (NAS self-host) spans wider on large screens and gets a
// stronger border — the DESIGN.md "featured = padding + border-strong, no side
// stripe" rule, carried over from the dashboard bento.
function cnFeatured(featured?: boolean): string {
  const base =
    'rounded-lg border border-border bg-surface-elevated p-6 transition-colors duration-fast hover:border-border-strong';
  return featured ? `${base} lg:col-span-1 lg:border-border-strong` : base;
}

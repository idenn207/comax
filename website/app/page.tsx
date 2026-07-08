import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import {
  AlertTriangle,
  ArrowRight,
  Check,
  Database,
  Info,
  LayoutGrid,
  Lock,
  MonitorSmartphone,
  PlayCircle,
  RefreshCw,
  Server,
  ShieldCheck,
  X,
  Zap,
} from 'lucide-react';
import { ButtonLink } from '@/components/ui/button-link';
import { HeroStage } from '@/components/hero-stage';
import { SecretDemo } from '@/components/secret-demo';
import { MotionReady } from '@/components/motion-ready';
import { pageMetadata } from '@/lib/metadata';
import { siteConfig } from '@/lib/site';

export const metadata: Metadata = pageMetadata({ path: '/' });

const wrap = 'mx-auto w-full max-w-content px-4 sm:px-6';
const sec = 'py-[clamp(3.5rem,8vw,6.5rem)]';

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

const mono = 'font-mono text-text';

const beforePoints: ReactNode[] = [
  <>
    프로젝트마다 다른 <code className={mono}>.env</code>, 어느 게 최신인지 모른다
  </>,
  'API 키를 메신저로 주고받고, 회수는 잊힌다',
  '"내 컴퓨터에선 됐는데"로 배포가 멈춘다',
  '누가 언제 무엇을 바꿨는지 아무도 모른다',
];

const afterPoints: ReactNode[] = [
  '모든 값이 한곳에, 항상 최신 상태로 모인다',
  '값은 늘 마스킹되고, 안전하게 주입된다',
  '어느 환경에 무엇이 빠졌는지 바로 보인다',
  '모든 변경이 이력에 남고, 언제든 롤백된다',
];

const steps = [
  {
    n: '01',
    Icon: Database,
    title: '한곳에 모읍니다',
    body: (
      <>
        흩어져 있던 값을 Comax에 올립니다. 기존 <code className={mono}>.env</code>가 있다면 그대로
        가져오면 됩니다.
      </>
    ),
  },
  {
    n: '02',
    Icon: LayoutGrid,
    title: '환경별로 정리합니다',
    body: (
      <>
        <strong className="font-semibold text-text">환경</strong>별로 값을 나눠 관리합니다.
        dev·staging·prod처럼요. 공통 값은 한 번만 정의하면 됩니다.
      </>
    ),
  },
  {
    n: '03',
    Icon: Zap,
    title: '필요한 곳에 주입합니다',
    body: '클릭 한 번 또는 명령 한 줄로, 로컬·CI·배포 서버 어디든 값을 주입합니다. 복사·붙여넣기는 없습니다.',
  },
];

const benefits = [
  {
    Icon: ShieldCheck,
    title: '안전합니다',
    body: '값은 항상 마스킹되고, 누가 언제 무엇을 바꿨는지 이력에 남습니다. 실수로 유출될 위험을 크게 줄입니다.',
  },
  {
    Icon: RefreshCw,
    title: '어긋나지 않습니다',
    body: '"내 환경에선 됐는데"가 사라집니다. 환경 간 누락·불일치를 자동으로 짚어 줍니다.',
  },
  {
    Icon: Server,
    title: '내 것입니다',
    body: '값을 외부 클라우드에 맡기지 않고 자체 서버에 둡니다. 오픈소스라 무료이고, 필요하면 코드까지 열어볼 수 있습니다.',
  },
  {
    Icon: MonitorSmartphone,
    title: 'CLI 없이도 됩니다',
    body: '대시보드에서 값 확인·환경 비교·롤백까지 끝냅니다. 터미널이 익숙하면 CLI로 똑같이 처리할 수 있습니다.',
  },
];

function Kicker({ no, children }: { no: string; children: ReactNode }) {
  return (
    <div className="reveal mb-5 flex items-baseline gap-3">
      <span className="font-mono text-xs tracking-[0.1em] text-text-faint">{no}</span>
      <span className="inline-flex items-center gap-2 font-mono text-xs uppercase tracking-[0.16em] text-muted">
        {children}
      </span>
      <span className="h-px flex-1 bg-border" />
    </div>
  );
}

const secHeading = 'text-3xl font-semibold leading-[1.14] tracking-[-0.02em] text-balance break-keep';
const lead = 'text-lg leading-normal text-text-subtle text-pretty break-keep';

export default function HomePage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <MotionReady />

      {/* ── Hero ── */}
      <section
        className={`${wrap} overflow-x-clip pb-[clamp(3.5rem,8vw,6.5rem)] pt-[clamp(2.5rem,6vw,5rem)]`}
      >
        <div className="grid items-center gap-[clamp(2.25rem,5vw,4.5rem)] lg:grid-cols-[1.02fr_0.98fr]">
          <div>
            <span className="reveal inline-flex items-center gap-2 font-mono text-xs uppercase tracking-[0.16em] text-muted">
              <span className="h-[7px] w-[7px] rounded-full bg-success" />
              오픈소스 · 셀프호스팅 시크릿 매니저
            </span>
            <h1 className="reveal mt-5 text-[clamp(2.5rem,6vw,4.1rem)] font-semibold leading-[1.03] tracking-[-0.035em] text-balance break-keep">
              흩어진 시크릿을,
              <br />
              <span className="text-brand">한곳에서</span> 안전하게.
            </h1>
            <p className={`reveal mt-5 max-w-xl ${lead}`}>
              API 키, DB 비밀번호, 액세스 토큰. 서비스를 움직이는{' '}
              <strong className="font-semibold text-text">시크릿</strong>이 팀원 노트북과 메신저,{' '}
              <code className="font-mono text-[0.88em] text-text">.env</code> 파일에 흩어져 있지
              않나요? Comax는 이 값들을 한곳에서 관리하고, 필요한 환경에 자동으로 주입합니다.
            </p>
            <div className="reveal mt-7 flex flex-wrap gap-3">
              <ButtonLink href="/docs" size="lg">
                무료로 시작하기
                <ArrowRight className="h-4 w-4" aria-hidden />
              </ButtonLink>
              <ButtonLink href="#how" variant="outline" size="lg">
                <PlayCircle className="h-4 w-4" aria-hidden />
                동작 방식 보기
              </ButtonLink>
            </div>
            <ul className="reveal mt-8 flex flex-wrap gap-x-5 gap-y-2 text-xs text-text-faint">
              {['오픈소스 · MIT', 'SQLite 하나로 실행', '셀프호스팅'].map((t) => (
                <li key={t} className="inline-flex items-center gap-1.5">
                  <Check className="h-3.5 w-3.5" aria-hidden />
                  {t}
                </li>
              ))}
            </ul>
          </div>

          <div className="reveal" style={{ transitionDelay: '0.12s' }}>
            {/* Slight scale-down on phones keeps the outermost secret chips
                inside the frame (they sit at the stage edges by design). */}
            <div className="origin-top scale-[0.82] sm:scale-100">
              <HeroStage />
            </div>
          </div>
        </div>
      </section>

      {/* ── What is a secret? (full-bleed divider + panel) ── */}
      <section className="border-t border-border bg-panel">
        <div className={`${wrap} flex flex-wrap items-start gap-4 py-7`}>
          <span className="grid h-[34px] w-[34px] shrink-0 place-items-center rounded-md border border-border bg-surface-elevated text-brand">
            <Info className="h-[18px] w-[18px]" aria-hidden />
          </span>
          <p className="min-w-[260px] flex-1 text-md leading-normal text-text-subtle break-keep">
            <strong className="font-semibold text-text">시크릿(secret)</strong>은 코드에 그대로 두면
            안 되는 민감한 값입니다. <span className="text-text">API 키</span>, DB{' '}
            <span className="text-text">비밀번호</span>, 액세스 <span className="text-text">토큰</span>
            처럼요. 보통 <code className="font-mono text-[0.9em] text-text">.env</code>로 관리하지만,
            팀원마다·환경마다 값이 달라지면서 문제가 시작됩니다.
          </p>
        </div>
      </section>

      {/* ── Why (before / after) ── */}
      <section id="why" className="scroll-mt-20 border-t border-border">
        <div className={`${wrap} ${sec}`}>
          <Kicker no="01">왜 한곳에 모아야 할까요</Kicker>
          <h2 className={`reveal max-w-[22rem] ${secHeading}`}>
            흩어진 시크릿은,
            <br />
            결국 사고가 됩니다
          </h2>
          <p className={`reveal mt-4 max-w-[40rem] ${lead}`} style={{ transitionDelay: '0.05s' }}>
            값이 팀원마다·환경마다 달라지면, 하나만 어긋나도 배포가 막히고 실수로 커밋된 키 하나가
            유출로 이어집니다. 한곳에서 관리하면 이런 위험이 사라집니다.
          </p>

          <div className="reveal mt-10 grid gap-[18px] md:grid-cols-2">
            <div className="rounded-lg border bg-surface-elevated p-[26px] [border-color:color-mix(in_srgb,var(--color-danger)_26%,var(--color-border))]">
              <div className="flex items-center gap-2.5 text-xs font-bold uppercase tracking-[0.1em] text-danger-strong">
                <AlertTriangle className="h-[15px] w-[15px]" aria-hidden />
                지금 (Comax 없이)
              </div>
              <ul className="mt-[18px] flex flex-col gap-3.5">
                {beforePoints.map((p, i) => (
                  <li key={i} className="flex gap-3 text-sm leading-snug text-text-subtle break-keep">
                    <X className="mt-px h-4 w-4 shrink-0 text-danger" aria-hidden />
                    <span>{p}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="rounded-lg border bg-surface-elevated p-[26px] [border-color:color-mix(in_srgb,var(--color-success)_30%,var(--color-border))]">
              <div className="flex items-center gap-2.5 text-xs font-bold uppercase tracking-[0.1em] text-success-strong">
                <Check className="h-[15px] w-[15px]" aria-hidden />
                Comax와 함께
              </div>
              <ul className="mt-[18px] flex flex-col gap-3.5">
                {afterPoints.map((p, i) => (
                  <li key={i} className="flex gap-3 text-sm leading-snug text-text-subtle break-keep">
                    <Check className="mt-px h-4 w-4 shrink-0 text-success" aria-hidden />
                    <span>{p}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* ── How it works (3 steps) ── */}
      <section id="how" className="scroll-mt-20 border-t border-border">
        <div className={`${wrap} ${sec}`}>
          <Kicker no="02">동작 방식</Kicker>
          <h2 className={`reveal max-w-[24rem] ${secHeading}`}>세 단계면 충분합니다</h2>
          <p className={`reveal mt-4 max-w-[38rem] ${lead}`} style={{ transitionDelay: '0.05s' }}>
            설치부터 사용까지, 흐름은 단순합니다. 아래 세 단계면 됩니다.
          </p>

          <div className="reveal mt-11 grid gap-4 md:grid-cols-3">
            {steps.map(({ n, Icon, title, body }) => (
              <div key={n} className="rounded-lg border border-border bg-surface-elevated p-[26px]">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-2xl font-semibold leading-none text-brand">
                    {n}
                  </span>
                  <Icon
                    className="h-[22px] w-[22px] text-text-subtle"
                    strokeWidth={1.5}
                    aria-hidden
                  />
                </div>
                <h3 className="mt-[18px] text-lg font-semibold tracking-[-0.01em]">{title}</h3>
                <p className="mt-2 text-sm leading-normal text-text-subtle break-keep">{body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Benefits ── */}
      <section className="border-t border-border">
        <div className={`${wrap} ${sec}`}>
          <Kicker no="03">무엇이 좋아지나요</Kicker>
          <h2 className={`reveal max-w-[26rem] ${secHeading}`}>
            새로운 걸 배우지 않아도, 팀 전체가 안전해집니다
          </h2>
          <div className="reveal mt-9 grid gap-4 md:grid-cols-2">
            {benefits.map(({ Icon, title, body }) => (
              <div
                key={title}
                className="flex items-start gap-4 rounded-lg border border-border bg-surface-elevated p-6"
              >
                <span className="grid h-[38px] w-[38px] shrink-0 place-items-center rounded-md bg-surface-hover text-text">
                  <Icon className="h-5 w-5" strokeWidth={1.6} aria-hidden />
                </span>
                <div>
                  <h3 className="text-md font-semibold">{title}</h3>
                  <p className="mt-1.5 text-sm leading-normal text-text-subtle break-keep">{body}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── GUI-first interactive demo ── */}
      <section id="demo" className="scroll-mt-20 border-t border-border bg-panel">
        <div
          className={`${wrap} ${sec} grid items-center gap-[clamp(1.75rem,5vw,3.75rem)] lg:grid-cols-[0.9fr_1.1fr]`}
        >
          <div className="reveal">
            <Kicker no="04">화면 미리보기</Kicker>
            <h2 className={`max-w-[20rem] ${secHeading}`}>
              눌러 보세요.
              <br />
              이게 전부입니다
            </h2>
            <p className={`mt-4 max-w-[30rem] ${lead}`}>
              환경을 전환하고, 값을 잠깐 열어 보세요. 어떤 환경에 무엇이{' '}
              <strong className="font-semibold text-text">누락</strong>됐는지 바로 표시됩니다. CLI
              없이도요.
            </p>
            <p className="mt-5 flex items-center gap-2 text-sm text-text-faint">
              <Lock className="h-[15px] w-[15px]" aria-hidden />
              실제 대시보드의 축소판입니다
            </p>
          </div>
          <div className="reveal" style={{ transitionDelay: '0.08s' }}>
            <SecretDemo />
          </div>
        </div>
      </section>

      {/* ── Developers / CLI ── */}
      <section className="border-t border-border">
        <div className={`${wrap} ${sec}`}>
          <div className="grid items-center gap-[clamp(1.75rem,5vw,3.5rem)] lg:grid-cols-[0.95fr_1.05fr]">
            <div className="reveal">
              <Kicker no="05">개발자라면</Kicker>
              <h2 className={`max-w-[22rem] ${secHeading}`}>
                터미널이 편하다면,
                <br />
                명령 한 줄이면 됩니다
              </h2>
              <p className={`mt-4 max-w-[30rem] ${lead}`}>
                대시보드 없이도 동일하게 동작합니다. 값을 올리고, 환경을 비교하고, 실행 프로세스에 바로
                주입하세요. CI 파이프라인에도 한 줄로 붙습니다.
              </p>
              <div className="mt-6">
                <ButtonLink href="/docs/cli" variant="outline">
                  CLI 문서 보기
                  <ArrowRight className="h-[15px] w-[15px]" aria-hidden />
                </ButtonLink>
              </div>
            </div>

            <div
              className="reveal overflow-hidden rounded-lg border border-border bg-surface-elevated shadow-md"
              style={{ transitionDelay: '0.08s' }}
            >
              <div className="flex items-center gap-2 border-b border-border bg-panel px-3.5 py-2.5">
                <span className="flex gap-1.5" aria-hidden>
                  <span className="h-2.5 w-2.5 rounded-full bg-border-strong" />
                  <span className="h-2.5 w-2.5 rounded-full bg-border-strong" />
                  <span className="h-2.5 w-2.5 rounded-full bg-border-strong" />
                </span>
                <span className="ml-1 font-mono text-xs text-muted">terminal · secret</span>
              </div>
              <pre className="scrollbar-thin overflow-x-auto px-[18px] py-4 font-mono text-sm leading-[1.9] text-code-text">
                <code>
                  <div className="text-text-faint"># 값을 올리고, 어디서든 주입해서 실행</div>
                  <div>
                    <span className="select-none text-muted">$ </span>secret push --env dev .env
                  </div>
                  <div className="text-text-subtle">→ 14개 값을 dev 환경에 저장했습니다</div>
                  <div>
                    <span className="select-none text-muted">$ </span>secret run --env dev -- npm
                    run dev
                  </div>
                  <div className="text-text-subtle">→ 프로세스에 값을 주입해 실행합니다</div>
                  <div>
                    <span className="select-none text-muted">$ </span>secret diff dev prod
                  </div>
                  <div className="text-danger-strong">{'  ▲ STRIPE_SECRET   (prod에 누락)'}</div>
                </code>
              </pre>
            </div>
          </div>
        </div>
      </section>

      {/* ── Final CTA ── */}
      <section className="border-t border-border">
        <div className={`${wrap} ${sec} text-center`}>
          <div className="reveal mx-auto max-w-[34rem]">
            <h2 className="mx-auto max-w-[30rem] text-display font-semibold leading-[1.1] tracking-[-0.02em] text-balance break-keep">
              흩어진 시크릿, 오늘 한곳으로 모으세요
            </h2>
            <p className={`mx-auto mt-4 ${lead}`}>
              설치는 몇 분이면 끝납니다. 자체 서버에 올리고 바로 시작하세요.
            </p>
            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <ButtonLink href="/docs" size="lg">
                무료로 시작하기
              </ButtonLink>
              <ButtonLink href="#demo" variant="outline" size="lg">
                대시보드 둘러보기
              </ButtonLink>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}

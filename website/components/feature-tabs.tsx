'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { cn } from '@/lib/cn';

/**
 * The four differentiators, consolidated into ONE interactive stage instead of
 * four scattered blocks: a vertical tablist drives a shared proof panel. Click
 * or arrow-key to switch; it auto-advances every 5s until the reader interacts
 * (then it stays put). Respects `prefers-reduced-motion` (no auto-advance, no
 * transitions). Full ARIA tabs semantics with roving tabindex.
 */

type LineKind = 'cmd' | 'cmt' | 'out' | 'add' | 'chg';

const lineClass: Record<LineKind, string> = {
  cmd: 'text-text',
  cmt: 'text-muted',
  out: 'text-brand',
  add: 'text-success',
  chg: 'text-warning',
};

type Line = { kind: LineKind; node: ReactNode };

type Tab = {
  k: string;
  title: string;
  desc: string;
  panelTitle: string;
  capStrong: string;
  capRest: string;
  lines: Line[];
};

const TABS: Tab[] = [
  {
    k: 'env diff',
    title: '환경 간 누락을 본다',
    desc: 'local엔 있고 prod엔 없는 키를 배포 전에 전용 신호로 표시한다.',
    panelTitle: 'terminal — secret diff',
    capStrong: 'secret diff',
    capRest: ' — 배포 전에 어긋남을 한눈에.',
    lines: [
      { kind: 'cmd', node: 'secret diff local prod' },
      { kind: 'add', node: '  + STRIPE_KEY      local 에만 있음' },
      { kind: 'chg', node: '  ~ DB_HOST         값이 다름' },
      { kind: 'cmt', node: '  = 12 keys 동일' },
      { kind: 'out', node: '→ prod에 STRIPE_KEY 추가하면 정합' },
    ],
  },
  {
    k: 'NAS · homelab',
    title: '홈랩에서 그냥 뜬다',
    desc: 'SQLite 파일 하나 마운트, docker compose up 한 줄. 외부 Postgres·Redis 없음.',
    panelTitle: 'terminal — docker compose',
    capStrong: 'docker compose up',
    capRest: ' — SQLite 하나, 외부 의존성 0.',
    lines: [
      { kind: 'cmd', node: 'docker compose up -d' },
      { kind: 'out', node: '✓ secret-server   ready on :8080' },
      { kind: 'cmt', node: '# 마운트: ./data/secrets.db (SQLite 한 개)' },
      { kind: 'cmt', node: '# 외부 Postgres·Redis 프로세스 없음' },
    ],
  },
  {
    k: 'worktree',
    title: '브랜치가 곧 환경',
    desc: '.secretrc와 git branch 매핑으로 worktree마다 올바른 환경이 자동으로 붙는다.',
    panelTitle: 'terminal — worktree',
    capStrong: 'worktree = 환경',
    capRest: ' — 브랜치만 바꾸면 맞는 시크릿.',
    lines: [
      { kind: 'cmt', node: '# .secretrc — branch → environment' },
      { kind: 'out', node: 'feat/*  →  dev' },
      { kind: 'out', node: 'main    →  prod' },
      { kind: 'cmd', node: 'secret run -- npm run dev' },
      { kind: 'out', node: '→ 현재 worktree(feat/x)의 dev 시크릿 주입' },
    ],
  },
  {
    k: 'github actions',
    title: 'Secret 등록이 사라진다',
    desc: 'load-action 한 줄이 지정한 environment를 step env로 주입한다.',
    panelTitle: 'deploy.yml — github actions',
    capStrong: 'load-action',
    capRest: ' — 저장소마다 Secret 재등록 없음.',
    lines: [
      { kind: 'cmt', node: '# .github/workflows/deploy.yml' },
      { kind: 'cmd', node: 'uses: comax/load-action@v1' },
      { kind: 'cmd', node: '  with: { environment: prod }' },
      { kind: 'out', node: '→ prod 시크릿이 step env로 주입됨' },
    ],
  },
];

const ADVANCE_MS = 5000;

export function FeatureTabs() {
  const [active, setActive] = useState(0);
  const [auto, setAuto] = useState(true);
  const [paused, setPaused] = useState(false);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  // Honor reduced-motion: no auto-advance at all.
  useEffect(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    if (mq.matches) setAuto(false);
  }, []);

  // Auto-advance timer, re-armed whenever the active tab changes.
  useEffect(() => {
    if (!auto || paused) return;
    const id = window.setTimeout(() => setActive((a) => (a + 1) % TABS.length), ADVANCE_MS);
    return () => window.clearTimeout(id);
  }, [auto, paused, active]);

  function stopAuto() {
    setAuto(false);
  }

  function select(i: number, focus: boolean) {
    setActive(i);
    if (focus) tabRefs.current[i]?.focus();
  }

  function onKeyDown(e: React.KeyboardEvent, i: number) {
    let next: number | null = null;
    if (e.key === 'ArrowDown' || e.key === 'ArrowRight') next = (i + 1) % TABS.length;
    else if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') next = (i - 1 + TABS.length) % TABS.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = TABS.length - 1;
    if (next !== null) {
      e.preventDefault();
      stopAuto();
      select(next, true);
    }
  }

  const showBar = auto && !paused;
  const current = TABS[active];

  return (
    <div
      className="grid items-start gap-7 lg:grid-cols-[0.82fr_1.18fr]"
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocusCapture={() => setPaused(true)}
      onBlurCapture={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node)) setPaused(false);
      }}
    >
      <div
        role="tablist"
        aria-label="주요 기능"
        aria-orientation="vertical"
        className="flex gap-1 overflow-x-auto lg:flex-col lg:overflow-visible"
      >
        {TABS.map((tab, i) => {
          const on = i === active;
          return (
            <button
              key={tab.k}
              ref={(el) => {
                tabRefs.current[i] = el;
              }}
              role="tab"
              id={`ft-tab-${i}`}
              aria-selected={on}
              aria-controls={`ft-panel-${i}`}
              tabIndex={on ? 0 : -1}
              onClick={() => {
                stopAuto();
                select(i, false);
              }}
              onKeyDown={(e) => onKeyDown(e, i)}
              className={cn(
                'relative min-w-[220px] overflow-hidden rounded-xl border border-transparent px-[18px] py-4 text-left transition-colors duration-fast lg:min-w-0',
                on
                  ? 'bg-brand-soft text-text'
                  : 'text-text-subtle hover:bg-surface-hover hover:text-text',
              )}
            >
              <span
                className={cn('block font-mono text-xs', on ? 'text-brand-strong' : 'text-muted')}
              >
                {tab.k}
              </span>
              <span className="mt-1.5 block text-[1.02rem] font-semibold tracking-[-0.01em]">
                {tab.title}
              </span>
              {on && (
                <span className="mt-2 block text-sm text-text-subtle motion-safe:animate-none">
                  {tab.desc}
                </span>
              )}
              {on && showBar && (
                <span
                  key={active}
                  aria-hidden
                  className="absolute inset-x-0 bottom-0 h-0.5 origin-left bg-brand"
                  style={{
                    animationName: 'ft-fill',
                    animationDuration: `${ADVANCE_MS}ms`,
                    animationTimingFunction: 'linear',
                    animationFillMode: 'forwards',
                  }}
                />
              )}
            </button>
          );
        })}
      </div>

      <div className="overflow-hidden rounded-xl border border-border bg-surface-elevated shadow-md">
        <div className="flex items-center gap-2 border-b border-border bg-panel px-3.5 py-2.5">
          <span className="h-2.5 w-2.5 rounded-full bg-border-strong" aria-hidden />
          <span className="font-mono text-xs text-muted">{current?.panelTitle}</span>
        </div>
        <div className="min-h-[210px] px-[18px] py-[18px]">
          {TABS.map((tab, i) => (
            <div
              key={tab.k}
              role="tabpanel"
              id={`ft-panel-${i}`}
              aria-labelledby={`ft-tab-${i}`}
              tabIndex={0}
              hidden={i !== active}
              className="font-mono text-[0.82rem] leading-[1.9] motion-safe:animate-[fade_0.32s_ease]"
            >
              {tab.lines.map((line, j) => (
                <div key={j} className={lineClass[line.kind]}>
                  {line.kind === 'cmd' && <span className="select-none text-muted">$ </span>}
                  {line.node}
                </div>
              ))}
            </div>
          ))}
        </div>
        <div className="border-t border-border px-[18px] py-3 text-sm text-text-subtle">
          <b className="font-semibold text-text">{current?.capStrong}</b>
          {current?.capRest}
        </div>
      </div>
    </div>
  );
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Command } from 'cmdk';
import { useNavigate } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { listProjects, prefetchEnvs, prefetchSecrets, queryKeys } from '../lib/queries';
import { useRecent, type RecentEntry } from '../lib/recent';
import type { Environment, ResolvedSecret } from '../lib/types';
import { Alert } from './Alert';
import { useThemeContext } from './ThemeRoot';

/**
 * Global ⌘K command palette.
 *
 * Hosted once by AppShell so every authenticated route shares the same
 * overlay. Groups (in empty-input order): recent → navigate → projects
 * → theme. When the operator types, cmdk filters across navigate +
 * projects + envs + keys + theme; recent is hidden once a query is in
 * play because it is positional, not a match.
 *
 * Lazy prefetch on open: ⌘K trips one shot that warms the env list for
 * every project plus the secret list for the active (project, env) pair
 * the AppShell handed us. We never fan out N×M secrets — cross-env key
 * search is a server-side index (backlog 8). Two distinct failure modes
 * surface in the same banner with different copy and retry targets:
 * (1) the projects fetch itself fails — banner offers refetch of the
 *     useQuery; navigate + theme keep working from static groups.
 * (2) prefetch (envs/secrets) fails after projects loaded — banner
 *     offers a prefetch re-attempt; the projects group still renders.
 * Both keep the operator inside the palette instead of forcing a reopen.
 *
 * cmdk's `value` attribute is overridden per row so a single token like
 * "stripe" finds keys across projects, while "staging stripe" narrows
 * to staging. That keyword expansion is doing the heavy lifting — no
 * client-side filter logic in this component.
 */

export interface ActivePaletteContext {
  project: string;
  env: string;
}

interface CommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  activeContext?: ActivePaletteContext | null;
}

export function CommandPalette({ open, onOpenChange, activeContext }: CommandPaletteProps) {
  // Defer router-dependent hooks (useNavigate) to the inner body so that
  // unit tests can mount AppShell without mocking the full router context.
  // The palette is closed by default; opening it lifts the body into the
  // tree where useNavigate runs against the live RouterProvider.
  if (!open) return null;
  return <PaletteBody onOpenChange={onOpenChange} activeContext={activeContext ?? null} />;
}

interface PaletteBodyProps {
  onOpenChange: (open: boolean) => void;
  activeContext: ActivePaletteContext | null;
}

type PrefetchStatus = 'pending' | 'ready' | 'error';

// Operator intent that should resurface the 테마 group during search.
// Exact-token match: substring includes() dragged unrelated English words
// through (e.g. "highlight" contains "light"), surfacing the theme group on
// queries with no theme intent. Token-split keeps Korean particles ("라이트
// 모드") friendly because each keyword is its own bare token in practice,
// while blocking English compounds that merely embed the substring.
const THEME_KEYWORDS = ['theme', '테마', 'light', 'dark', '라이트', '다크'] as const;

function matchesThemeIntent(queryLower: string): boolean {
  const tokens = queryLower.split(/\s+/).filter(Boolean);
  return tokens.some((token) => THEME_KEYWORDS.some((keyword) => keyword === token));
}

// Banner retry copy. The visible label names the *result* (refresh / re-sync)
// so the operator can predict what fires before clicking; the banner text
// above already names what failed. Keyed by failure mode for one source of
// truth instead of inline ternaries across visible + aria.
const RETRY_LABEL = {
  projects: '목록 새로고침',
  prefetch: '동기화 다시',
} as const;

function PaletteBody({ onOpenChange, activeContext }: PaletteBodyProps) {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const themeCtx = useThemeContext();
  const recent = useRecent();
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<PrefetchStatus>('pending');
  // Bumped by retry / activeContext change so the prefetch effect re-runs
  // without us having to coordinate AbortControllers manually.
  const [prefetchTick, setPrefetchTick] = useState(0);
  // Brief inline confirmation when a failed prefetch recovers. The banner
  // disappearing was the only signal before — too quiet for an error→success
  // transition the operator just triggered. 2s footer text with aria-live so
  // SR users hear it. No motion: text toggle only, on system per DESIGN.md.
  const [showRecovered, setShowRecovered] = useState(false);
  const hadErrorRef = useRef(false);

  const projectsQuery = useQuery({
    queryKey: queryKeys.projects(),
    queryFn: ({ signal }) => listProjects(signal),
  });

  const close = useCallback(() => onOpenChange(false), [onOpenChange]);
  // projects fetch is its own failure surface. Reading isError directly
  // (instead of folding into the status union) keeps the prefetch
  // lifecycle one thing and the upstream fetch another — retry knows
  // which to re-run, and the banner copy can name the actual failure.
  const projectsErrored = projectsQuery.isError;

  useEffect(() => {
    const projects = projectsQuery.data;
    if (!projects) return;

    let cancelled = false;
    setStatus('pending');

    Promise.all([
      prefetchEnvs(qc, projects),
      activeContext ? prefetchSecrets(qc, activeContext.project, activeContext.env) : null,
    ])
      .then(() => {
        if (!cancelled) setStatus('ready');
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        // English system log per the project's grep policy. The operator
        // sees the Korean banner; the engineer sees this in devtools.
        console.error('command palette prefetch failed', err);
        setStatus('error');
      });

    return () => {
      cancelled = true;
    };
  }, [projectsQuery.data, activeContext, qc, prefetchTick]);

  function retry() {
    if (projectsErrored) {
      void projectsQuery.refetch();
      return;
    }
    setPrefetchTick((t) => t + 1);
  }

  // Watch the error→ready transition across both failure modes and surface
  // a 2s "동기화 완료" in the footer. We use a ref so the effect only fires
  // on transitions (not on the steady ready state at first open). The
  // success message clears itself; no follow-up clicks needed.
  useEffect(() => {
    const hasError = projectsErrored || status === 'error';
    if (hasError) {
      hadErrorRef.current = true;
      return;
    }
    if (!hadErrorRef.current || status !== 'ready') return;
    hadErrorRef.current = false;
    setShowRecovered(true);
    const t = setTimeout(() => setShowRecovered(false), 2000);
    return () => clearTimeout(t);
  }, [projectsErrored, status]);

  function run(fn: () => void | Promise<void>) {
    close();
    // Microtask so the palette unmount completes before navigation.
    queueMicrotask(() => void fn());
  }

  // Memoize so a stable reference is fed into the env-items useMemo
  // below. Without this, every render would mint a fresh [] and the
  // memo would re-run (and trip exhaustive-deps).
  const projects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const trimmedQuery = query.trim();

  const envItems = useMemo(() => {
    if (status !== 'ready') return [];
    const out: { project: string; env: string }[] = [];
    for (const p of projects) {
      const envs = qc.getQueryData<Environment[]>(queryKeys.envs(p.name)) ?? [];
      for (const e of envs) out.push({ project: p.name, env: e.name });
    }
    return out;
  }, [status, projects, qc]);

  const keyItems = useMemo(() => {
    if (status !== 'ready' || !activeContext) return [];
    const secrets =
      qc.getQueryData<ResolvedSecret[]>(
        queryKeys.secrets(activeContext.project, activeContext.env),
      ) ?? [];
    return secrets.map((s) => ({
      project: activeContext.project,
      env: activeContext.env,
      key: s.key,
    }));
  }, [status, activeContext, qc]);

  const recentVisible = trimmedQuery === '' && recent.length > 0;
  // env & key groups need an actual query to be useful; rendering them
  // empty under "이동" is noisy. cmdk's own filter respects this.
  const envKeyVisible = trimmedQuery.length > 0;
  // Theme commands are a category-specific intent; surfacing them under
  // every unrelated query (e.g. "stripe") drops the search SNR. Show the
  // group only when the catalog is the default view, or when the operator
  // explicitly reaches for a theme keyword.
  const themeVisible = trimmedQuery === '' || matchesThemeIntent(trimmedQuery.toLowerCase());

  function goEnv(project: string, env: string) {
    return () =>
      run(() =>
        navigate({
          to: '/projects/$project/envs/$env',
          params: { project, env },
        }),
      );
  }

  function goKey(project: string, env: string, key: string) {
    return () =>
      run(() =>
        navigate({
          to: '/projects/$project/envs/$env',
          params: { project, env },
          hash: key,
        }),
      );
  }

  return (
    <>
      <div className="cmdk-overlay" onClick={close} aria-hidden="true" />
      <Command
        label="명령 팔레트"
        className="cmdk-shell"
        loop
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            e.preventDefault();
            close();
          }
        }}
      >
        <div className="cmdk-input-row">
          <CmdSearchIcon />
          <Command.Input
            autoFocus
            value={query}
            onValueChange={setQuery}
            placeholder="프로젝트, 환경, 키 검색"
            className="cmdk-input"
          />
          <kbd className="cmdk-input-kbd" aria-hidden="true">
            esc
          </kbd>
        </div>

        {projectsErrored || status === 'error' ? (
          <div className="cmdk-banner-band">
            <Alert
              variant="page"
              message={
                projectsErrored ? '프로젝트 목록을 불러오지 못했습니다' : '환경·키 동기화 실패'
              }
            >
              <button type="button" className="alert-page-action" onClick={retry}>
                {projectsErrored ? RETRY_LABEL.projects : RETRY_LABEL.prefetch}
              </button>
            </Alert>
          </div>
        ) : null}

        <Command.List className="cmdk-list">
          <Command.Empty className="cmdk-empty">
            <p className="cmdk-empty-title">일치하는 항목이 없습니다.</p>
            {/* 90초 진입점 보강: 검색이 빗나가도 운영자가 막다른 길에
                서지 않도록 감사 로그로 한 번에 빠진다. cmdk filter는
                value 기반이라 plain <button>은 자동 회피된다. */}
            <button
              type="button"
              className="cmdk-empty-action"
              onClick={() => run(() => navigate({ to: '/audit', search: {} }))}
            >
              감사 로그에서 검색
            </button>
          </Command.Empty>

          {recentVisible ? (
            <Command.Group heading="최근" className="cmdk-group">
              {recent.map((entry) => (
                <RecentItem key={recentItemKey(entry)} entry={entry} onSelect={(fn) => run(fn)} />
              ))}
            </Command.Group>
          ) : null}

          <Command.Group heading="이동" className="cmdk-group">
            <CmdItem
              value="이동 프로젝트 목록 홈"
              onSelect={() => run(() => navigate({ to: '/' }))}
            >
              <CmdIcon>
                <IconProjects />
              </CmdIcon>
              <span>프로젝트 목록</span>
              <span className="cmdk-item-meta">페이지 · 홈</span>
            </CmdItem>
            <CmdItem
              value="이동 감사 로그 audit"
              onSelect={() => run(() => navigate({ to: '/audit', search: {} }))}
            >
              <CmdIcon>
                <IconAudit />
              </CmdIcon>
              <span>감사 로그</span>
              <span className="cmdk-item-meta">페이지 · 감사</span>
            </CmdItem>
          </Command.Group>

          {projects.length > 0 ? (
            <Command.Group heading="프로젝트" className="cmdk-group">
              {projects.map((p) => (
                <CmdItem
                  key={p.id}
                  value={`project ${p.name}`}
                  onSelect={() =>
                    run(() =>
                      navigate({
                        to: '/projects/$project',
                        params: { project: p.name },
                      }),
                    )
                  }
                >
                  <CmdIcon>
                    <IconFolder />
                  </CmdIcon>
                  <span>{p.name}</span>
                </CmdItem>
              ))}
            </Command.Group>
          ) : null}

          {envKeyVisible && envItems.length > 0 ? (
            <Command.Group heading="환경" className="cmdk-group">
              {envItems.map((item) => (
                <CmdItem
                  key={`${item.project}/${item.env}`}
                  value={`env ${item.project} ${item.env}`}
                  onSelect={goEnv(item.project, item.env)}
                >
                  <CmdIcon>
                    <IconEnv />
                  </CmdIcon>
                  <span>{item.env}</span>
                  <span className="cmdk-item-meta">환경 · {item.project}</span>
                </CmdItem>
              ))}
            </Command.Group>
          ) : null}

          {envKeyVisible && keyItems.length > 0 ? (
            <Command.Group
              heading={
                activeContext ? (
                  <>
                    키{' '}
                    <span className="cmdk-group-heading-scope">
                      · {activeContext.project}/{activeContext.env}
                    </span>
                  </>
                ) : (
                  '키'
                )
              }
              className="cmdk-group"
            >
              {keyItems.map((item) => (
                <CmdItem
                  key={`${item.project}/${item.env}/${item.key}`}
                  value={`key ${item.project} ${item.env} ${item.key}`}
                  onSelect={goKey(item.project, item.env, item.key)}
                >
                  <CmdIcon>
                    <IconKey />
                  </CmdIcon>
                  <span className="cmdk-item-key">{item.key}</span>
                  <span className="cmdk-item-meta">
                    키 · {item.project}/{item.env}
                  </span>
                </CmdItem>
              ))}
            </Command.Group>
          ) : null}

          {themeVisible ? (
            <Command.Group heading="테마" className="cmdk-group">
              <CmdItem
                value="theme system 자동"
                onSelect={() => run(() => themeCtx.setPreference('system'))}
              >
                <CmdIcon>
                  <IconSystem />
                </CmdIcon>
                <span>테마: 시스템 설정 따름</span>
                {themeCtx.preference === 'system' ? (
                  <span className="cmdk-item-meta">현재</span>
                ) : null}
              </CmdItem>
              <CmdItem
                value="theme light 라이트"
                onSelect={() => run(() => themeCtx.setPreference('light'))}
              >
                <CmdIcon>
                  <IconLight />
                </CmdIcon>
                <span>테마: 라이트</span>
                {themeCtx.preference === 'light' ? (
                  <span className="cmdk-item-meta">현재</span>
                ) : null}
              </CmdItem>
              <CmdItem
                value="theme dark 다크"
                onSelect={() => run(() => themeCtx.setPreference('dark'))}
              >
                <CmdIcon>
                  <IconDark />
                </CmdIcon>
                <span>테마: 다크</span>
                {themeCtx.preference === 'dark' ? (
                  <span className="cmdk-item-meta">현재</span>
                ) : null}
              </CmdItem>
            </Command.Group>
          ) : null}
        </Command.List>

        <div className="cmdk-footer">
          <span>
            <kbd>↑</kbd>
            <kbd>↓</kbd> 이동 · <kbd>⏎</kbd> 실행 · <kbd>esc</kbd> 닫기
          </span>
          {showRecovered ? (
            <span className="cmdk-footer-recovered" aria-live="polite">
              동기화 완료
            </span>
          ) : status === 'pending' && !projectsErrored ? (
            <span>동기화 중…</span>
          ) : null}
        </div>
      </Command>
    </>
  );
}

interface RecentItemProps {
  entry: RecentEntry;
  onSelect: (fn: () => void | Promise<void>) => void;
}

function RecentItem({ entry, onSelect }: RecentItemProps) {
  const navigate = useNavigate();
  const label = entry.env ? entry.env : entry.project;
  // Type-first to match the 4-format unification across the palette.
  // Recent project rows show project name as the label, so no further
  // context is needed in meta — type alone keeps the column scannable.
  const meta = entry.env ? `환경 · ${entry.project}` : '프로젝트';
  const value = entry.env
    ? `recent env ${entry.project} ${entry.env}`
    : `recent project ${entry.project}`;
  return (
    <CmdItem
      value={value}
      onSelect={() =>
        onSelect(() =>
          entry.env
            ? navigate({
                to: '/projects/$project/envs/$env',
                params: { project: entry.project, env: entry.env },
              })
            : navigate({
                to: '/projects/$project',
                params: { project: entry.project },
              }),
        )
      }
    >
      <CmdIcon>{entry.env ? <IconEnv /> : <IconFolder />}</CmdIcon>
      <span>{label}</span>
      <span className="cmdk-item-meta">{meta}</span>
    </CmdItem>
  );
}

function recentItemKey(entry: RecentEntry): string {
  return entry.env ? `${entry.project}/${entry.env}` : entry.project;
}

/* ── Public hook: ⌘K toggles open state app-wide ── */

interface UseCommandPaletteResult {
  open: boolean;
  setOpen: (next: boolean) => void;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useCommandPalette(): UseCommandPaletteResult {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // ⌘K (macOS) or Ctrl+K (others). Avoid intercepting when typing
      // in form fields with modifier-free input.
      const isToggle = (e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K');
      if (isToggle) {
        e.preventDefault();
        setOpen((v) => !v);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return useMemo(() => ({ open, setOpen }), [open]);
}

/* ── Internal helpers ── */

interface CmdItemProps {
  value?: string;
  onSelect: () => void;
  children: React.ReactNode;
}

function CmdItem({ value, onSelect, children }: CmdItemProps) {
  return (
    <Command.Item className="cmdk-item" value={value} onSelect={onSelect}>
      {children}
    </Command.Item>
  );
}

function CmdIcon({ children }: { children: React.ReactNode }) {
  return <span className="cmdk-item-icon">{children}</span>;
}

function CmdSearchIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="11" cy="11" r="6.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="m16 16 4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function IconProjects() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M3 6.5A2.5 2.5 0 0 1 5.5 4H10l2 2h6.5A2.5 2.5 0 0 1 21 8.5v8A2.5 2.5 0 0 1 18.5 19h-13A2.5 2.5 0 0 1 3 16.5v-10Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function IconFolder() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function IconAudit() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M5 4h10l4 4v12a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path
        d="M14 4v5h5M8 13h7M8 17h5"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function IconEnv() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M4 7.5 12 4l8 3.5v9L12 20l-8-3.5v-9Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path d="M4 7.5 12 11l8-3.5M12 11v9" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}

function IconKey() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="8" cy="14" r="3.5" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="m11 12 9-9M16 7l2 2M14 9l2 2"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function IconSystem() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="3" y="5" width="18" height="12" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M9 21h6M12 17v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function IconLight() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4 7 17M17 7l1.4-1.4"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function IconDark() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M20 14.5A8 8 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

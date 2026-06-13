import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Link, useRouter, useRouterState } from '@tanstack/react-router';

import { ApiError } from '../lib/api';
import { logout } from '../lib/auth';
import { pushRecent } from '../lib/recent';
import { Alert } from './Alert';
import { CommandPalette, useCommandPalette, type ActivePaletteContext } from './CommandPalette';
import { ThemeToggle } from './ThemeToggle';

/**
 * Three-column desktop shell.
 *
 *   ┌────────────┬──────────────────────────────────────────────┐
 *   │            │  crumb · search (locked center) · theme · ⎋  │
 *   │  sidebar   ├──────────────────────────────────────────────┤
 *   │            │                                              │
 *   │            │              main (page owns its header)     │
 *   │            │                                              │
 *   └────────────┴──────────────────────────────────────────────┘
 *
 * The header is a 3-column CSS grid so the search bar lives at a fixed
 * position regardless of crumb depth — earlier flex layout let the
 * search slide right as breadcrumbs grew, which broke the muscle memory
 * for ⌘K. The center column is sized off --shell-header-search-width.
 *
 * Page actions DO NOT live in the header. Each page renders its own
 * .page-head (title left, buttons right) so primary actions land in the
 * section they affect, matching GitHub and Linear. AppShell no longer
 * accepts an `actions` prop.
 *
 * Accessibility landmarks: skip link points at <main id="main">, sidebar
 * is wrapped in <nav aria-label="주 메뉴">, header search trigger carries
 * aria-keyshortcuts="Meta+K Control+K".
 */

export interface Crumb {
  label: string;
  to?: string;
  params?: Record<string, string>;
}

export type ActiveSection = 'projects' | 'audit' | 'sessions';

interface AppShellProps {
  active?: ActiveSection;
  crumbs?: Crumb[];
  children: ReactNode;
}

export function AppShell({ active, crumbs, children }: AppShellProps) {
  const router = useRouter();
  const { open: paletteOpen, setOpen: setPaletteOpen } = useCommandPalette();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Active (project, env) pair, read from the deepest matched route's
  // params. The palette uses it to scope its secrets prefetch to only
  // the env currently on screen (instead of fanning out N×M requests).
  // Cast through unknown because route params is a discriminated union
  // across all registered routes; we only care about two optional keys.
  const matchParams = useRouterState({
    select: (s) => {
      const last = s.matches[s.matches.length - 1];
      return (last?.params ?? {}) as { project?: string; env?: string };
    },
  });
  const activePaletteContext = useMemo<ActivePaletteContext | null>(() => {
    if (matchParams.project && matchParams.env) {
      return { project: matchParams.project, env: matchParams.env };
    }
    return null;
  }, [matchParams.project, matchParams.env]);

  // Track recent visits for the palette's empty-input state. The router
  // subscription fires after route resolution, which is the moment the
  // operator has committed to a destination. We read params from the
  // freshly-resolved match list (the event's toLocation only carries the
  // ParsedLocation, not match-level params). Dedupe and FIFO-5 bounding
  // live inside the recent store; this side-effect just feeds it.
  useEffect(() => {
    const unsubscribe = router.subscribe('onResolved', () => {
      const matches = router.state.matches;
      const last = matches[matches.length - 1];
      const params = (last?.params ?? {}) as { project?: string; env?: string };
      if (!params.project) return;
      pushRecent({ project: params.project, env: params.env });
    });
    return unsubscribe;
  }, [router]);

  async function onLogout() {
    setError(null);
    setBusy(true);
    try {
      await logout();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Logout failed.');
      setBusy(false);
      return;
    }
    setBusy(false);
    await router.navigate({ to: '/login', replace: true });
  }

  return (
    <>
      <a href="#main" className="skip-link">
        본문으로 건너뛰기
      </a>
      <div className="shell">
        <nav className="shell-nav" aria-label="주 메뉴">
          <Link to="/" className="nav-brand" aria-label="Comax Secrets 홈">
            <span className="nav-brand-mark" aria-hidden="true">
              C
            </span>
            <span className="flex flex-col leading-tight">
              <span className="nav-brand-name">Comax Secrets</span>
              <span className="nav-brand-tag">self-hosted</span>
            </span>
          </Link>

          <div className="nav-section" role="list">
            <div className="nav-section-label">운영</div>
            <SidebarLink to="/" active={active === 'projects'}>
              <IconProjects />
              <span>프로젝트</span>
            </SidebarLink>
            <SidebarLink to="/audit" active={active === 'audit'}>
              <IconAudit />
              <span>감사 로그</span>
            </SidebarLink>
          </div>

          <div className="nav-section" role="list">
            <div className="nav-section-label">설정</div>
            <SidebarLink to="/settings/sessions" active={active === 'sessions'}>
              <IconSessions />
              <span>세션</span>
            </SidebarLink>
          </div>

          <div className="nav-footer">
            <button
              type="button"
              className="nav-link w-full bg-transparent cursor-pointer"
              onClick={onLogout}
              disabled={busy}
              aria-label="로그아웃"
            >
              <span className="nav-link-icon">
                <IconLogout />
              </span>
              <span>{busy ? '로그아웃 중…' : '로그아웃'}</span>
            </button>
          </div>
        </nav>

        <header className="shell-header" role="banner">
          <div className="shell-header-left">
            {crumbs && crumbs.length > 0 ? (
              <nav aria-label="현재 위치" className="header-crumb">
                {crumbs.map((crumb, idx) => {
                  const isLast = idx === crumbs.length - 1;
                  return (
                    <span key={`${crumb.label}-${idx}`} className="header-crumb-item">
                      {idx > 0 ? (
                        <span className="header-crumb-sep" aria-hidden="true">
                          /
                        </span>
                      ) : null}
                      {crumb.to && !isLast ? (
                        <Link to={crumb.to} params={crumb.params}>
                          {crumb.label}
                        </Link>
                      ) : (
                        <span aria-current={isLast ? 'page' : undefined}>{crumb.label}</span>
                      )}
                    </span>
                  );
                })}
              </nav>
            ) : null}
          </div>

          <div className="shell-header-center">
            <button
              type="button"
              className="header-search-trigger"
              onClick={() => setPaletteOpen(true)}
              aria-label="명령 팔레트 열기"
              aria-keyshortcuts="Meta+K Control+K"
            >
              <IconSearch />
              <span>프로젝트, 환경, 키 검색</span>
              <kbd>⌘K</kbd>
            </button>
          </div>

          <div className="shell-header-right">
            <ThemeToggle />
          </div>
        </header>

        <main id="main" tabIndex={-1} className="shell-main outline-none">
          {error ? (
            <div className="mb-4">
              <Alert variant="page" message={error} />
            </div>
          ) : null}
          {children}
        </main>
      </div>

      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        activeContext={activePaletteContext}
      />
    </>
  );
}

interface SidebarLinkProps {
  to: string;
  active: boolean;
  children: ReactNode;
}

function SidebarLink({ to, active, children }: SidebarLinkProps) {
  return (
    <Link to={to} className="nav-link" aria-current={active ? 'page' : undefined} role="listitem">
      {children}
    </Link>
  );
}

/* Inline icons (no icon library, per design brief): 1.5px stroke, square box. */

function IconProjects() {
  return (
    <span className="nav-link-icon" aria-hidden="true">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
        <path
          d="M3 6.5A2.5 2.5 0 0 1 5.5 4H10l2 2h6.5A2.5 2.5 0 0 1 21 8.5v8A2.5 2.5 0 0 1 18.5 19h-13A2.5 2.5 0 0 1 3 16.5v-10Z"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
      </svg>
    </span>
  );
}

function IconAudit() {
  return (
    <span className="nav-link-icon" aria-hidden="true">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
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
    </span>
  );
}

function IconSessions() {
  return (
    <span className="nav-link-icon" aria-hidden="true">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
        <rect
          x="3.5"
          y="5"
          width="17"
          height="11"
          rx="2"
          stroke="currentColor"
          strokeWidth="1.5"
        />
        <path d="M8 20h8M12 16v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    </span>
  );
}

function IconSearch() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="11" cy="11" r="6.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="m16 16 4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function IconLogout() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M14 4h4a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-4M10 8l-4 4 4 4M6 12h11"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

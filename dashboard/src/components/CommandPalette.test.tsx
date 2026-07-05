import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { CommandPalette } from './CommandPalette';
import { clearRecent, pushRecent } from '../lib/recent';
import { renderWithProviders } from '../test/renderWithProviders';

const navigateMock = vi.fn();

vi.mock('@tanstack/react-router', async () => {
  const actual =
    await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router');
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

const fetchMock = vi.fn();

beforeEach(() => {
  navigateMock.mockReset();
  fetchMock.mockReset();
  clearRecent();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function envelope(data: unknown, status = 200) {
  return new Response(JSON.stringify({ ok: status < 400, data }), { status });
}

function rejected(message = 'boom') {
  return Promise.reject(new TypeError(message));
}

const projects = [
  { id: 1, name: 'core', created_at: '2026-01-01T00:00:00Z', env_count: 2 },
  { id: 2, name: 'billing', created_at: '2026-01-02T00:00:00Z', env_count: 1 },
];
const coreEnvs = [
  { id: 11, project_id: 1, name: 'staging', created_at: '2026-01-03T00:00:00Z' },
  { id: 12, project_id: 1, name: 'prod', created_at: '2026-01-04T00:00:00Z' },
];
const billingEnvs = [{ id: 21, project_id: 2, name: 'prod', created_at: '2026-01-05T00:00:00Z' }];
const stagingSecrets = [
  {
    secret_id: 100,
    key: 'STRIPE_API_KEY',
    value: 'sk-staging',
    version: 1,
    updated_at: '2026-01-06T00:00:00Z',
  },
  {
    secret_id: 101,
    key: 'DATABASE_URL',
    value: 'postgres://x',
    version: 1,
    updated_at: '2026-01-07T00:00:00Z',
  },
];

function mockHappyPath() {
  // The palette fires GET /projects, then prefetchEnvs runs in parallel
  // for every project. With no active context, secrets are not fetched.
  // Order of env requests is preserved by Promise.all → order of input
  // array. fetchMock returns by call order so we mirror that order:
  // 1. /projects
  // 2. /projects/core/envs
  // 3. /projects/billing/envs
  fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString();
    if (url.endsWith('/api/v1/projects')) return envelope(projects);
    if (url.endsWith('/api/v1/projects/core/envs')) return envelope(coreEnvs);
    if (url.endsWith('/api/v1/projects/billing/envs')) return envelope(billingEnvs);
    if (url.endsWith('/api/v1/projects/core/envs/staging/secrets')) return envelope(stagingSecrets);
    throw new Error(`unexpected fetch ${url}`);
  });
}

describe('CommandPalette', () => {
  it('renders empty-input layout: navigate + projects + theme, no recent, no env/key', async () => {
    mockHappyPath();
    renderWithProviders(<CommandPalette open onOpenChange={() => {}} activeContext={null} />);

    await screen.findByRole('group', { name: '프로젝트' });

    expect(screen.getByRole('group', { name: '이동' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: '테마' })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: '최근' })).not.toBeInTheDocument();
    // env/key groups are hidden until the operator types
    expect(screen.queryByRole('group', { name: '환경' })).not.toBeInTheDocument();
    expect(screen.queryByRole('group', { name: '키' })).not.toBeInTheDocument();
    // Placeholder reflects the locked scope
    expect(screen.getByPlaceholderText('프로젝트, 환경, 키 검색')).toBeInTheDocument();
  });

  it('shows the 최근 group after pushRecent and hides it once the operator types', async () => {
    mockHappyPath();
    pushRecent({ project: 'core', env: 'staging' });

    const user = userEvent.setup();
    renderWithProviders(<CommandPalette open onOpenChange={() => {}} activeContext={null} />);

    const recentGroup = await screen.findByRole('group', { name: '최근' });
    // Recent renders the env label with the type-first meta "환경 · {project}".
    expect(recentGroup).toHaveTextContent('staging');
    expect(recentGroup).toHaveTextContent('환경 · core');

    // Typing hides 최근 (positional, not match-relevant)
    await user.type(screen.getByPlaceholderText('프로젝트, 환경, 키 검색'), 'core');
    expect(screen.queryByRole('group', { name: '최근' })).not.toBeInTheDocument();
  });

  it('offers a 감사 로그 recovery action when the search misses', async () => {
    mockHappyPath();
    const user = userEvent.setup();
    renderWithProviders(<CommandPalette open onOpenChange={() => {}} activeContext={null} />);

    // Wait until projects load so cmdk's filter has real items to miss.
    await screen.findByRole('group', { name: '프로젝트' });

    // Type a query nothing matches.
    await user.type(screen.getByPlaceholderText('프로젝트, 환경, 키 검색'), 'zzzzznomatch');

    const recoverBtn = await screen.findByRole('button', {
      name: '감사 로그에서 검색',
    });
    await user.click(recoverBtn);

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith({ to: '/audit', search: {} });
    });
  });

  it('hides 테마 on unrelated queries and resurfaces it on a theme keyword', async () => {
    mockHappyPath();
    const user = userEvent.setup();
    renderWithProviders(<CommandPalette open onOpenChange={() => {}} activeContext={null} />);

    // Default catalog: 테마 group is visible.
    await screen.findByRole('group', { name: '테마' });

    const input = screen.getByPlaceholderText('프로젝트, 환경, 키 검색');

    // Unrelated query drops the group entirely — not just cmdk-filtered to
    // zero items — so the result panel's SNR stays high.
    await user.type(input, 'stripe');
    expect(screen.queryByRole('group', { name: '테마' })).not.toBeInTheDocument();

    // Token-split (not substring) matching: English words that merely embed
    // a theme keyword (e.g. "highlight" contains "light") must not surface
    // the group. Exact-token match keeps the heuristic honest.
    await user.clear(input);
    await user.type(input, 'highlight');
    expect(screen.queryByRole('group', { name: '테마' })).not.toBeInTheDocument();

    // Explicit theme intent (Korean keyword works just like the English one).
    await user.clear(input);
    await user.type(input, '라이트');
    await screen.findByRole('group', { name: '테마' });
  });

  it('reveals 환경 + 키 groups on input and matches by cmdk value keywords', async () => {
    mockHappyPath();
    const user = userEvent.setup();
    renderWithProviders(
      <CommandPalette
        open
        onOpenChange={() => {}}
        activeContext={{ project: 'core', env: 'staging' }}
      />,
    );

    // Wait until the projects-then-envs prefetch chain has settled so the
    // 환경/키 groups have their materialized data ready to show.
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining('/api/v1/projects/core/envs/staging/secrets'),
        expect.anything(),
      );
    });

    await user.type(screen.getByPlaceholderText('프로젝트, 환경, 키 검색'), 'stripe');

    // The 키 group surfaces STRIPE_API_KEY by matching "stripe" against
    // value="key core staging STRIPE_API_KEY". Database row stays out.
    // Heading now embeds the activeContext scope ("키 · core/staging") so
    // the operator can read what's loaded without trusting the placeholder.
    const keyGroup = await screen.findByRole('group', { name: /^키 · core\/staging$/ });
    expect(keyGroup).toHaveTextContent('STRIPE_API_KEY');
    expect(keyGroup).not.toHaveTextContent('DATABASE_URL');
  });

  it('renders the 프로젝트 실패 banner when the projects query errors and refetches on click', async () => {
    // First /projects call rejects; subsequent calls succeed so retry can
    // resolve. Env prefetches only fire after a successful projects load.
    let projectAttempts = 0;
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString();
      if (url.endsWith('/api/v1/projects')) {
        projectAttempts += 1;
        if (projectAttempts === 1) return rejected('network');
        return envelope(projects);
      }
      if (url.endsWith('/api/v1/projects/core/envs')) return envelope(coreEnvs);
      if (url.endsWith('/api/v1/projects/billing/envs')) return envelope(billingEnvs);
      throw new Error(`unexpected fetch ${url}`);
    });

    const user = userEvent.setup();
    renderWithProviders(<CommandPalette open onOpenChange={() => {}} activeContext={null} />);

    const banner = await screen.findByRole('alert');
    expect(banner).toHaveTextContent('프로젝트 목록을 불러오지 못했습니다');
    // Footer must not lie about syncing once we've already failed upstream.
    expect(screen.queryByText('동기화 중…')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '목록 새로고침' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
    // After a successful refetch the projects group materializes and the
    // env prefetch chain runs, proving retry routed to the right action.
    await screen.findByRole('group', { name: '프로젝트' });
  });

  it('renders the inline 동기화 실패 banner when prefetch fails and retries on click', async () => {
    // First call: projects ok. Subsequent env calls fail until retry.
    let failNext = true;
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString();
      if (url.endsWith('/api/v1/projects')) return envelope(projects);
      if (url.includes('/envs') && !url.includes('/secrets')) {
        if (failNext) return rejected('network');
        if (url.endsWith('/api/v1/projects/core/envs')) return envelope(coreEnvs);
        if (url.endsWith('/api/v1/projects/billing/envs')) return envelope(billingEnvs);
      }
      throw new Error(`unexpected fetch ${url}`);
    });
    // Suppress the intentional console.error so test output stays clean.
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    const user = userEvent.setup();
    renderWithProviders(<CommandPalette open onOpenChange={() => {}} activeContext={null} />);

    const banner = await screen.findByRole('alert');
    expect(banner).toHaveTextContent('환경·키 동기화 실패');
    expect(errSpy).toHaveBeenCalledWith('command palette prefetch failed', expect.anything());

    // Flip to success then retry; banner should clear and the footer should
    // briefly confirm recovery. Quiet text toggle on a state transition that
    // the operator just triggered — banner unmount alone was too silent.
    failNext = false;
    await user.click(screen.getByRole('button', { name: '동기화 다시' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
    await screen.findByText('동기화 완료');

    errSpy.mockRestore();
  });
});

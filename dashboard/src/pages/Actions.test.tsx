import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import { renderWithProviders } from '../test/renderWithProviders';
import { ActionsPage } from './Actions';

const navigateMock = vi.fn();

vi.mock('@tanstack/react-router', async () => {
  const actual =
    await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router');
  return {
    ...actual,
    useRouter: () => ({
      navigate: navigateMock,
      state: { matches: [] },
      subscribe: () => () => {},
    }),
    useRouterState: <T,>({ select }: { select: (s: { matches: unknown[] }) => T }) =>
      select({ matches: [] }),
    Link: ({ children, ...rest }: { children: React.ReactNode } & Record<string, unknown>) => (
      <a {...(rest as Record<string, unknown>)}>{children}</a>
    ),
  };
});

beforeEach(() => {
  navigateMock.mockReset();
  // AppShell/CommandPalette never fetch here (palette is closed), but stub
  // fetch so an accidental network call fails loudly instead of hitting a
  // real endpoint.
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.reject(new Error('no network in Actions test'))),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ActionsPage', () => {
  it('renders the page header and both injection-model sections', () => {
    renderWithProviders(<ActionsPage />);
    expect(screen.getByRole('heading', { name: 'GitHub Actions', level: 1 })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '2. process-env (기본)' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '3. github-env (opt-in)' })).toBeInTheDocument();
  });

  it('shows copyable workflow snippets for both models', () => {
    renderWithProviders(<ActionsPage />);
    // The two snippet <pre> blocks carry their aria-label.
    expect(screen.getByLabelText('process-env 워크플로 스니펫')).toBeInTheDocument();
    expect(screen.getByLabelText('github-env 워크플로 스니펫')).toBeInTheDocument();
    // Each has a copy button.
    expect(
      screen.getByRole('button', { name: 'process-env 워크플로 스니펫 복사' }),
    ).toBeInTheDocument();
  });

  it('links to the token issuance screen', () => {
    renderWithProviders(<ActionsPage />);
    // The mocked Link renders <a to="..."> without href, so it has no link
    // role; query by its unique text and assert the router target.
    const link = screen.getByText('서비스 토큰');
    expect(link).toHaveAttribute('to', '/settings/tokens');
  });

  it('surfaces the honest scope limits (read-scope M4, masking best-effort)', () => {
    renderWithProviders(<ActionsPage />);
    expect(screen.getByText(/read scope가 없습니다/)).toBeInTheDocument();
    expect(screen.getByText(/best-effort/)).toBeInTheDocument();
  });
});

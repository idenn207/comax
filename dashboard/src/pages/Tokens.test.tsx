import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { TokensPage } from './Tokens';

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

const fetchMock = vi.fn();

const BOOTSTRAP = { id: 1, name: 'bootstrap', is_admin: true, created_at: '2026-01-01T00:00:00Z' };
const CI = { id: 2, name: 'ci', is_admin: false, created_at: '2026-01-02T00:00:00Z' };

beforeEach(() => {
  navigateMock.mockReset();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  setCsrfToken('csrf-1');
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function mockEnvelope(data: unknown, init?: { status?: number; ok?: boolean }) {
  const status = init?.status ?? 200;
  const ok = init?.ok ?? true;
  const body = ok ? { ok: true, data } : { ok: false, error: data };
  return new Response(JSON.stringify(body), { status });
}

describe('TokensPage', () => {
  it('lists tokens with the admin badge on privileged rows', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([BOOTSTRAP, CI]));
    renderWithProviders(<TokensPage />);

    expect(await screen.findByText('bootstrap')).toBeInTheDocument();
    expect(screen.getByText('ci')).toBeInTheDocument();
    // Exactly one admin badge (the bootstrap token).
    expect(screen.getByText('admin')).toBeInTheDocument();
  });

  it('shows an admin-only notice and hides the create button on 403', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ ok: false, error: { code: 'forbidden', message: 'admin token required' } }),
        { status: 403 },
      ),
    );
    renderWithProviders(<TokensPage />);

    expect(await screen.findByText(/관리자 토큰으로만/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '새 토큰' })).not.toBeInTheDocument();
  });

  it('issues a token and reveals the plaintext exactly once', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([BOOTSTRAP])); // initial list
    fetchMock.mockResolvedValueOnce(
      mockEnvelope(
        {
          token: 'plain-xyz-once',
          id: 3,
          name: 'ci-github',
          is_admin: false,
          created_at: '2026-01-03T00:00:00Z',
        },
        { status: 201 },
      ),
    );
    fetchMock.mockResolvedValueOnce(mockEnvelope([BOOTSTRAP])); // invalidate refetch

    const user = userEvent.setup();
    renderWithProviders(<TokensPage />);

    await user.click(await screen.findByRole('button', { name: '새 토큰' }));
    const dialog = await screen.findByRole('dialog', { name: '새 서비스 토큰' });
    await user.type(within(dialog).getByLabelText('토큰 이름'), 'ci-github');
    await user.click(within(dialog).getByRole('button', { name: '발급' }));

    // The plaintext is shown once, in a read-only field.
    expect(await screen.findByDisplayValue('plain-xyz-once')).toBeInTheDocument();

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        ([path, init]) =>
          path === '/api/v1/tokens' && (init as RequestInit | undefined)?.method === 'POST',
      );
      expect(post).toBeDefined();
    });
  });

  it('revokes a token via DELETE after confirmation', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([BOOTSTRAP, CI])); // list
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 })); // revoke (204 = null body)
    fetchMock.mockResolvedValueOnce(
      mockEnvelope([BOOTSTRAP, { ...CI, revoked_at: '2026-02-01T00:00:00Z' }]),
    ); // refetch

    const user = userEvent.setup();
    renderWithProviders(<TokensPage />);

    await user.click(await screen.findByRole('button', { name: '토큰 ci 회수' }));
    const confirm = await screen.findByRole('alertdialog');
    await user.click(within(confirm).getByRole('button', { name: '회수' }));

    await waitFor(() => {
      const del = fetchMock.mock.calls.find(
        ([path, init]) =>
          path === '/api/v1/tokens/2' && (init as RequestInit | undefined)?.method === 'DELETE',
      );
      expect(del).toBeDefined();
    });
  });
});

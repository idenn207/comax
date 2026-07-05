import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { WebhooksPage } from './Webhooks';

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

const HOOK = {
  id: 1,
  project: 'comax',
  env: 'prod',
  url: 'http://deploy.internal/hook',
  events: ['secret.upsert', 'secret.delete'],
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

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

describe('WebhooksPage', () => {
  it('lists webhooks with scope, url, and event badges', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([HOOK]));
    renderWithProviders(<WebhooksPage />);

    expect(await screen.findByText('comax/prod')).toBeInTheDocument();
    expect(screen.getByText('http://deploy.internal/hook')).toBeInTheDocument();
    expect(screen.getByText('활성')).toBeInTheDocument();
  });

  it('shows an admin-only notice and hides the create button on 403', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ ok: false, error: { code: 'forbidden', message: 'admin token required' } }),
        { status: 403 },
      ),
    );
    renderWithProviders(<WebhooksPage />);

    expect(await screen.findByText(/관리자 토큰으로만/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '새 웹훅' })).not.toBeInTheDocument();
  });

  it('registers a webhook and reveals the signing secret exactly once', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([])); // initial empty list
    fetchMock.mockResolvedValueOnce(
      mockEnvelope(
        {
          id: 5,
          project: 'comax',
          url: 'http://deploy.internal/hook',
          events: ['secret.upsert', 'secret.rollback', 'secret.delete'],
          enabled: true,
          signing_secret: 'whsec-once-xyz',
          created_at: '2026-01-03T00:00:00Z',
        },
        { status: 201 },
      ),
    );
    fetchMock.mockResolvedValueOnce(mockEnvelope([HOOK])); // invalidate refetch

    const user = userEvent.setup();
    renderWithProviders(<WebhooksPage />);

    await user.click(await screen.findByRole('button', { name: '새 웹훅' }));
    const dialog = await screen.findByRole('dialog', { name: '새 웹훅' });
    await user.type(within(dialog).getByLabelText('프로젝트'), 'comax');
    await user.type(within(dialog).getByLabelText('수신 URL'), 'http://deploy.internal/hook');
    await user.click(within(dialog).getByRole('button', { name: '등록' }));

    // The signing secret is shown once, in a read-only field.
    expect(await screen.findByDisplayValue('whsec-once-xyz')).toBeInTheDocument();

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        ([path, init]) =>
          path === '/api/v1/webhooks' && (init as RequestInit | undefined)?.method === 'POST',
      );
      expect(post).toBeDefined();
    });
  });

  it('soft-disables a webhook via PATCH', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([HOOK])); // list (enabled)
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 })); // patch
    fetchMock.mockResolvedValueOnce(mockEnvelope([{ ...HOOK, enabled: false }])); // refetch

    const user = userEvent.setup();
    renderWithProviders(<WebhooksPage />);

    // The button label states what will happen: an enabled webhook shows 비활성화.
    await user.click(await screen.findByRole('button', { name: '웹훅 comax/prod 비활성화' }));

    await waitFor(() => {
      const patch = fetchMock.mock.calls.find(
        ([path, init]) =>
          path === '/api/v1/webhooks/1' && (init as RequestInit | undefined)?.method === 'PATCH',
      );
      expect(patch).toBeDefined();
      expect(JSON.parse((patch![1] as RequestInit).body as string)).toEqual({ enabled: false });
    });
  });

  it('deletes a webhook via DELETE after confirmation', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([HOOK])); // list
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 })); // delete
    fetchMock.mockResolvedValueOnce(mockEnvelope([])); // refetch

    const user = userEvent.setup();
    renderWithProviders(<WebhooksPage />);

    await user.click(await screen.findByRole('button', { name: '웹훅 comax/prod 삭제' }));
    const confirm = await screen.findByRole('alertdialog');
    await user.click(within(confirm).getByRole('button', { name: '삭제' }));

    await waitFor(() => {
      const del = fetchMock.mock.calls.find(
        ([path, init]) =>
          path === '/api/v1/webhooks/1' && (init as RequestInit | undefined)?.method === 'DELETE',
      );
      expect(del).toBeDefined();
    });
  });
});

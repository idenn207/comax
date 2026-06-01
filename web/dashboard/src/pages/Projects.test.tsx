import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { ProjectsPage } from './Projects';

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

describe('ProjectsPage', () => {
  it('shows the empty state and create dialog when no projects exist', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([]));
    const user = userEvent.setup();
    renderWithProviders(<ProjectsPage />);

    expect(
      await screen.findByRole('heading', { name: '아직 프로젝트가 없습니다' }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '첫 프로젝트 만들기' }));
    expect(await screen.findByRole('dialog', { name: '새 프로젝트' })).toBeInTheDocument();
  });

  it('lists projects returned from the API and renders each card with its env_count chip', async () => {
    fetchMock.mockResolvedValueOnce(
      mockEnvelope([
        { id: 1, name: 'alpha', created_at: '2026-01-01T00:00:00Z', env_count: 3 },
        { id: 2, name: 'beta', created_at: '2026-01-02T00:00:00Z', env_count: 0 },
      ]),
    );
    renderWithProviders(<ProjectsPage />);

    expect(await screen.findByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('beta')).toBeInTheDocument();
    // Each card carries the Doppler-vocabulary chip wired to env_count.
    // Beta has zero envs and must still render "0 configs" rather than
    // collapsing the chip (DESIGN.md principle 3: missing is a signal).
    expect(screen.getByText('3 configs')).toBeInTheDocument();
    expect(screen.getByText('0 configs')).toBeInTheDocument();
  });

  it('creates a project via POST and clears the form on success', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([])); // initial list
    fetchMock.mockResolvedValueOnce(
      mockEnvelope(
        { id: 1, name: 'gamma', created_at: '2026-01-03T00:00:00Z', env_count: 0 },
        { status: 201 },
      ),
    );
    fetchMock.mockResolvedValueOnce(
      mockEnvelope([{ id: 1, name: 'gamma', created_at: '2026-01-03T00:00:00Z', env_count: 0 }]),
    );

    const user = userEvent.setup();
    renderWithProviders(<ProjectsPage />);

    await user.click(await screen.findByRole('button', { name: '새 프로젝트' }));
    const dialog = await screen.findByRole('dialog', { name: '새 프로젝트' });
    await user.type(within(dialog).getByLabelText('프로젝트 이름'), 'gamma');
    await user.click(within(dialog).getByRole('button', { name: '생성' }));

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find(
        ([path, init]) =>
          path === '/api/v1/projects' && (init as RequestInit | undefined)?.method === 'POST',
      );
      expect(postCall).toBeDefined();
    });

    expect(await screen.findByText('gamma')).toBeInTheDocument();
  });

  it('shows a localized error when project name conflicts', async () => {
    fetchMock.mockResolvedValueOnce(mockEnvelope([]));
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ ok: false, error: { code: 'conflict', message: 'duplicate' } }),
        { status: 409 },
      ),
    );

    const user = userEvent.setup();
    renderWithProviders(<ProjectsPage />);

    await user.click(await screen.findByRole('button', { name: '새 프로젝트' }));
    const dialog = await screen.findByRole('dialog', { name: '새 프로젝트' });
    await user.type(within(dialog).getByLabelText('프로젝트 이름'), 'dup');
    await user.click(within(dialog).getByRole('button', { name: '생성' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('이미 존재');
  });
});

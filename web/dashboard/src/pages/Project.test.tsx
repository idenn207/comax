import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { ProjectPage } from './Project';

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

function envelope(data: unknown, status = 200) {
  return new Response(JSON.stringify({ ok: status < 400, data }), { status });
}

describe('ProjectPage', () => {
  it('renders envs with inheritance badges', async () => {
    fetchMock.mockResolvedValueOnce(
      envelope([
        { id: 1, project_id: 1, name: 'base', created_at: '2026-01-01T00:00:00Z' },
        {
          id: 2,
          project_id: 1,
          name: 'prod',
          inherits_from: 'base',
          created_at: '2026-01-02T00:00:00Z',
        },
      ]),
    );
    renderWithProviders(<ProjectPage projectName="alpha" />);
    expect(await screen.findByText('base')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText('← base')).toBeInTheDocument();
  });

  it('opens the create dialog and prepopulates the inheritance picker', async () => {
    fetchMock.mockResolvedValueOnce(
      envelope([{ id: 1, project_id: 1, name: 'base', created_at: '2026-01-01T00:00:00Z' }]),
    );
    const user = userEvent.setup();
    renderWithProviders(<ProjectPage projectName="alpha" />);
    await screen.findByText('base');
    await user.click(screen.getByRole('button', { name: '새 환경' }));
    const dialog = await screen.findByRole('dialog', { name: '새 환경' });
    expect(within(dialog).getByLabelText('환경 이름')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('상속받을 환경')).toBeInTheDocument();
  });

  it('surfaces project-not-found with a friendly message', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ ok: false, error: { code: 'not_found', message: 'not found' } }),
        { status: 404 },
      ),
    );
    renderWithProviders(<ProjectPage projectName="ghost" />);
    expect(await screen.findByRole('alert')).toHaveTextContent('찾을 수 없습니다');
  });
});

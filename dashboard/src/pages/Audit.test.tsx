import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { AuditPage } from './Audit';

const navigateMock = vi.fn();

vi.mock('@tanstack/react-router', async () => {
  const actual =
    await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router');
  return {
    ...actual,
    useNavigate: () => navigateMock,
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

function envelopeWithMeta(data: unknown, meta: unknown, status = 200) {
  return new Response(JSON.stringify({ ok: status < 400, data, meta }), { status });
}

describe('AuditPage', () => {
  it('renders the empty state when no entries match', async () => {
    fetchMock.mockResolvedValueOnce(envelopeWithMeta([], { limit: 50 }));
    renderWithProviders(<AuditPage filter={{}} />);
    expect(
      await screen.findByRole('heading', { name: '조회된 이벤트가 없습니다' }),
    ).toBeInTheDocument();
  });

  it('renders table rows with action / target / token columns', async () => {
    fetchMock.mockResolvedValueOnce(
      envelopeWithMeta(
        [
          {
            id: 99,
            action: 'secret.upsert',
            target: 'project=alpha env=prod key=DB_URL',
            actor_token_id: 3,
            metadata: 'version=4',
            created_at: '2026-01-01T00:00:00Z',
          },
          {
            id: 98,
            action: 'env.create',
            target: 'project=alpha env=staging',
            created_at: '2026-01-01T00:01:00Z',
          },
        ],
        { limit: 50 },
      ),
    );
    renderWithProviders(<AuditPage filter={{}} />);
    expect(await screen.findByText('secret.upsert')).toBeInTheDocument();
    expect(screen.getByText('env.create')).toBeInTheDocument();
    expect(screen.getByText('project=alpha env=prod key=DB_URL')).toBeInTheDocument();
    expect(screen.getByText('version=4')).toBeInTheDocument();
    expect(screen.getByText('2개 로드됨')).toBeInTheDocument();
  });

  it('disables "더 보기" when the page is full but next_before missing', async () => {
    fetchMock.mockResolvedValueOnce(
      envelopeWithMeta(
        [
          {
            id: 1,
            action: 'secret.upsert',
            target: 'project=alpha env=prod key=A',
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        { limit: 50 },
      ),
    );
    renderWithProviders(<AuditPage filter={{}} />);
    const moreButton = await screen.findByRole('button', { name: '마지막 페이지' });
    expect(moreButton).toBeDisabled();
  });

  it('loads the next page when "더 보기" is pressed', async () => {
    fetchMock.mockResolvedValueOnce(
      envelopeWithMeta(
        [
          {
            id: 10,
            action: 'secret.upsert',
            target: 'project=alpha env=prod key=A',
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        { limit: 1, next_before: 10 },
      ),
    );
    fetchMock.mockResolvedValueOnce(
      envelopeWithMeta(
        [
          {
            id: 5,
            action: 'env.create',
            target: 'project=alpha env=staging',
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        { limit: 1 },
      ),
    );
    const user = userEvent.setup();
    renderWithProviders(<AuditPage filter={{}} />);
    await screen.findByText('secret.upsert');
    await user.click(screen.getByRole('button', { name: '더 보기' }));
    expect(await screen.findByText('env.create')).toBeInTheDocument();
    // Second fetch must carry before=10 cursor.
    const secondCall = fetchMock.mock.calls[1]?.[0] as string;
    expect(secondCall).toContain('before=10');
  });

  it('submits filter form and navigates with search params', async () => {
    fetchMock.mockResolvedValue(envelopeWithMeta([], { limit: 50 }));
    const user = userEvent.setup();
    renderWithProviders(<AuditPage filter={{}} />);
    await screen.findByLabelText('프로젝트');
    await user.type(screen.getByLabelText('프로젝트'), 'alpha');
    await user.type(screen.getByLabelText('액션'), 'secret.upsert');
    await user.click(screen.getByRole('button', { name: '필터 적용' }));
    expect(navigateMock).toHaveBeenCalledWith({
      to: '/audit',
      search: { project: 'alpha', action: 'secret.upsert' },
      replace: true,
    });
  });

  it('rejects non-integer actor input', async () => {
    fetchMock.mockResolvedValue(envelopeWithMeta([], { limit: 50 }));
    const user = userEvent.setup();
    renderWithProviders(<AuditPage filter={{}} />);
    await user.type(screen.getByLabelText('토큰 ID'), 'abc');
    await user.click(screen.getByRole('button', { name: '필터 적용' }));
    expect(screen.getByText('actor는 양의 정수여야 합니다.')).toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it('reset button clears the filters via navigate', async () => {
    fetchMock.mockResolvedValue(envelopeWithMeta([], { limit: 50 }));
    const user = userEvent.setup();
    renderWithProviders(<AuditPage filter={{ project: 'alpha' }} />);
    await user.click(screen.getByRole('button', { name: '초기화' }));
    expect(navigateMock).toHaveBeenCalledWith({ to: '/audit', search: {}, replace: true });
  });
});

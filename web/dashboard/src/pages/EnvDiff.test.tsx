import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { EnvDiffPage } from './EnvDiff';

const navigateMock = vi.fn();

vi.mock('@tanstack/react-router', async () => {
  const actual =
    await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router');
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useRouter: () => ({ navigate: navigateMock }),
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

function errorEnvelope(code: string, message: string, status = 400) {
  return new Response(JSON.stringify({ ok: false, error: { code, message } }), { status });
}

const seedEnvs = [
  { id: 1, project_id: 10, name: 'local', created_at: '2026-01-01T00:00:00Z' },
  { id: 2, project_id: 10, name: 'prod', created_at: '2026-01-02T00:00:00Z' },
  { id: 3, project_id: 10, name: 'staging', created_at: '2026-01-03T00:00:00Z' },
];

describe('EnvDiffPage', () => {
  it('shows the env-picker empty state when no against is chosen', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedEnvs));
    renderWithProviders(<EnvDiffPage projectName="alpha" envName="local" against="" />);
    expect(await screen.findByRole('heading', { name: '환경을 선택하세요' })).toBeInTheDocument();
    // Diff endpoint should not have been called yet.
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/projects/alpha/envs');
  });

  it('renders three diff columns with counts when diff loads', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedEnvs));
    fetchMock.mockResolvedValueOnce(
      envelope({
        lhs: 'local',
        rhs: 'prod',
        added: ['NEW_KEY'],
        removed: ['OLD_KEY', 'GONE_KEY'],
        changed: [{ key: 'DB_URL', lhs_version: 4, rhs_version: 7 }],
      }),
    );
    renderWithProviders(<EnvDiffPage projectName="alpha" envName="local" against="prod" />);

    // Counts in the summary line.
    expect(await screen.findByText('+1 / −2 / ~1')).toBeInTheDocument();
    // Each list section by accessible name.
    expect(screen.getByRole('region', { name: 'local에만 있음' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'prod에만 있음' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '값이 다름' })).toBeInTheDocument();
    // Key labels present.
    expect(screen.getByText('NEW_KEY')).toBeInTheDocument();
    expect(screen.getByText('OLD_KEY')).toBeInTheDocument();
    expect(screen.getByText('DB_URL')).toBeInTheDocument();
    expect(screen.getByText('local v4 ↔ prod v7')).toBeInTheDocument();
  });

  it('shows "두 환경이 동일합니다" when all lists are empty', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedEnvs));
    fetchMock.mockResolvedValueOnce(
      envelope({ lhs: 'local', rhs: 'prod', added: [], removed: [], changed: [] }),
    );
    renderWithProviders(<EnvDiffPage projectName="alpha" envName="local" against="prod" />);
    expect(
      await screen.findByRole('heading', { name: '두 환경이 동일합니다' }),
    ).toBeInTheDocument();
  });

  it('renders ApiError code on diff failure', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedEnvs));
    fetchMock.mockResolvedValueOnce(errorEnvelope('bad_reference', 'cycle in chain', 422));
    renderWithProviders(<EnvDiffPage projectName="alpha" envName="local" against="prod" />);
    expect(
      await screen.findByText(/상속 체인에 문제가 있습니다: cycle in chain/),
    ).toBeInTheDocument();
  });
});

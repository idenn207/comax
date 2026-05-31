import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { setCsrfToken } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';
import { EnvSecretsPage } from './EnvSecrets';

const navigateMock = vi.fn();

vi.mock('@tanstack/react-router', async () => {
  const actual =
    await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router');
  return {
    ...actual,
    useRouter: () => ({ navigate: navigateMock }),
    Link: ({ children, ...rest }: { children: React.ReactNode } & Record<string, unknown>) => (
      <a {...(rest as Record<string, unknown>)}>{children}</a>
    ),
  };
});

const fetchMock = vi.fn();
const clipboardWrite = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  navigateMock.mockReset();
  fetchMock.mockReset();
  clipboardWrite.mockReset();
  clipboardWrite.mockResolvedValue(undefined);
  vi.stubGlobal('fetch', fetchMock);
  // navigator.clipboard is a getter in modern jsdom; vi.stubGlobal lets
  // us replace the whole navigator with a clone that proxies writeText
  // to our spy. afterEach unwinds the stub automatically.
  vi.stubGlobal('navigator', {
    ...globalThis.navigator,
    clipboard: { writeText: clipboardWrite },
  });
  setCsrfToken('csrf-1');
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function envelope(data: unknown, status = 200) {
  return new Response(JSON.stringify({ ok: status < 400, data }), { status });
}

const seedSecrets = [
  {
    secret_id: 100,
    key: 'DATABASE_URL',
    value: 'postgres://u:p@h/db',
    version: 3,
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    secret_id: 101,
    key: 'API_KEY',
    value: 'sk-test',
    version: 1,
    updated_at: '2026-01-02T00:00:00Z',
  },
];

describe('EnvSecretsPage', () => {
  it('renders rows masked by default and toggles reveal per row', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedSecrets));
    const user = userEvent.setup();
    renderWithProviders(<EnvSecretsPage projectName="alpha" envName="prod" />);

    await screen.findByText('DATABASE_URL');
    // Plaintext is not present yet.
    expect(screen.queryByText('postgres://u:p@h/db')).not.toBeInTheDocument();
    expect(screen.queryByText('sk-test')).not.toBeInTheDocument();

    // Default sort is by key, so API_KEY is row 1, DATABASE_URL row 2.
    const apiKeyRow = screen.getByText('API_KEY').closest('tr')! as HTMLElement;
    await user.click(within(apiKeyRow).getByRole('button', { name: '값 표시' }));
    expect(await screen.findByText('sk-test')).toBeInTheDocument();
    // The other row stays masked.
    expect(screen.queryByText('postgres://u:p@h/db')).not.toBeInTheDocument();
  });

  it('filters rows by key substring', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedSecrets));
    const user = userEvent.setup();
    renderWithProviders(<EnvSecretsPage projectName="alpha" envName="prod" />);
    await screen.findByText('DATABASE_URL');
    await user.type(screen.getByLabelText('키 이름으로 필터'), 'API');
    expect(screen.getByText('API_KEY')).toBeInTheDocument();
    expect(screen.queryByText('DATABASE_URL')).not.toBeInTheDocument();
  });

  it('enables .env copy when secrets load and disables it on empty', async () => {
    // jsdom 25's navigator.clipboard sits behind a getter that resists
    // straightforward stubbing; the end-to-end "click → clipboard.writeText"
    // assertion belongs in Playwright (Task 12). Here we verify the
    // button's enabled-state contract — i.e. it wires the right disabled
    // condition — and lean on dotenv.test.ts for the formatting contract.
    fetchMock.mockResolvedValueOnce(envelope(seedSecrets));
    renderWithProviders(<EnvSecretsPage projectName="alpha" envName="prod" />);
    await screen.findByText('DATABASE_URL');
    const button = await screen.findByRole('button', { name: /\.env/ });
    expect(button).not.toBeDisabled();
  });

  it('disables .env copy when the env has no secrets', async () => {
    fetchMock.mockResolvedValueOnce(envelope([]));
    renderWithProviders(<EnvSecretsPage projectName="alpha" envName="prod" />);
    await screen.findByRole('heading', { name: '시크릿이 없습니다' });
    const button = screen.getByRole('button', { name: /\.env/ });
    expect(button).toBeDisabled();
  });

  it('opens the version timeline panel from the row 이력 button', async () => {
    fetchMock.mockResolvedValueOnce(envelope(seedSecrets));
    // versions list lazy-fetched when the panel opens
    fetchMock.mockResolvedValueOnce(
      envelope([
        {
          id: 10,
          secret_id: 100,
          version: 3,
          ciphertext_size: 32,
          created_at: '2026-01-01T00:00:00Z',
        },
        {
          id: 9,
          secret_id: 100,
          version: 2,
          ciphertext_size: 30,
          created_at: '2026-01-01T00:00:00Z',
        },
        {
          id: 8,
          secret_id: 100,
          version: 1,
          ciphertext_size: 28,
          created_at: '2026-01-01T00:00:00Z',
        },
      ]),
    );

    const user = userEvent.setup();
    renderWithProviders(<EnvSecretsPage projectName="alpha" envName="prod" />);
    await screen.findByText('DATABASE_URL');
    const dbRow = screen.getByText('DATABASE_URL').closest('tr')! as HTMLElement;
    await user.click(within(dbRow).getByRole('button', { name: 'DATABASE_URL 버전 이력' }));

    const dialog = await screen.findByRole('dialog');
    // Side panel renders a <button> per version with a stable
    // aria-label, so we can scope our assertion to those instead of
    // the noisier free-text "v3 (현재)" badge in the dialog title.
    expect(within(dialog).getByRole('button', { name: /버전 3.*현재.*선택/ })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: /버전 2 선택/ })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: /버전 1 선택/ })).toBeInTheDocument();
  });

  // Regression: two keys in the same env can share a current version
  // number. Before secret_id was exposed on ResolvedSecret, the panel
  // grouped versions heuristically by "max == secret.version" and could
  // surface the wrong key's timeline (Map iteration order). Now the
  // filter is exact, so opening "이력" on DATABASE_URL must list its
  // own three versions, not API_KEY's.
  it('isolates version timeline per secret_id even when two keys share the current version', async () => {
    const sameVersionSeed = [
      {
        secret_id: 200,
        key: 'DATABASE_URL',
        value: 'postgres://x',
        version: 2,
        updated_at: '2026-01-01T00:00:00Z',
      },
      {
        secret_id: 201,
        key: 'API_KEY',
        value: 'sk-x',
        version: 2,
        updated_at: '2026-01-02T00:00:00Z',
      },
    ];
    fetchMock.mockResolvedValueOnce(envelope(sameVersionSeed));
    // Env-level versions endpoint returns history for BOTH keys, in an
    // order that would have tripped the old heuristic (API_KEY first).
    fetchMock.mockResolvedValueOnce(
      envelope([
        {
          id: 21,
          secret_id: 201,
          version: 2,
          ciphertext_size: 16,
          created_at: '2026-01-02T00:00:00Z',
        },
        {
          id: 20,
          secret_id: 201,
          version: 1,
          ciphertext_size: 16,
          created_at: '2026-01-02T00:00:00Z',
        },
        {
          id: 11,
          secret_id: 200,
          version: 2,
          ciphertext_size: 24,
          created_at: '2026-01-01T00:00:00Z',
        },
        {
          id: 10,
          secret_id: 200,
          version: 1,
          ciphertext_size: 24,
          created_at: '2026-01-01T00:00:00Z',
        },
      ]),
    );

    const user = userEvent.setup();
    renderWithProviders(<EnvSecretsPage projectName="alpha" envName="prod" />);
    await screen.findByText('DATABASE_URL');
    const dbRow = screen.getByText('DATABASE_URL').closest('tr')! as HTMLElement;
    await user.click(within(dbRow).getByRole('button', { name: 'DATABASE_URL 버전 이력' }));

    const dialog = await screen.findByRole('dialog');
    const buttons = within(dialog).getAllByRole('button', { name: /버전 \d+.*선택/ });
    // Exactly DATABASE_URL's two versions — API_KEY's must be filtered out.
    expect(buttons).toHaveLength(2);
    expect(within(dialog).getByRole('button', { name: /버전 2.*현재.*선택/ })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: /버전 1 선택/ })).toBeInTheDocument();
  });
});

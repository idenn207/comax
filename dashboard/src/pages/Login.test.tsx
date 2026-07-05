import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { registerUnauthorizedHandler } from '../lib/api';
import { renderWithProviders } from '../test/renderWithProviders';

const navigateMock = vi.fn();

vi.mock('@tanstack/react-router', () => ({
  useRouter: () => ({ navigate: navigateMock }),
}));

const fetchMock = vi.fn();

beforeEach(() => {
  navigateMock.mockReset();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  registerUnauthorizedHandler(() => {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

async function loadLoginPage() {
  // Dynamic import so the vi.mock above is applied before the module
  // captures useRouter from @tanstack/react-router.
  const { LoginPage } = await import('./Login');
  return renderWithProviders(<LoginPage />);
}

describe('LoginPage', () => {
  it('disables the submit button until a token is typed', async () => {
    const user = userEvent.setup();
    await loadLoginPage();

    const submit = screen.getByRole('button', { name: /로그인/ });
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText('서비스 토큰'), 'svc-token');
    expect(submit).toBeEnabled();
  });

  it('submits the trimmed token and navigates to / on success', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(201, {
        ok: true,
        data: { csrf: 'csrf-1', expires_at: '2030-01-01T00:00:00Z' },
      }),
    );
    const user = userEvent.setup();
    await loadLoginPage();

    await user.type(screen.getByLabelText('서비스 토큰'), '  my-token  ');
    await user.click(screen.getByRole('button', { name: /로그인/ }));

    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith({ to: '/', replace: true }));
    const [, init] = fetchMock.mock.calls[0];
    expect(init.body).toBe(JSON.stringify({ token: 'my-token' }));
  });

  it('shows a friendly error on unknown_token (401)', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { ok: false, error: { code: 'unknown_token', message: 'bad' } }),
    );
    const user = userEvent.setup();
    await loadLoginPage();

    await user.type(screen.getByLabelText('서비스 토큰'), 'wrong');
    await user.click(screen.getByRole('button', { name: /로그인/ }));

    expect(await screen.findByText(/Invalid token/)).toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it('shows a network-specific message when fetch rejects', async () => {
    fetchMock.mockRejectedValueOnce(new Error('refused'));
    const user = userEvent.setup();
    await loadLoginPage();

    await user.type(screen.getByLabelText('서비스 토큰'), 'tok');
    await user.click(screen.getByRole('button', { name: /로그인/ }));

    expect(await screen.findByText(/Cannot reach the server/)).toBeInTheDocument();
  });
});

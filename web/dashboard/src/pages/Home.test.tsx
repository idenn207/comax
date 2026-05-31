import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { registerUnauthorizedHandler, setCsrfToken } from '../lib/api';
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

async function loadHomePage() {
  const { HomePage } = await import('./Home');
  return renderWithProviders(<HomePage />);
}

describe('HomePage', () => {
  it('renders the logout button and the scaffold content', async () => {
    await loadHomePage();
    expect(screen.getByRole('button', { name: '로그아웃' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Comax Secrets' })).toBeInTheDocument();
  });

  it('logout: DELETEs the session and navigates back to /login', async () => {
    setCsrfToken('csrf-1');
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    await loadHomePage();

    await user.click(screen.getByRole('button', { name: '로그아웃' }));

    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith({ to: '/login', replace: true }));
    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe('/api/v1/dashboard/session');
    expect(init.method).toBe('DELETE');
  });

  it('logout: still navigates back to /login when the server is down', async () => {
    setCsrfToken('csrf-1');
    fetchMock.mockRejectedValueOnce(new Error('refused'));
    const user = userEvent.setup();
    await loadHomePage();

    await user.click(screen.getByRole('button', { name: '로그아웃' }));

    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith({ to: '/login', replace: true }));
    // The error is surfaced to the operator but the redirect still fires.
    expect(await screen.findByRole('alert')).toHaveTextContent(/refused|로그아웃/);
  });
});

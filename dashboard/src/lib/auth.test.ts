import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { getCsrfToken, registerUnauthorizedHandler, setCsrfToken } from './api';
import { forceLogout, isAuthenticated, login, logout, subscribeAuth } from './auth';

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockReset();
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

describe('isAuthenticated', () => {
  it('is false when no CSRF token is stored', () => {
    expect(isAuthenticated()).toBe(false);
  });

  it('is true once a CSRF token has been stored', () => {
    setCsrfToken('csrf');
    expect(isAuthenticated()).toBe(true);
  });
});

describe('login', () => {
  it('POSTs the trimmed token, stores the CSRF, and notifies listeners', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(201, {
        ok: true,
        data: { csrf: 'csrf-1', expires_at: '2030-01-01T00:00:00Z' },
      }),
    );
    const listener = vi.fn();
    const unsubscribe = subscribeAuth(listener);

    await login('  service-token  ');

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe('/api/v1/dashboard/session');
    expect(init.method).toBe('POST');
    expect(init.body).toBe(JSON.stringify({ token: 'service-token' }));
    expect(getCsrfToken()).toBe('csrf-1');
    expect(isAuthenticated()).toBe(true);
    expect(listener).toHaveBeenCalledWith(true);

    unsubscribe();
  });

  it('rejects empty tokens locally without calling fetch', async () => {
    await expect(login('   ')).rejects.toMatchObject({
      code: 'bad_request',
    });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(getCsrfToken()).toBeNull();
  });

  it('does not store CSRF when the server rejects the token', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { ok: false, error: { code: 'unknown_token', message: 'bad' } }),
    );
    const listener = vi.fn();
    const unsubscribe = subscribeAuth(listener);

    await expect(login('bad')).rejects.toMatchObject({ status: 401 });
    expect(getCsrfToken()).toBeNull();
    expect(isAuthenticated()).toBe(false);
    expect(listener).not.toHaveBeenCalled();

    unsubscribe();
  });
});

describe('logout', () => {
  it('DELETEs the session, clears CSRF, and notifies listeners', async () => {
    setCsrfToken('csrf-9');
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    const listener = vi.fn();
    const unsubscribe = subscribeAuth(listener);

    await logout();

    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe('/api/v1/dashboard/session');
    expect(init.method).toBe('DELETE');
    expect(init.headers['X-CSRF-Token']).toBe('csrf-9');
    expect(getCsrfToken()).toBeNull();
    expect(listener).toHaveBeenCalledWith(false);

    unsubscribe();
  });

  it('treats 401 as already-logged-out: clears state without throwing', async () => {
    setCsrfToken('csrf-9');
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { ok: false, error: { code: 'unknown_token', message: 'gone' } }),
    );

    await expect(logout()).resolves.toBeUndefined();
    expect(getCsrfToken()).toBeNull();
  });

  it('clears local state but rethrows on non-auth errors', async () => {
    setCsrfToken('csrf-9');
    fetchMock.mockResolvedValueOnce(
      jsonResponse(500, { ok: false, error: { code: 'internal', message: 'boom' } }),
    );

    await expect(logout()).rejects.toMatchObject({ status: 500 });
    expect(getCsrfToken()).toBeNull();
  });
});

describe('forceLogout', () => {
  it('clears CSRF and notifies when authenticated', () => {
    setCsrfToken('csrf');
    const listener = vi.fn();
    const unsubscribe = subscribeAuth(listener);

    forceLogout();

    expect(getCsrfToken()).toBeNull();
    expect(listener).toHaveBeenCalledWith(false);

    unsubscribe();
  });

  it('is a no-op when not authenticated', () => {
    const listener = vi.fn();
    const unsubscribe = subscribeAuth(listener);

    forceLogout();

    expect(listener).not.toHaveBeenCalled();

    unsubscribe();
  });
});

describe('subscribeAuth', () => {
  it('stops invoking listeners after unsubscribe', async () => {
    const listener = vi.fn();
    const unsubscribe = subscribeAuth(listener);
    unsubscribe();

    fetchMock.mockResolvedValueOnce(
      jsonResponse(201, {
        ok: true,
        data: { csrf: 'csrf', expires_at: '2030-01-01T00:00:00Z' },
      }),
    );
    await login('tok');

    expect(listener).not.toHaveBeenCalled();
  });
});

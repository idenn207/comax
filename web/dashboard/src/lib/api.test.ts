import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ApiError,
  apiFetch,
  clearCsrfToken,
  getCsrfToken,
  registerUnauthorizedHandler,
  setCsrfToken,
} from './api';

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

describe('apiFetch — request shape', () => {
  it('sends GET with credentials and no body', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { ok: true, data: { hello: 'world' } }));

    const data = await apiFetch<{ hello: string }>('/api/v1/healthz');

    expect(data).toEqual({ hello: 'world' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe('/api/v1/healthz');
    expect(init.method).toBe('GET');
    expect(init.credentials).toBe('include');
    expect(init.body).toBeUndefined();
  });

  it('attaches X-CSRF-Token on mutating requests when CSRF is stored', async () => {
    setCsrfToken('csrf-123');
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { ok: true, data: {} }));

    await apiFetch('/api/v1/projects', { method: 'POST', body: { name: 'x' } });

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBe('csrf-123');
    expect(init.headers['Content-Type']).toBe('application/json');
    expect(init.body).toBe(JSON.stringify({ name: 'x' }));
  });

  it('does not attach X-CSRF-Token on GET requests', async () => {
    setCsrfToken('csrf-123');
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { ok: true, data: [] }));

    await apiFetch('/api/v1/projects');

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBeUndefined();
  });

  it('omits X-CSRF-Token on mutations when no CSRF is stored (login bootstrap)', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(201, { ok: true, data: { csrf: 'new', expires_at: '2030-01-01T00:00:00Z' } }),
    );

    await apiFetch('/api/v1/dashboard/session', {
      method: 'POST',
      body: { token: 'svc' },
    });

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers['X-CSRF-Token']).toBeUndefined();
  });
});

describe('apiFetch — error mapping', () => {
  it('throws ApiError for envelope error responses', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(404, { ok: false, error: { code: 'not_found', message: 'no such project' } }),
    );

    await expect(apiFetch('/api/v1/projects/x')).rejects.toMatchObject({
      name: 'ApiError',
      status: 404,
      code: 'not_found',
      message: 'no such project',
    });
  });

  it('throws ApiError on network failure with code "network"', async () => {
    fetchMock.mockRejectedValueOnce(new Error('refused'));

    await expect(apiFetch('/api/v1/projects')).rejects.toMatchObject({
      status: 0,
      code: 'network',
    });
  });

  it('throws ApiError on invalid JSON', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response('<html>not json</html>', { status: 500, headers: { 'Content-Type': 'text/html' } }),
    );

    await expect(apiFetch('/api/v1/projects')).rejects.toMatchObject({
      status: 500,
      code: 'invalid_response',
    });
  });

  it('throws ApiError when response is ok=true but http status is 4xx (defensive)', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(400, { ok: true, data: null }));

    await expect(apiFetch('/api/v1/projects')).rejects.toBeInstanceOf(ApiError);
  });
});

describe('apiFetch — unauthorized handler', () => {
  it('fires the handler with 401 on any path other than session-create', async () => {
    const handler = vi.fn();
    registerUnauthorizedHandler(handler);
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { ok: false, error: { code: 'unknown_token', message: 'gone' } }),
    );

    await expect(apiFetch('/api/v1/projects')).rejects.toBeInstanceOf(ApiError);
    expect(handler).toHaveBeenCalledWith(401);
  });

  it('does NOT fire the handler for 401 on POST /api/v1/dashboard/session', async () => {
    const handler = vi.fn();
    registerUnauthorizedHandler(handler);
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { ok: false, error: { code: 'unknown_token', message: 'bad token' } }),
    );

    await expect(
      apiFetch('/api/v1/dashboard/session', { method: 'POST', body: { token: 'bad' } }),
    ).rejects.toBeInstanceOf(ApiError);
    expect(handler).not.toHaveBeenCalled();
  });

  it('fires the handler with 403 only when the error code is csrf_mismatch', async () => {
    const handler = vi.fn();
    registerUnauthorizedHandler(handler);
    fetchMock.mockResolvedValueOnce(
      jsonResponse(403, { ok: false, error: { code: 'csrf_mismatch', message: 'csrf' } }),
    );

    await expect(
      apiFetch('/api/v1/projects', { method: 'POST', body: {} }),
    ).rejects.toBeInstanceOf(ApiError);
    expect(handler).toHaveBeenCalledWith(403);
  });

  it('does NOT fire the handler for 403 with a non-csrf code', async () => {
    const handler = vi.fn();
    registerUnauthorizedHandler(handler);
    fetchMock.mockResolvedValueOnce(
      jsonResponse(403, { ok: false, error: { code: 'forbidden', message: 'nope' } }),
    );

    await expect(apiFetch('/api/v1/projects')).rejects.toBeInstanceOf(ApiError);
    expect(handler).not.toHaveBeenCalled();
  });
});

describe('apiFetch — empty body', () => {
  it('returns undefined for 204 No Content responses', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const result = await apiFetch('/api/v1/dashboard/session', { method: 'DELETE' });

    expect(result).toBeUndefined();
  });

  it('throws ApiError when the body is empty but the status is not 2xx', async () => {
    fetchMock.mockResolvedValueOnce(new Response('', { status: 502, statusText: 'Bad Gateway' }));

    await expect(apiFetch('/api/v1/projects')).rejects.toMatchObject({
      status: 502,
      code: 'unknown',
    });
  });
});

describe('CSRF token storage', () => {
  it('round-trips through sessionStorage', () => {
    expect(getCsrfToken()).toBeNull();
    setCsrfToken('abc');
    expect(getCsrfToken()).toBe('abc');
    clearCsrfToken();
    expect(getCsrfToken()).toBeNull();
  });
});

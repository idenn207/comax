/**
 * API client wrapping the server's response envelope.
 *
 * Responsibilities:
 *   - Send credentials (the comax_session HttpOnly cookie travels).
 *   - Attach X-CSRF-Token on mutating requests when one is present in
 *     sessionStorage (login flow stores it after POST /dashboard/session).
 *   - Unwrap { ok, data, error, meta } and either return data or throw
 *     a typed ApiError. Callers — including TanStack Query — never see
 *     the envelope shape, only the payload or a structured error.
 *
 * Why throw instead of returning a Result<T, E>?
 *   TanStack Query already models success/error transitions, and React
 *   error boundaries are the conventional surface for unhandled API
 *   failures. Mirroring TanStack's contract is less friction than
 *   introducing a custom monad the rest of the code has to thread.
 */

export interface Envelope<T = unknown> {
  ok: boolean;
  data?: T;
  error?: { code: string; message: string };
  meta?: unknown;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

const CSRF_STORAGE_KEY = 'comax.csrf';
const MUTATING_METHODS: ReadonlySet<string> = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

/**
 * Store the CSRF token returned by POST /api/v1/dashboard/session.
 * Memory is preferred for a token this sensitive, but sessionStorage
 * survives a soft reload — the alternative would log the user out on
 * every refresh.
 */
export function setCsrfToken(token: string): void {
  sessionStorage.setItem(CSRF_STORAGE_KEY, token);
}

export function clearCsrfToken(): void {
  sessionStorage.removeItem(CSRF_STORAGE_KEY);
}

export function getCsrfToken(): string | null {
  return sessionStorage.getItem(CSRF_STORAGE_KEY);
}

/**
 * Hook called whenever the server signals the browser session is no
 * longer valid (401 on any path other than session-create, or 403 with
 * csrf_mismatch). The default is a no-op so the module stays usable in
 * isolation (tests, scripts); main.tsx registers the real handler that
 * drops CSRF + redirects to /login.
 *
 * Registered via a callback to avoid a circular import between api.ts
 * (low-level fetch) and auth.ts / router (high-level shell).
 */
export type UnauthorizedHandler = (status: 401 | 403) => void;
let onUnauthorized: UnauthorizedHandler = () => {};

export function registerUnauthorizedHandler(handler: UnauthorizedHandler): void {
  onUnauthorized = handler;
}

const SESSION_CREATE_PATH = '/api/v1/dashboard/session';

export interface RequestOptions {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
  headers?: Record<string, string>;
}

/**
 * apiFetch is the single seam between the UI and the secret-server.
 * Pass paths starting with "/api/v1/..." (or "/healthz"); never a full
 * URL — the Vite dev server proxies relative paths to :8080, and
 * production serves the SPA from the same origin.
 */
export async function apiFetch<T = unknown>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const env = await apiFetchEnvelope<T>(path, options);
  return env.data as T;
}

/**
 * Same wire contract as apiFetch but surfaces the full envelope
 * (data + meta). Pagination cursors live on `meta`, so list endpoints
 * with `before=`-style paging (audit feed) reach for this variant.
 */
export async function apiFetchEnvelope<T = unknown, M = unknown>(
  path: string,
  options: RequestOptions = {},
): Promise<{ data: T | undefined; meta?: M }> {
  const method = (options.method ?? 'GET').toUpperCase();
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...(options.headers ?? {}),
  };

  let body: BodyInit | undefined;
  if (options.body !== undefined) {
    body = JSON.stringify(options.body);
    headers['Content-Type'] = 'application/json';
  }

  if (MUTATING_METHODS.has(method)) {
    const csrf = getCsrfToken();
    if (csrf) headers['X-CSRF-Token'] = csrf;
  }

  let response: Response;
  try {
    response = await fetch(path, {
      method,
      body,
      headers,
      credentials: 'include',
      signal: options.signal,
    });
  } catch (cause) {
    // Network / abort error — surface a stable code so UI can branch.
    throw new ApiError(0, 'network', cause instanceof Error ? cause.message : 'network error');
  }

  const text = await response.text();

  // 204 No Content (and any other intentionally empty success response)
  // arrives with response.ok=true and an empty body. The DELETE
  // /dashboard/session logout endpoint relies on this — without the
  // early-return, we'd fall through to envelope parsing and throw.
  if (response.ok && text.length === 0) {
    return { data: undefined };
  }

  let envelope: Envelope<T> | null = null;
  if (text.length > 0) {
    try {
      envelope = JSON.parse(text) as Envelope<T>;
    } catch {
      throw new ApiError(response.status, 'invalid_response', text || response.statusText);
    }
  }

  if (!response.ok || !envelope?.ok) {
    const code = envelope?.error?.code ?? 'unknown';
    const message = envelope?.error?.message ?? response.statusText;

    // Auth signal: any 401 (unknown bearer / expired session) on a path
    // other than session-create means the browser session is gone.
    // 403 with csrf_mismatch on the cookie arm has the same UX — the
    // session cannot mutate, so we treat it as logged-out too. POST
    // /dashboard/session itself returns 401 on a bad token; bubble that
    // to the form rather than triggering a redirect loop.
    const isSessionCreate = method === 'POST' && path === SESSION_CREATE_PATH;
    if (!isSessionCreate) {
      if (response.status === 401) {
        onUnauthorized(401);
      } else if (response.status === 403 && code === 'csrf_mismatch') {
        onUnauthorized(403);
      }
    }

    throw new ApiError(response.status, code, message);
  }

  return { data: envelope.data, meta: envelope.meta as M | undefined };
}

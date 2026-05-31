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
    throw new ApiError(response.status, code, message);
  }

  return envelope.data as T;
}

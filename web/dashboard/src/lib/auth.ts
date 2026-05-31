/**
 * Dashboard auth state machine.
 *
 * The browser session lives in two places:
 *   1. The `comax_session` HttpOnly cookie — set by POST
 *      /api/v1/dashboard/session, sent automatically by `credentials:
 *      'include'`. JS cannot read it, which is the point.
 *   2. The CSRF token returned in the create-session response body —
 *      kept in sessionStorage via api.ts and attached to every mutating
 *      request as `X-CSRF-Token`. We use the presence of this token as
 *      the authoritative "am I logged in" signal because it's the only
 *      half of the session JS can observe.
 *
 * Why sessionStorage and not memory only?
 *   A hard reload would otherwise drop CSRF and log the user out, even
 *   though the cookie is still valid. sessionStorage scopes to the tab,
 *   clears on close, and is not exposed to other origins. XSS would also
 *   be game over for the cookie itself via fetch, so storing CSRF here
 *   does not meaningfully widen the blast radius.
 */

import { ApiError, apiFetch, clearCsrfToken, getCsrfToken, setCsrfToken } from './api';

interface SessionCreateResponse {
  csrf: string;
  expires_at: string;
}

type AuthListener = (authenticated: boolean) => void;

const listeners = new Set<AuthListener>();

function notify(authenticated: boolean): void {
  for (const listener of listeners) {
    listener(authenticated);
  }
}

/**
 * Snapshot of the auth state. Returns true iff a CSRF token is in
 * sessionStorage — see the file-level comment for why this is the
 * source of truth on the browser side.
 */
export function isAuthenticated(): boolean {
  return getCsrfToken() !== null;
}

/**
 * Subscribe to auth state transitions. Returns an unsubscribe function
 * so React effects can clean up on unmount.
 *
 * The listener is invoked with the new value after login/logout/forced
 * logout (401, 403 csrf_mismatch). It is NOT invoked synchronously with
 * the current value — callers that need the initial value should call
 * isAuthenticated() themselves.
 */
export function subscribeAuth(listener: AuthListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Exchange a service token for a dashboard session. On success the
 * server sets the HttpOnly cookie and returns a CSRF token; we persist
 * the CSRF and notify listeners. On any error we leave existing CSRF
 * untouched so a failed re-login doesn't accidentally log the user out.
 */
export async function login(token: string): Promise<void> {
  const trimmed = token.trim();
  if (trimmed === '') {
    throw new ApiError(0, 'bad_request', 'Service token is required.');
  }
  const data = await apiFetch<SessionCreateResponse>('/api/v1/dashboard/session', {
    method: 'POST',
    body: { token: trimmed },
  });
  setCsrfToken(data.csrf);
  notify(true);
}

/**
 * Revoke the current session server-side and clear local CSRF.
 *
 * Best-effort: a 401 means the session is already gone (revoked
 * concurrently, expired, server pruned it) and the local cleanup is
 * still the correct outcome. Any other error rethrows so the UI can
 * surface it, but local state is still cleared in the finally block —
 * the user clicked logout, they expect to be logged out.
 */
export async function logout(): Promise<void> {
  try {
    await apiFetch('/api/v1/dashboard/session', { method: 'DELETE' });
  } catch (err) {
    if (!(err instanceof ApiError) || err.status !== 401) {
      // Drop local state first so the rethrow doesn't leave a stuck UI.
      clearCsrfToken();
      notify(false);
      throw err;
    }
  }
  clearCsrfToken();
  notify(false);
}

/**
 * Forced logout: invoked by the api.ts unauthorized handler when the
 * server says the session is gone (401) or the CSRF is wrong (403
 * csrf_mismatch). No DELETE call — there's nothing to revoke.
 */
export function forceLogout(): void {
  if (!isAuthenticated()) {
    return;
  }
  clearCsrfToken();
  notify(false);
}

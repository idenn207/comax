/**
 * Server response shapes (mirrors of internal/server/handlers_*.go).
 * Each interface matches the JSON tag for tag on its Go counterpart.
 *
 * These are not Zod schemas because every payload arrives via apiFetch,
 * which already passes through the envelope unwrap; the dashboard trusts
 * its own server. If we ever embed the dashboard in something cross-origin
 * we will swap these for parsed schemas.
 */

export interface Project {
  id: number;
  name: string;
  created_at: string;
  /**
   * Number of environments under this project. Filled by the list
   * endpoint (server handlers_projects.go::handleListProjects via
   * ListWithEnvCounts) so the Projects grid renders the configs chip
   * without a second round-trip per card. Single-project create/lookup
   * endpoints leave this 0; only the list view reads it.
   */
  env_count: number;
}

export interface Environment {
  id: number;
  project_id: number;
  name: string;
  inherits_from?: string;
  created_at: string;
}

export interface ResolvedSecret {
  // Underlying secrets row id (parent env's row for inherited entries).
  // The version timeline filters per-key via this id, so two keys in the
  // same env that happen to share a current version number cannot
  // collide. Server always emits it (handlers_secrets.go::secretView).
  secret_id: number;
  key: string;
  value: string;
  version: number;
  updated_at: string;
}

export interface SecretVersionListEntry {
  id: number;
  secret_id: number;
  version: number;
  ciphertext_size: number;
  actor_token_id?: number;
  created_at: string;
}

export interface SecretVersionDetail {
  key: string;
  version: number;
  value: string;
  actor_token_id?: number;
  created_at: string;
}

export interface EnvDiffChanged {
  key: string;
  lhs_version: number;
  rhs_version: number;
}

export interface EnvDiff {
  lhs: string;
  rhs: string;
  added: string[];
  removed: string[];
  changed: EnvDiffChanged[];
}

export interface AuditEntry {
  id: number;
  action: string;
  target: string;
  metadata?: string;
  actor_token_id?: number;
  created_at: string;
}

export interface AuditMeta {
  next_before?: number;
  limit: number;
}

export interface AuditPage {
  entries: AuditEntry[];
  meta: AuditMeta;
}

/**
 * One row of the /settings/sessions table.
 *
 * Mirrors the server's sessionListItem shape (handlers_sessions.go).
 * Hashes are deliberately omitted; the operator never needs them and
 * exposing them would weaken the "plaintext leaves the server once"
 * invariant.
 *
 * is_current flags the session whose cookie carried *this* request, so
 * the UI can disable revoke on it. Revoking the active session would
 * land the operator on /login mid-action — see Design Critique P1 #2.
 */
export interface Session {
  id: number;
  user_agent: string;
  ip_prefix: string;
  created_at: string;
  expires_at: string;
  is_current: boolean;
}

/**
 * One row of the /settings/tokens table.
 *
 * Mirrors the server's tokenView shape (handlers_tokens.go). The token
 * hash never appears — a listing carries only non-secret metadata.
 * revoked_at is present (non-null) once a token is soft-revoked, so the
 * row can badge it distinctly; last_used_at is absent until first use.
 */
export interface Token {
  id: number;
  name: string;
  is_admin: boolean;
  created_at: string;
  last_used_at?: string;
  revoked_at?: string;
}

/**
 * Response of POST /api/v1/tokens (tokenCreatedView). `token` is the
 * plaintext bearer, shown exactly once — the server keeps only its hash.
 */
export interface CreatedToken {
  token: string;
  id: number;
  name: string;
  is_admin: boolean;
  created_at: string;
}

/**
 * Mirrors the server's webhookView (handlers_webhooks.go). The signing secret
 * never appears in a listing — it is shown once at creation. `env` is absent
 * for an all-environments subscription.
 */
export interface Webhook {
  id: number;
  project: string;
  env?: string;
  url: string;
  events: string[];
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Response of POST /api/v1/webhooks (webhookCreatedView). `signing_secret` is
 * the plaintext HMAC key, shown exactly once — the server keeps only its
 * ciphertext.
 */
export interface CreatedWebhook {
  id: number;
  project: string;
  env?: string;
  url: string;
  events: string[];
  enabled: boolean;
  signing_secret: string;
  created_at: string;
}

/**
 * Mirrors the server's deliveryView. One recent delivery attempt for a
 * webhook. `last_status` is absent for a transport error or a not-yet-attempted
 * row; `delivered_at` is present only once the delivery succeeds.
 */
export interface WebhookDelivery {
  id: number;
  event: string;
  status: string;
  attempts: number;
  last_status?: number;
  last_error?: string;
  next_attempt_at: string;
  created_at: string;
  delivered_at?: string;
}

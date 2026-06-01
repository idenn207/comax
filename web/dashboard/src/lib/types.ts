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

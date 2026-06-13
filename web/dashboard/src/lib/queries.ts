/**
 * TanStack Query keys + fetcher functions for every M2 resource.
 *
 * Why one file?
 *   Keeping query keys + fetchers colocated makes cache invalidation
 *   patterns greppable. A page that mutates secrets calls
 *   queryClient.invalidateQueries({ queryKey: queryKeys.secrets(p, e) })
 *   without having to remember which feature folder owns the shape.
 *
 * The fetchers are thin — they encode URLs and unwrap typed payloads
 * from apiFetch. Optimistic updates and rollbacks happen in the page
 * components, where intent (success toast vs. silent refresh) lives.
 */

import type { QueryClient } from '@tanstack/react-query';

import { apiFetch, apiFetchEnvelope } from './api';
import type {
  AuditEntry,
  AuditMeta,
  AuditPage,
  EnvDiff,
  Environment,
  Project,
  ResolvedSecret,
  SecretVersionDetail,
  SecretVersionListEntry,
  Session,
} from './types';

const encode = encodeURIComponent;

export interface AuditFilter {
  project?: string;
  env?: string;
  actor?: number;
  action?: string;
  before?: number;
  limit?: number;
}

export const queryKeys = {
  projects: () => ['projects'] as const,
  envs: (project: string) => ['projects', project, 'envs'] as const,
  secrets: (project: string, env: string) => ['projects', project, 'envs', env, 'secrets'] as const,
  versions: (project: string, env: string) =>
    ['projects', project, 'envs', env, 'versions'] as const,
  versionDetail: (project: string, env: string, key: string, version: number) =>
    ['projects', project, 'envs', env, 'secrets', key, 'versions', version] as const,
  envDiff: (project: string, env: string, against: string) =>
    ['projects', project, 'envs', env, 'diff', against] as const,
  audit: (filter: AuditFilter) =>
    [
      'audit',
      filter.project ?? '',
      filter.env ?? '',
      filter.actor ?? 0,
      filter.action ?? '',
      filter.before ?? 0,
      filter.limit ?? 0,
    ] as const,
  sessions: () => ['sessions'] as const,
} as const;

export async function listProjects(signal?: AbortSignal): Promise<Project[]> {
  return apiFetch<Project[]>('/api/v1/projects', { signal });
}

export async function createProject(name: string): Promise<Project> {
  return apiFetch<Project>('/api/v1/projects', {
    method: 'POST',
    body: { name },
  });
}

export async function listEnvs(project: string, signal?: AbortSignal): Promise<Environment[]> {
  return apiFetch<Environment[]>(`/api/v1/projects/${encode(project)}/envs`, { signal });
}

export async function createEnv(
  project: string,
  name: string,
  inheritsFrom: string,
): Promise<Environment> {
  return apiFetch<Environment>(`/api/v1/projects/${encode(project)}/envs`, {
    method: 'POST',
    body: { name, inherits_from: inheritsFrom },
  });
}

export async function listSecrets(
  project: string,
  env: string,
  signal?: AbortSignal,
): Promise<ResolvedSecret[]> {
  return apiFetch<ResolvedSecret[]>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/secrets`,
    { signal },
  );
}

export async function putSecret(
  project: string,
  env: string,
  key: string,
  value: string,
): Promise<ResolvedSecret> {
  return apiFetch<ResolvedSecret>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/secrets/${encode(key)}`,
    { method: 'PUT', body: { value } },
  );
}

export async function deleteSecret(project: string, env: string, key: string): Promise<void> {
  await apiFetch<void>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/secrets/${encode(key)}`,
    { method: 'DELETE' },
  );
}

export async function listVersions(
  project: string,
  env: string,
  signal?: AbortSignal,
): Promise<SecretVersionListEntry[]> {
  return apiFetch<SecretVersionListEntry[]>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/versions`,
    { signal },
  );
}

export async function getVersionDetail(
  project: string,
  env: string,
  key: string,
  version: number,
  signal?: AbortSignal,
): Promise<SecretVersionDetail> {
  return apiFetch<SecretVersionDetail>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/secrets/${encode(key)}/versions/${version}`,
    { signal },
  );
}

export async function rollbackSecret(
  project: string,
  env: string,
  key: string,
  targetVersion: number,
): Promise<ResolvedSecret> {
  return apiFetch<ResolvedSecret>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/secrets/${encode(key)}/rollback`,
    { method: 'POST', body: { target_version: targetVersion } },
  );
}

export async function diffEnvs(
  project: string,
  env: string,
  against: string,
  signal?: AbortSignal,
): Promise<EnvDiff> {
  return apiFetch<EnvDiff>(
    `/api/v1/projects/${encode(project)}/envs/${encode(env)}/diff?against=${encode(against)}`,
    { signal },
  );
}

/**
 * Warm the env list cache for every project in parallel.
 *
 * Used by the command palette on open: ⌘K trips a single shot that
 * materializes env coordinates for fuzzy match. fetchQuery (not
 * prefetchQuery) is used so a single project failure rejects the whole
 * Promise.all — prefetchQuery swallows errors into the cache and would
 * leave the palette silently empty. The caller catches and surfaces a
 * single banner instead of partial silence.
 */
export async function prefetchEnvs(qc: QueryClient, projects: readonly Project[]): Promise<void> {
  await Promise.all(
    projects.map((p) =>
      qc.fetchQuery({
        queryKey: queryKeys.envs(p.name),
        queryFn: ({ signal }) => listEnvs(p.name, signal),
      }),
    ),
  );
}

/**
 * Warm the secrets list for the active (project, env) pair only.
 *
 * The palette intentionally does NOT prefetch N×M secret lists — that
 * would balloon to hundreds of round-trips on a populated tenant. Cross-
 * env key search is backlog 8 (server-side index). Here we only widen
 * the currently visible env so a key the operator just saw becomes
 * palette-searchable without typing the env first.
 */
export async function prefetchSecrets(
  qc: QueryClient,
  project: string,
  env: string,
): Promise<void> {
  await qc.fetchQuery({
    queryKey: queryKeys.secrets(project, env),
    queryFn: ({ signal }) => listSecrets(project, env, signal),
  });
}

export async function listSessions(signal?: AbortSignal): Promise<Session[]> {
  return apiFetch<Session[]>('/api/v1/dashboard/sessions', { signal });
}

/**
 * Revoke a single session by id. Server returns 204 in all of:
 *   - the id belongs to the actor (success path, audit-logged),
 *   - the id belongs to a different token (silent no-op, no audit row),
 *   - the id does not exist (silent no-op, no audit row),
 *   - the row is already revoked (silent no-op).
 *
 * The single 204 response keeps cross-token id probes from producing an
 * existence oracle — see SessionRepo.RevokeByIDAndTokenID.
 */
export async function revokeSession(id: number): Promise<void> {
  await apiFetch<void>(`/api/v1/dashboard/sessions/${id}`, { method: 'DELETE' });
}

export async function listAudit(filter: AuditFilter, signal?: AbortSignal): Promise<AuditPage> {
  const params = new URLSearchParams();
  if (filter.project) params.set('project', filter.project);
  if (filter.env) params.set('env', filter.env);
  if (filter.actor && filter.actor > 0) params.set('actor', String(filter.actor));
  if (filter.action) params.set('action', filter.action);
  if (filter.before && filter.before > 0) params.set('before', String(filter.before));
  if (filter.limit && filter.limit > 0) params.set('limit', String(filter.limit));
  const qs = params.toString();
  const response = await apiFetchEnvelope<AuditEntry[], AuditMeta>(
    `/api/v1/audit${qs ? `?${qs}` : ''}`,
    { signal },
  );
  return {
    entries: response.data ?? [],
    meta: response.meta ?? { limit: filter.limit ?? 50 },
  };
}

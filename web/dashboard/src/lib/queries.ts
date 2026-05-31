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

import { apiFetch } from './api';
import type {
  Environment,
  Project,
  ResolvedSecret,
  SecretVersionDetail,
  SecretVersionListEntry,
} from './types';

const encode = encodeURIComponent;

export const queryKeys = {
  projects: () => ['projects'] as const,
  envs: (project: string) => ['projects', project, 'envs'] as const,
  secrets: (project: string, env: string) => ['projects', project, 'envs', env, 'secrets'] as const,
  versions: (project: string, env: string) =>
    ['projects', project, 'envs', env, 'versions'] as const,
  versionDetail: (project: string, env: string, key: string, version: number) =>
    ['projects', project, 'envs', env, 'secrets', key, 'versions', version] as const,
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

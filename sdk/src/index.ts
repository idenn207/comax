/**
 * @comax-secrets/sdk — runtime secret fetch + cache + reload for Next.js /
 * Node / Edge.
 *
 * Server-side only: a service token grants read access to an environment's
 * secrets, so the client must never run where the token would reach a
 * browser. Use it from Route Handlers, Server Components, middleware, or any
 * server runtime. See README for the Next.js singleton pattern.
 */

import { Http, type FetchLike } from "./http";
import { SecretsClient } from "./secrets";
import { ComaxError } from "./errors";

export interface CreateClientOptions {
  /** Server base URL including scheme, e.g. "https://secrets.example.com". */
  baseUrl: string;
  /** Bearer service token (M3). */
  token: string;
  /** Project name. */
  project: string;
  /** Environment name, e.g. "prod". */
  env: string;
  /** Cache freshness window in ms. Default 60_000. */
  ttlMs?: number;
  /** Per-request timeout in ms. Default 10_000. */
  timeoutMs?: number;
  /** Injected fetch (tests / non-global Edge runtimes). */
  fetch?: FetchLike;
  /** Background refresh interval in ms (long-running Node only). */
  refreshIntervalMs?: number;
}

/** Construct a `SecretsClient` from explicit options. */
export function createClient(opts: CreateClientOptions): SecretsClient {
  const http = new Http({
    baseUrl: opts.baseUrl,
    token: opts.token,
    fetch: opts.fetch,
    timeoutMs: opts.timeoutMs,
  });
  return new SecretsClient({
    http,
    project: opts.project,
    env: opts.env,
    ttlMs: opts.ttlMs,
    refreshIntervalMs: opts.refreshIntervalMs,
  });
}

/**
 * Construct a client from environment variables, with optional overrides:
 *   COMAX_URL, COMAX_TOKEN, COMAX_PROJECT, COMAX_ENV.
 *
 * Throws `ComaxError("bad_request")` naming every missing variable. Reads
 * `process.env` when available; on runtimes without `process`, pass the
 * values via `overrides`.
 */
export function createClientFromEnv(overrides: Partial<CreateClientOptions> = {}): SecretsClient {
  const fromEnv = (name: string): string | undefined =>
    typeof process !== "undefined" ? process.env?.[name] : undefined;

  const baseUrl = overrides.baseUrl ?? fromEnv("COMAX_URL");
  const token = overrides.token ?? fromEnv("COMAX_TOKEN");
  const project = overrides.project ?? fromEnv("COMAX_PROJECT");
  const env = overrides.env ?? fromEnv("COMAX_ENV");

  const missing: string[] = [];
  if (!baseUrl) missing.push("COMAX_URL");
  if (!token) missing.push("COMAX_TOKEN");
  if (!project) missing.push("COMAX_PROJECT");
  if (!env) missing.push("COMAX_ENV");
  if (missing.length > 0) {
    throw new ComaxError("bad_request", `missing required config: ${missing.join(", ")}`);
  }

  return createClient({ ...overrides, baseUrl: baseUrl!, token: token!, project: project!, env: env! });
}

export { SecretsClient } from "./secrets";
export type { Secret, SecretsClientOptions } from "./secrets";
export { Http } from "./http";
export type { FetchLike, HttpOptions } from "./http";
export {
  ComaxError,
  ComaxAuthError,
  ComaxNotFoundError,
  ComaxConflictError,
  type ComaxErrorCode,
} from "./errors";

/**
 * SecretsClient — cache + reload layer over the server's resolved-secret
 * endpoints.
 *
 * Design (Codex plan-gate F1/F2 absorbed):
 *   - `get(key)` hits the single-key endpoint and caches PER KEY. A caller
 *     that needs one secret never pulls the whole environment's plaintext
 *     into process memory. Whole-env load is opt-in via `getAll()`/`preload()`
 *     (bulk inject).
 *   - Every fetch is bounded by the Http `timeoutMs`, and concurrent reads
 *     share one in-flight promise (single-flight). A failed/timed-out flight
 *     is cleared in `finally`, so it never poisons the cache or wedges later
 *     reads. A caller-supplied `AbortSignal` unblocks that caller's await
 *     without aborting the shared underlying fetch other callers joined.
 *
 * Mirrors `pkg/client/client.go`: `GetSecret` (single key) and `ListSecrets`
 * (whole env).
 */

import { Http } from "./http";
import { ComaxError } from "./errors";

/** Resolved-plaintext secret. Mirrors `client.go` `Secret`. */
export interface Secret {
  key: string;
  value: string;
  version: number;
  updated_at: string;
}

export interface SecretsClientOptions {
  http: Http;
  project: string;
  env: string;
  /** Cache freshness window in ms. 0 disables caching (always fetch). Default 60_000. */
  ttlMs?: number;
  /**
   * Optional background refresh interval in ms. Long-running Node servers
   * only — Edge/serverless have no persistent timer. The timer is `unref`'d
   * so it never keeps the process alive. Start with `startAutoRefresh()`.
   */
  refreshIntervalMs?: number;
}

interface CacheEntry {
  secret: Secret;
  fetchedAt: number;
}

const DEFAULT_TTL_MS = 60_000;

export class SecretsClient {
  private readonly http: Http;
  private readonly project: string;
  private readonly env: string;
  private readonly ttlMs: number;
  private readonly refreshIntervalMs: number;

  private readonly cache = new Map<string, CacheEntry>();
  private readonly inflight = new Map<string, Promise<Secret>>();
  private allInflight: Promise<Secret[]> | null = null;
  private refreshTimer: ReturnType<typeof setInterval> | undefined;
  // Monotonic generation. reload() bumps it; a fetch that started before the
  // bump refuses to write its (now potentially stale) result into the cache.
  // This closes the reload-during-in-flight race: without it, a fetch already
  // reading the pre-change value would resurrect it AFTER reload() cleared it.
  private epoch = 0;

  constructor(opts: SecretsClientOptions) {
    if (!opts.project) throw new ComaxError("bad_request", "project is required");
    if (!opts.env) throw new ComaxError("bad_request", "env is required");
    this.http = opts.http;
    this.project = opts.project;
    this.env = opts.env;
    this.ttlMs = opts.ttlMs ?? DEFAULT_TTL_MS;
    this.refreshIntervalMs = opts.refreshIntervalMs ?? 0;
  }

  /** Resolved value for one key. Single-key fetch + per-key cache. */
  async get(key: string, signal?: AbortSignal): Promise<string> {
    const secret = await this.getSecret(key, signal);
    return secret.value;
  }

  /** Full `Secret` (value + version + updated_at) for one key. */
  getSecret(key: string, signal?: AbortSignal): Promise<Secret> {
    const cached = this.cache.get(key);
    if (cached && !this.isStale(cached.fetchedAt)) {
      return raceWithSignal(Promise.resolve(cached.secret), signal);
    }
    let flight = this.inflight.get(key);
    if (!flight) {
      // The shared flight uses only the Http timeout — not the caller signal —
      // so one caller aborting cannot cancel the fetch other callers joined.
      const owned = this.fetchOne(key, this.epoch);
      this.inflight.set(key, owned);
      // Identity-guarded cleanup: reload() may clear or replace this slot while
      // the fetch is in flight, so only delete it if it is still ours (never
      // clobber a newer flight a post-reload read may have started).
      void owned
        .finally(() => {
          if (this.inflight.get(key) === owned) this.inflight.delete(key);
        })
        .catch(() => undefined);
      flight = owned;
    }
    return raceWithSignal(flight, signal);
  }

  /**
   * Whole-environment fetch as a `{ KEY: value }` map (bulk inject). Opt-in:
   * this loads every secret's plaintext into memory. Populates the per-key
   * cache as a side effect.
   */
  async getAll(signal?: AbortSignal): Promise<Record<string, string>> {
    const secrets = await this.preload(signal);
    const out: Record<string, string> = {};
    for (const s of secrets) out[s.key] = s.value;
    return out;
  }

  /** Whole-environment fetch returning the full `Secret[]`. Opt-in. */
  preload(signal?: AbortSignal): Promise<Secret[]> {
    if (!this.allInflight) {
      const owned = this.fetchAll(this.epoch);
      this.allInflight = owned;
      void owned
        .finally(() => {
          if (this.allInflight === owned) this.allInflight = null;
        })
        .catch(() => undefined);
    }
    return raceWithSignal(this.allInflight, signal);
  }

  /** True when `key` is cached and still fresh. Never triggers a fetch. */
  has(key: string): boolean {
    const cached = this.cache.get(key);
    return cached !== undefined && !this.isStale(cached.fetchedAt);
  }

  /**
   * Drop the cache (all keys) or one key so the next read refetches. Also
   * clears the matching in-flight slot and bumps the generation, so a fetch
   * that is already running cannot repopulate the cache with a pre-reload
   * (potentially stale) value after this returns.
   */
  reload(key?: string): void {
    if (key !== undefined) {
      this.cache.delete(key);
      this.inflight.delete(key);
    } else {
      this.cache.clear();
      this.inflight.clear();
      this.allInflight = null;
    }
    this.epoch += 1;
  }

  /** Start background refresh (no-op if no interval configured or already running). */
  startAutoRefresh(): void {
    if (this.refreshIntervalMs <= 0 || this.refreshTimer !== undefined) return;
    this.refreshTimer = setInterval(() => {
      // Best-effort: swallow errors so a transient failure doesn't crash the
      // host process. Stale cache entries survive until the next success.
      void this.preload().catch(() => undefined);
    }, this.refreshIntervalMs);
    if (typeof (this.refreshTimer as { unref?: () => void }).unref === "function") {
      (this.refreshTimer as { unref: () => void }).unref();
    }
  }

  /** Stop background refresh. */
  stopAutoRefresh(): void {
    if (this.refreshTimer !== undefined) {
      clearInterval(this.refreshTimer);
      this.refreshTimer = undefined;
    }
  }

  private async fetchOne(key: string, startEpoch: number): Promise<Secret> {
    const secret = await this.http.get<Secret>(`${this.basePath()}/secrets/${encodeURIComponent(key)}`);
    // Skip the cache write if a reload() ran while this fetch was in flight —
    // otherwise the pre-reload response would resurrect an invalidated value.
    // The caller still receives `secret`; only the shared cache is protected.
    if (this.epoch === startEpoch) {
      this.cache.set(key, { secret, fetchedAt: Date.now() });
    }
    return secret;
  }

  private async fetchAll(startEpoch: number): Promise<Secret[]> {
    const secrets = await this.http.get<Secret[]>(`${this.basePath()}/secrets`);
    if (this.epoch === startEpoch) {
      const at = Date.now();
      for (const s of secrets) this.cache.set(s.key, { secret: s, fetchedAt: at });
    }
    return secrets;
  }

  private basePath(): string {
    return `/api/v1/projects/${encodeURIComponent(this.project)}/envs/${encodeURIComponent(this.env)}`;
  }

  private isStale(fetchedAt: number): boolean {
    if (this.ttlMs <= 0) return true;
    return Date.now() - fetchedAt >= this.ttlMs;
  }
}

/**
 * Resolve `p`, but reject early if `signal` aborts first. The underlying
 * promise (a shared single-flight fetch) is NOT cancelled — only this
 * caller's await returns. Keeps one caller's cancellation from starving the
 * others who joined the same flight.
 */
function raceWithSignal<T>(p: Promise<T>, signal?: AbortSignal): Promise<T> {
  if (!signal) return p;
  if (signal.aborted) return Promise.reject(new ComaxError("timeout", "aborted by caller"));
  return new Promise<T>((resolve, reject) => {
    const onAbort = () => reject(new ComaxError("timeout", "aborted by caller"));
    signal.addEventListener("abort", onAbort, { once: true });
    p.then(resolve, reject).finally(() => signal.removeEventListener("abort", onAbort));
  });
}

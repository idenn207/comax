/**
 * Minimal HTTP core. Mirrors `pkg/client/client.go`'s `do()`:
 *   build URL → Bearer header → parse `{ok,data,error}` envelope →
 *   status >= 400 maps `error.code` to a typed error.
 *
 * Zero runtime dependencies: uses the global `fetch` (Node 18+, Edge,
 * Workers, Deno, Bun). A custom `fetch` can be injected for tests or Edge
 * runtimes that expose a non-global implementation.
 */

import { ComaxError, toComaxError } from "./errors";

/** Structural subset of the global `fetch` we depend on. */
export type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/** Server response envelope. Mirrors `internal/server/response.go` `Envelope`. */
interface Envelope<T> {
  ok: boolean;
  data?: T;
  error?: { code: string; message: string };
  meta?: unknown;
}

export interface HttpOptions {
  /** Server base URL including scheme, e.g. "https://secrets.example.com". */
  baseUrl: string;
  /** Bearer service token (M3). Pass "" only for public endpoints. */
  token: string;
  /** Injected fetch. Defaults to the global `fetch`. */
  fetch?: FetchLike;
  /**
   * Per-request timeout in ms. The request aborts when it elapses so a
   * hung connection can never block a shared in-flight cache load (see
   * SecretsClient single-flight). 0 disables. Default 10_000 — mirrors the
   * Go client's 10s default.
   */
  timeoutMs?: number;
  /** User-Agent header. Ignored by runtimes that forbid setting it. */
  userAgent?: string;
}

const DEFAULT_TIMEOUT_MS = 10_000;

export class Http {
  private readonly base: string;
  private readonly token: string;
  private readonly doFetch: FetchLike;
  private readonly timeoutMs: number;
  private readonly userAgent: string;

  constructor(opts: HttpOptions) {
    if (!opts.baseUrl) {
      throw new ComaxError("bad_request", "baseUrl is required");
    }
    let parsed: URL;
    try {
      parsed = new URL(opts.baseUrl);
    } catch {
      throw new ComaxError("bad_request", "baseUrl is not a valid URL");
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      throw new ComaxError("bad_request", "baseUrl must use http or https");
    }
    this.base = opts.baseUrl.replace(/\/+$/, "");
    this.token = opts.token;
    this.doFetch = opts.fetch ?? resolveGlobalFetch();
    this.timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    this.userAgent = opts.userAgent ?? "comax-secrets-sdk";
  }

  /** GET `path` and unmarshal the envelope's `data` into `T`. */
  get<T>(path: string): Promise<T> {
    return this.request<T>("GET", path);
  }

  private async request<T>(method: string, path: string): Promise<T> {
    const url = this.base + path;
    const controller = new AbortController();
    const timer =
      this.timeoutMs > 0
        ? setTimeout(() => controller.abort(new ComaxError("timeout", "request timed out")), this.timeoutMs)
        : undefined;
    // Detach the timer from the event loop so a long-lived process is not
    // held open by a pending request timeout.
    if (timer && typeof (timer as { unref?: () => void }).unref === "function") {
      (timer as { unref: () => void }).unref();
    }

    let resp: Response;
    try {
      resp = await this.doFetch(url, {
        method,
        headers: this.headers(),
        signal: controller.signal,
      });
    } catch (err) {
      if (isAbortError(err)) {
        throw new ComaxError("timeout", "request timed out or was aborted");
      }
      // Never surface the raw error — it may embed the URL/token. Only the code.
      throw new ComaxError("network", "network request failed");
    } finally {
      if (timer) clearTimeout(timer);
    }

    const raw = await resp.text();
    let env: Envelope<T> | undefined;
    if (raw.length > 0) {
      try {
        env = JSON.parse(raw) as Envelope<T>;
      } catch {
        throw new ComaxError("invalid_response", `malformed response envelope (status ${resp.status})`, resp.status);
      }
    }

    if (resp.status >= 400) {
      const code = env?.error?.code ?? "internal";
      const message = env?.error?.message ?? `HTTP ${resp.status}`;
      throw toComaxError(resp.status, code, message);
    }

    return (env?.data ?? (undefined as T)) as T;
  }

  private headers(): Record<string, string> {
    const h: Record<string, string> = {
      Accept: "application/json",
      "User-Agent": this.userAgent,
    };
    if (this.token) {
      h["Authorization"] = `Bearer ${this.token}`;
    }
    return h;
  }
}

function resolveGlobalFetch(): FetchLike {
  const f = (globalThis as { fetch?: FetchLike }).fetch;
  if (typeof f !== "function") {
    throw new ComaxError(
      "internal",
      "global fetch is not available; upgrade to Node 18+ or pass options.fetch",
    );
  }
  return f;
}

function isAbortError(err: unknown): boolean {
  if (err instanceof ComaxError) return err.code === "timeout";
  return err instanceof Error && (err.name === "AbortError" || err.name === "TimeoutError");
}

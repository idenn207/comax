import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { Http, type FetchLike } from "./http";
import { SecretsClient, type Secret } from "./secrets";
import { ComaxError } from "./errors";

function secretResponse(s: Secret): Response {
  return new Response(JSON.stringify({ ok: true, data: s }), { status: 200 });
}
function listResponse(list: Secret[]): Response {
  return new Response(JSON.stringify({ ok: true, data: list }), { status: 200 });
}
function makeClient(fetchMock: FetchLike, ttlMs = 60_000): SecretsClient {
  const http = new Http({ baseUrl: "https://s", token: "t", fetch: fetchMock, timeoutMs: 0 });
  return new SecretsClient({ http, project: "api", env: "prod", ttlMs });
}
const S = (key: string, value: string): Secret => ({ key, value, version: 1, updated_at: "2026-01-01T00:00:00Z" });

describe("SecretsClient.get (single-key path)", () => {
  it("fetches the single-key endpoint, not the whole environment", async () => {
    const fetchMock = vi.fn<FetchLike>(async (url) => {
      expect(url).toBe("https://s/api/v1/projects/api/envs/prod/secrets/DB_URL");
      return secretResponse(S("DB_URL", "postgres://x"));
    });
    const client = makeClient(fetchMock);
    expect(await client.get("DB_URL")).toBe("postgres://x");
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("serves a fresh cache hit without refetching", async () => {
    const fetchMock = vi.fn<FetchLike>(async () => secretResponse(S("K", "v")));
    const client = makeClient(fetchMock);
    await client.get("K");
    await client.get("K");
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(client.has("K")).toBe(true);
  });

  it("refetches after reload()", async () => {
    const fetchMock = vi.fn<FetchLike>(async () => secretResponse(S("K", "v")));
    const client = makeClient(fetchMock);
    await client.get("K");
    client.reload("K");
    expect(client.has("K")).toBe(false);
    await client.get("K");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("collapses concurrent reads into one fetch (single-flight)", async () => {
    let release: (() => void) | undefined;
    const fetchMock = vi.fn<FetchLike>(
      () => new Promise<Response>((resolve) => (release = () => resolve(secretResponse(S("K", "v"))))),
    );
    const client = makeClient(fetchMock);
    const p1 = client.get("K");
    const p2 = client.get("K");
    release?.();
    expect(await p1).toBe("v");
    expect(await p2).toBe("v");
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("does not poison the cache when a flight fails (recovers on next read)", async () => {
    const fetchMock = vi
      .fn<FetchLike>()
      .mockRejectedValueOnce(new Error("boom"))
      .mockResolvedValueOnce(secretResponse(S("K", "v2")));
    const client = makeClient(fetchMock);
    await expect(client.get("K")).rejects.toBeInstanceOf(ComaxError);
    expect(await client.get("K")).toBe("v2");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("reload() during an in-flight fetch is not overwritten by the late response", async () => {
    const deferreds: Array<(r: Response) => void> = [];
    const fetchMock = vi.fn<FetchLike>(() => new Promise<Response>((res) => deferreds.push(res)));
    const client = makeClient(fetchMock);

    const p1 = client.get("K"); // flight 1 starts, still pending
    client.reload("K"); // invalidate while flight 1 is in flight
    deferreds[0]!(secretResponse(S("K", "stale"))); // flight 1 resolves with the pre-reload value
    await expect(p1).resolves.toBe("stale"); // the original caller still gets its value
    expect(client.has("K")).toBe(false); // but the cache was NOT repopulated by the stale flight

    const p2 = client.get("K"); // fresh miss → a new fetch, not the stale cache
    deferreds[1]!(secretResponse(S("K", "fresh")));
    await expect(p2).resolves.toBe("fresh");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("rejects when the caller signal is already aborted", async () => {
    const client = makeClient(async () => secretResponse(S("K", "v")));
    const ac = new AbortController();
    ac.abort();
    await expect(client.get("K", ac.signal)).rejects.toMatchObject({ code: "timeout" });
  });
});

describe("SecretsClient TTL", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("refetches once the entry is stale", async () => {
    const fetchMock = vi.fn<FetchLike>(async () => secretResponse(S("K", "v")));
    const client = makeClient(fetchMock, 1000);
    await client.get("K");
    vi.advanceTimersByTime(1500);
    await client.get("K");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("ttlMs=0 always refetches", async () => {
    const fetchMock = vi.fn<FetchLike>(async () => secretResponse(S("K", "v")));
    const client = makeClient(fetchMock, 0);
    await client.get("K");
    await client.get("K");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});

describe("SecretsClient.getAll / preload (whole-env opt-in)", () => {
  it("fetches the list endpoint and returns a key→value map", async () => {
    const fetchMock = vi.fn<FetchLike>(async (url) => {
      expect(url).toBe("https://s/api/v1/projects/api/envs/prod/secrets");
      return listResponse([S("A", "1"), S("B", "2")]);
    });
    const client = makeClient(fetchMock);
    expect(await client.getAll()).toEqual({ A: "1", B: "2" });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("populates the per-key cache so a later get() is a hit", async () => {
    const fetchMock = vi.fn<FetchLike>(async () => listResponse([S("A", "1")]));
    const client = makeClient(fetchMock);
    await client.preload();
    expect(await client.get("A")).toBe("1");
    expect(fetchMock).toHaveBeenCalledOnce();
  });
});

describe("SecretsClient.startAutoRefresh", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("refreshes on the interval and stops cleanly", async () => {
    const fetchMock = vi.fn<FetchLike>(async () => listResponse([S("A", "1")]));
    const http = new Http({ baseUrl: "https://s", token: "t", fetch: fetchMock, timeoutMs: 0 });
    const client = new SecretsClient({ http, project: "api", env: "prod", refreshIntervalMs: 1000 });
    client.startAutoRefresh();
    await vi.advanceTimersByTimeAsync(2500);
    client.stopAutoRefresh();
    const calls = fetchMock.mock.calls.length;
    expect(calls).toBeGreaterThanOrEqual(2);
    await vi.advanceTimersByTimeAsync(2000);
    expect(fetchMock.mock.calls.length).toBe(calls);
  });
});
